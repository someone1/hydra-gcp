// Copyright © 2017 Aeneas Rekkas <aeneas+oss@aeneas.io>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package hydragcp

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/sessions"
	"github.com/julienschmidt/httprouter"
	"github.com/ory/fosite"
	"github.com/ory/fosite/compose"
	foauth2 "github.com/ory/fosite/handler/oauth2"
	"github.com/ory/fosite/handler/openid"
	"github.com/ory/herodot"
	"github.com/ory/hydra/client"
	"github.com/ory/hydra/config"
	"github.com/ory/hydra/consent"
	"github.com/ory/hydra/jwk"
	"github.com/ory/hydra/oauth2"
	"github.com/ory/hydra/pkg"
	"github.com/ory/sqlcon"

	fgoauth2 "github.com/someone1/fosite-gcp-oauth2"
	dconfig "github.com/someone1/hydra-gcp/config"
	doauth2 "github.com/someone1/hydra-gcp/oauth2"
)

func injectFositeStore(c *config.Config, clients client.Manager) {
	var ctx = c.Context()
	var store pkg.FositeStorer

	switch con := ctx.Connection.(type) {
	case *config.MemoryConnection:
		store = oauth2.NewFositeMemoryStore(clients, c.GetAccessTokenLifespan())
	case *sqlcon.SQLConnection:
		expectDependency(c.GetLogger(), con.GetDatabase())
		store = oauth2.NewFositeSQLStore(clients, con.GetDatabase(), c.GetLogger(), c.GetAccessTokenLifespan(), c.OAuth2AccessTokenStrategy == "jwt")
	case *config.PluginConnection:
		var err error
		if store, err = con.NewOAuth2Manager(clients); err != nil {
			c.GetLogger().Fatalf("Could not load client manager plugin %s", err)
		}
		break
	case *dconfig.DatastoreConnection:
		expectDependency(c.GetLogger(), con.Context(), con.Client())
		store = doauth2.NewFositeDatastoreStore(clients, con.Client(), con.Namespace(), c.GetLogger(), c.GetAccessTokenLifespan())
	default:
		panic("Unknown connection type.")
	}

	ctx.FositeStore = store
}

func newOAuth2Provider(ctxx context.Context, c *config.Config) fosite.OAuth2Provider {
	var ctx = c.Context()
	var store = ctx.FositeStore
	expectDependency(c.GetLogger(), ctx.FositeStore)

	fc := &compose.Config{
		AccessTokenLifespan:            c.GetAccessTokenLifespan(),
		AuthorizeCodeLifespan:          c.GetAuthCodeLifespan(),
		IDTokenLifespan:                c.GetIDTokenLifespan(),
		HashCost:                       c.BCryptWorkFactor,
		ScopeStrategy:                  c.GetScopeStrategy(),
		SendDebugMessagesToClients:     c.SendOAuth2DebugMessagesToClients,
		EnforcePKCE:                    false,
		EnablePKCEPlainChallengeMethod: false,
		TokenURL:                       strings.TrimRight(c.Issuer, "/") + oauth2.TokenPath,
	}

	oidcStrategy := fgoauth2.NewOpenIDConnectStrategy(ctxx)
	oidcStrategy.Issuer = c.Issuer

	var coreStrategy foauth2.CoreStrategy
	hmacStrategy := compose.NewOAuth2HMACStrategy(fc, c.GetSystemSecret())

	if c.OAuth2AccessTokenStrategy == "jwt" {
		oauth2Strategy := fgoauth2.NewOAuth2GCPStrategy(ctxx, hmacStrategy)
		oauth2Strategy.Issuer = c.Issuer
		coreStrategy = oauth2Strategy
	} else if c.OAuth2AccessTokenStrategy == "opaque" {
		coreStrategy = hmacStrategy
	} else {
		c.GetLogger().Fatalf(`Environment variable OAUTH2_ACCESS_TOKEN_STRATEGY is set to "%s" but only "opaque" and "jwt" are valid values.`, c.OAuth2AccessTokenStrategy)
	}

	return compose.Compose(
		fc,
		store,
		&compose.CommonStrategy{
			CoreStrategy:               coreStrategy,
			OpenIDConnectTokenStrategy: oidcStrategy,
			JWTStrategy:                oidcStrategy.JWTStrategy,
		},
		nil,
		compose.OAuth2AuthorizeExplicitFactory,
		compose.OAuth2AuthorizeImplicitFactory,
		compose.OAuth2ClientCredentialsGrantFactory,
		compose.OAuth2RefreshTokenGrantFactory,
		compose.OAuth2PKCEFactory,
		compose.OpenIDConnectExplicitFactory,
		compose.OpenIDConnectHybridFactory,
		compose.OpenIDConnectImplicitFactory,
		compose.OpenIDConnectRefreshFactory,
		compose.OAuth2TokenRevocationFactory,
		compose.OAuth2TokenIntrospectionFactory,
	)
}

