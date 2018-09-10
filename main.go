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

	dconfig "github.com/someone1/hydra-gcp/config"
)

func init() {
	config.RegisterBackend(&dconfig.DatastoreConnection{})
}

// GenerateHydraHandler will bootstrap Hydra and return a http.Handler for you to use.
func GenerateHydraHandler(ctx context.Context, c *config.Config, h herodot.Writer, enableCors bool) (http.Handler, http.Handler) {
	c.BuildVersion = "hydra-gcp"
	handler := server.NewHandler(c, h)

	frontend := httprouter.New()
	backend := httprouter.New()

	handler.RegisterRoutes(frontend, backend)

	enhancedFrontend := server.EnhanceRouter(c, nil, handler, frontend, nil, false)
	enhanceBackend := server.EnhanceRouter(c, nil, handler, backend, nil, enableCors)

	jwtConfig, ok := gcp_jwt.FromContextJWT(ctx)
	if !ok {
		panic("must send a context with a IAMSignJWTConfig embedded")
	}

	injectGCPOauth2(ctx, handler, c)

	serveMux := http.NewServeMux()
	serveMux.HandleFunc(jwk.WellKnownKeysPath, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, fmt.Sprintf("https://www.googleapis.com/service_accounts/v1/jwk/%s", jwtConfig.ServiceAccount), http.StatusTemporaryRedirect)
	})
	serveMux.Handle("/", enhancedFrontend)

	return serveMux, enhanceBackend
}
