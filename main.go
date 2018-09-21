package hydragcp

import (
	"context"
	"fmt"
	"net/http"

	"github.com/julienschmidt/httprouter"
	"github.com/ory/herodot"
	"github.com/ory/hydra/cmd/server"
	"github.com/ory/hydra/config"
	"github.com/ory/hydra/jwk"
	"github.com/someone1/gcp-jwt-go"
	"github.com/spf13/viper"

	"github.com/someone1/fosite-gcp-oauth2"
	dconfig "github.com/someone1/hydra-gcp/config"
)

func init() {
	config.RegisterBackend(&dconfig.DatastoreConnection{})
}

// GenerateIAMHydraHandler will bootstrap Hydra using the IAM API to sign JWT AccessTokens and return http.Handlers for you to use.
func GenerateIAMHydraHandler(ctx context.Context, c *config.Config, gcpconfig *gcpjwt.IAMConfig, h herodot.Writer, enableCors bool) (http.Handler, http.Handler) {
	viper.AutomaticEnv()
	viper.Set("CORS_ENABLED", enableCors)

	c.BuildVersion = "hydra-gcp"
	handler := server.NewHandler(c, h)

	frontend := httprouter.New()
	backend := httprouter.New()

	handler.RegisterRoutes(frontend, backend)

	enhancedFrontend := server.EnhanceRouter(c, nil, handler, frontend, nil, false)
	enhanceBackend := server.EnhanceRouter(c, nil, handler, backend, nil, enableCors)

	jwtStrat := oauth2.NewIAMStrategy(ctx, gcpjwt.SigningMethodIAMJWT, gcpconfig)

	injectGCPOauth2(ctx, handler, c, jwtStrat)

	serveMux := http.NewServeMux()
	serveMux.HandleFunc(jwk.WellKnownKeysPath, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, fmt.Sprintf("https://www.googleapis.com/service_accounts/v1/jwk/%s", gcpconfig.ServiceAccount), http.StatusTemporaryRedirect)
	})
	serveMux.Handle("/", enhancedFrontend)

	return serveMux, enhanceBackend
}
