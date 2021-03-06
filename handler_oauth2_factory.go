package hydragcp

import (
	"context"
	"strings"
	"time"

	"github.com/gorilla/sessions"
	"github.com/ory/fosite"
	"github.com/ory/fosite/compose"
	foauth2 "github.com/ory/fosite/handler/oauth2"
	"github.com/ory/fosite/handler/openid"
	"github.com/ory/go-convenience/stringslice"
	"github.com/ory/hydra/cmd/server"
	"github.com/ory/hydra/config"
	"github.com/ory/hydra/consent"
	"github.com/ory/hydra/jwk"
	"github.com/ory/hydra/oauth2"
	"go.opencensus.io/trace"

	fgoauth2 "github.com/someone1/fosite-gcp-oauth2"
)

func newOAuth2Provider(ctxx context.Context, c *config.Config, jwtStrat jwk.JWTStrategy) fosite.OAuth2Provider {
	var ctx = c.Context()
	var store = ctx.FositeStore
	attrs := []trace.Attribute{trace.Int64Attribute("bcrypt.workfactor", int64(c.BCryptWorkFactor))}
	hasher := NewTracedHasher(&fosite.BCrypt{WorkFactor: c.BCryptWorkFactor}, attrs)
	ctx.Hasher = hasher

	fc := &compose.Config{
		AccessTokenLifespan:            c.GetAccessTokenLifespan(),
		AuthorizeCodeLifespan:          c.GetAuthCodeLifespan(),
		IDTokenLifespan:                c.GetIDTokenLifespan(),
		IDTokenIssuer:                  c.Issuer,
		HashCost:                       c.BCryptWorkFactor,
		ScopeStrategy:                  c.GetScopeStrategy(),
		SendDebugMessagesToClients:     c.SendOAuth2DebugMessagesToClients,
		EnforcePKCE:                    false,
		EnablePKCEPlainChallengeMethod: false,
		TokenURL:                       strings.TrimRight(c.Issuer, "/") + oauth2.TokenPath,
	}

	oidcStrategy := fgoauth2.NewOpenIDConnectStrategy(ctxx, jwtStrat)
	oidcStrategy.Issuer = c.Issuer

	var coreStrategy foauth2.CoreStrategy
	hmacStrategy := compose.NewOAuth2HMACStrategy(fc, c.GetSystemSecret(), nil)

	if c.OAuth2AccessTokenStrategy == "jwt" {
		oauth2Strategy := fgoauth2.NewOAuth2GCPStrategy(ctxx, jwtStrat, hmacStrategy)
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
			JWTStrategy:                jwtStrat,
		},
		hasher,
		compose.OAuth2AuthorizeExplicitFactory,
		compose.OAuth2AuthorizeImplicitFactory,
		compose.OAuth2ClientCredentialsGrantFactory,
		compose.OAuth2RefreshTokenGrantFactory,
		compose.OpenIDConnectExplicitFactory,
		compose.OpenIDConnectHybridFactory,
		compose.OpenIDConnectImplicitFactory,
		compose.OpenIDConnectRefreshFactory,
		compose.OAuth2TokenRevocationFactory,
		compose.OAuth2TokenIntrospectionFactory,
		compose.OAuth2PKCEFactory,
	)
}

func injectGCPOauth2(ctx context.Context, handler *server.Handler, c *config.Config, jwtStrat jwk.JWTStrategy) {
	cm := c.Context().ConsentManager

	handler.OAuth2.OAuth2 = newOAuth2Provider(ctx, c, jwtStrat)

	oidcStrategy := fgoauth2.NewOpenIDConnectStrategy(ctx, jwtStrat)
	oidcStrategy.Issuer = c.Issuer

	handler.OAuth2.OpenIDJWTStrategy = oidcStrategy

	sias := map[string]consent.SubjectIdentifierAlgorithm{}
	if stringslice.Has(c.GetSubjectTypesSupported(), "pairwise") {
		sias["pairwise"] = consent.NewSubjectIdentifierAlgorithmPairwise([]byte(c.SubjectIdentifierAlgorithmSalt))
	}
	if stringslice.Has(c.GetSubjectTypesSupported(), "public") {
		sias["public"] = consent.NewSubjectIdentifierAlgorithmPublic()
	}

	handler.OAuth2.Consent = consent.NewStrategy(
		c.LoginURL, c.ConsentURL, c.Issuer,
		"/oauth2/auth", cm,
		sessions.NewCookieStore(c.GetCookieSecret()), c.GetScopeStrategy(),
		!c.ForceHTTP, time.Minute*15,
		oidcStrategy,
		openid.NewOpenIDConnectRequestValidator(nil, oidcStrategy),
		sias,
	)

	if c.OAuth2AccessTokenStrategy == "jwt" {
		oauth2Strategy := fgoauth2.NewOAuth2GCPStrategy(ctx, jwtStrat, nil)
		oauth2Strategy.Issuer = c.Issuer
		handler.OAuth2.AccessTokenJWTStrategy = oauth2Strategy
	}
}
