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

	"github.com/gorilla/sessions"
	"github.com/julienschmidt/httprouter"
	"github.com/ory/fosite"
	"github.com/ory/fosite/compose"
	"github.com/ory/herodot"
	"github.com/ory/hydra/client"
	"github.com/ory/hydra/config"
	"github.com/ory/hydra/oauth2"
	"github.com/ory/hydra/pkg"
	"github.com/ory/hydra/warden"

	foauth2 "github.com/someone1/forsite-gcp-oauth2"
	dconfig "github.com/someone1/hydra-gcp/config"
	doauth2 "github.com/someone1/hydra-gcp/oauth2"
)

func injectFositeStore(c *config.Config, clients client.Manager) {
	var ctx = c.Context()
	var store pkg.FositeStorer

	switch con := ctx.Connection.(type) {
	case *config.MemoryConnection:
		store = oauth2.NewFositeMemoryStore(clients, c.GetAccessTokenLifespan())
		break
	case *config.SQLConnection:
		store = oauth2.NewFositeSQLStore(clients, con.GetDatabase(), c.GetLogger(), c.GetAccessTokenLifespan())
		break
	case *config.PluginConnection:
		var err error
		if store, err = con.NewOAuth2Manager(clients); err != nil {
			c.GetLogger().Fatalf("Could not load client manager plugin %s", err)
		}
		break
	case *dconfig.DatastoreConnection:
		store = doauth2.NewFositeDatastoreStore(clients, con.Client(), con.Namespace(), c.GetLogger(), c.GetAccessTokenLifespan())
	default:
		panic("Unknown connection type.")
	}

	ctx.FositeStore = store
}

func newOAuth2Provider(ctxx context.Context, c *config.Config) fosite.OAuth2Provider {
	var ctx = c.Context()
	var store = ctx.FositeStore

	fc := &compose.Config{
		AccessTokenLifespan:            c.GetAccessTokenLifespan(),
		AuthorizeCodeLifespan:          c.GetAuthCodeLifespan(),
		IDTokenLifespan:                c.GetIDTokenLifespan(),
		HashCost:                       c.BCryptWorkFactor,
		ScopeStrategy:                  c.GetScopeStrategy(),
		SendDebugMessagesToClients:     c.SendOAuth2DebugMessagesToClients,
		EnforcePKCE:                    false,
		EnablePKCEPlainChallengeMethod: false,
	}

	oauth2strat := foauth2.NewOAuth2GCPStrategy(ctxx, compose.NewOAuth2HMACStrategy(fc, c.GetSystemSecret()))
	openidstrat := foauth2.NewOpenIDConnectStrategy(ctxx)
	oauth2strat.Issuer = c.Issuer
	openidstrat.Issuer = c.Issuer

	return compose.Compose(
		fc,
		store,
		&compose.CommonStrategy{
			CoreStrategy:               oauth2strat,
			OpenIDConnectTokenStrategy: openidstrat,
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
		compose.OAuth2TokenRevocationFactory,
		warden.OAuth2TokenIntrospectionFactory,
	)
}

func newOAuth2Handler(c *config.Config, router *httprouter.Router, cm oauth2.ConsentRequestManager, o fosite.OAuth2Provider) *oauth2.Handler {
	if c.ConsentURL == "" {
		proto := "https"
		if c.ForceHTTP {
			proto = "http"
		}
		host := "localhost"
		if c.BindHost != "" {
			host = c.BindHost
		}
		c.ConsentURL = fmt.Sprintf("%s://%s:%d/oauth2/consent", proto, host, c.BindPort)
	}

	consentURL, err := url.Parse(c.ConsentURL)
	pkg.Must(err, "Could not parse consent url %s.", c.ConsentURL)

	handler := &oauth2.Handler{
		ScopesSupported:  c.OpenIDDiscoveryScopesSupported,
		UserinfoEndpoint: c.OpenIDDiscoveryUserinfoEndpoint,
		ClaimsSupported:  c.OpenIDDiscoveryClaimsSupported,
		ForcedHTTP:       c.ForceHTTP,
		OAuth2:           o,
		ScopeStrategy:    c.GetScopeStrategy(),
		Consent: &oauth2.DefaultConsentStrategy{
			Issuer:                   c.Issuer,
			ConsentManager:           c.Context().ConsentManager,
			DefaultChallengeLifespan: c.GetChallengeTokenLifespan(),
			DefaultIDTokenLifespan:   c.GetIDTokenLifespan(),
		},
		Storage:             c.Context().FositeStore,
		ConsentURL:          *consentURL,
		H:                   herodot.NewJSONWriter(c.GetLogger()),
		AccessTokenLifespan: c.GetAccessTokenLifespan(),
		CookieStore:         sessions.NewCookieStore(c.GetCookieSecret()),
		Issuer:              c.Issuer,
		L:                   c.GetLogger(),
		W:                   c.Context().Warden,
		ResourcePrefix:      c.AccessControlResourcePrefix,
	}

	handler.SetRoutes(router)
	return handler
}