func setDefaultConsentURL(s string, c *config.Config, path string) string {
	if s != "" {
		return s
	}
	proto := "https"
	if c.ForceHTTP {
		proto = "http"
	}
	host := "localhost"
	if c.BindHost != "" {
		host = c.BindHost
	}
	return fmt.Sprintf("%s://%s:%d/%s", proto, host, c.BindPort, path)
}

func newOAuth2Handler(ctx context.Context, c *config.Config, router *httprouter.Router, cm consent.Manager, o fosite.OAuth2Provider) *oauth2.Handler {
	expectDependency(c.GetLogger(), c.Context().FositeStore)

	c.ConsentURL = setDefaultConsentURL(c.ConsentURL, c, "oauth2/fallbacks/consent")
	c.LoginURL = setDefaultConsentURL(c.LoginURL, c, "oauth2/fallbacks/consent")
	c.ErrorURL = setDefaultConsentURL(c.ErrorURL, c, "oauth2/fallbacks/error")

	errorURL, err := url.Parse(c.ErrorURL)
	pkg.Must(err, "Could not parse error url %s.", errorURL)

	oidcStrategy := fgoauth2.NewOpenIDConnectStrategy(ctx)
	oidcStrategy.Issuer = c.Issuer

	w := herodot.NewJSONWriter(c.GetLogger())
	w.ErrorEnhancer = writerErrorEnhancer

	var accessTokenJWTStrategy *jwk.RS256JWTStrategy

	if c.OAuth2AccessTokenStrategy == "jwt" {
		oauth2Strategy := fgoauth2.NewOAuth2GCPStrategy(ctx, nil)
		oauth2Strategy.Issuer = c.Issuer
		//accessTokenJWTStrategy = oauth2Strategy.JWTStrategy
	}

	handler := &oauth2.Handler{
		ScopesSupported:  c.OpenIDDiscoveryScopesSupported,
		UserinfoEndpoint: c.OpenIDDiscoveryUserinfoEndpoint,
		ClaimsSupported:  c.OpenIDDiscoveryClaimsSupported,
		ForcedHTTP:       c.ForceHTTP,
		OAuth2:           o,
		ScopeStrategy:    c.GetScopeStrategy(),
		Consent: consent.NewStrategy(
			c.LoginURL, c.ConsentURL, c.Issuer,
			"/oauth2/auth", cm,
			sessions.NewCookieStore(c.GetCookieSecret()), c.GetScopeStrategy(),
			!c.ForceHTTP, time.Minute*15,
			oidcStrategy,
			openid.NewOpenIDConnectRequestValidator(nil, oidcStrategy),
		),
		Storage:             c.Context().FositeStore,
		ErrorURL:            *errorURL,
		H:                   w,
		AccessTokenLifespan: c.GetAccessTokenLifespan(),
		CookieStore:         sessions.NewCookieStore(c.GetCookieSecret()),
		IssuerURL:           c.Issuer,
		//OpenIDJWTStrategy:      oidcStrategy.JWTStrategy,
		AccessTokenJWTStrategy: accessTokenJWTStrategy,
		IDTokenLifespan:        c.GetIDTokenLifespan(),
	}

	handler.SetRoutes(router)
	return handler
}
