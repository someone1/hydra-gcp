package hydragcp

import (
	"context"
	"net/http"
	"net/url"

	"github.com/julienschmidt/httprouter"
	negronilogrus "github.com/meatballhat/negroni-logrus"
	"github.com/ory/herodot"
	"github.com/ory/hydra/config"
	"github.com/ory/hydra/pkg"
	"github.com/rs/cors"
	"github.com/urfave/negroni"

	dconfig "github.com/someone1/hydra-gcp/config"
)

// GenerateHydraHandler will bootstrap Hydra and return a http.Handler for you to use.
func GenerateHydraHandler(ctx context.Context, c *config.Config, corsOpts cors.Options) http.Handler {
	router := httprouter.New()
	logger := c.GetLogger()
	w := herodot.NewJSONWriter(logger)
	w.ErrorEnhancer = nil

	serverHandler := &Handler{
		Config: c,
		H:      w,
	}

	// Let's Hijack the database options
	if c.DatabaseURL != "" && c.DatabaseURL != "memory" {
		u, err := url.Parse(c.DatabaseURL)
		if err != nil {
			c.GetLogger().Fatalf("Could not parse DATABASE_URL: %s", err)
		}
		if u.Scheme == "datastore" {
			// Switch to a memory database and override with datastore options
			c.GetLogger().Infof("Setting up Datastore connections...")
			old := c.DatabaseURL
			c.DatabaseURL = "memory"
			gctx := c.Context()
			c.DatabaseURL = old
			dm, err := dconfig.NewDatastoreConnection(ctx, u, c.GetLogger())
			if err != nil {
				c.GetLogger().Fatal(err)
			}
			gctx.Connection = dm
			c.GetLogger().Infof("Switched from memory database to datastore")
		}
	}

	serverHandler.registerRoutes(ctx, router)

	if !c.ForceHTTP {
		if c.Issuer == "" {
			logger.Fatalln("IssuerURL must be explicitly specified unless --dangerous-force-http is passed. To find out more, use `hydra help host`.")
		}
		issuer, err := url.Parse(c.Issuer)
		pkg.Must(err, "Could not parse issuer URL: %s", err)
		if issuer.Scheme != "https" {
			logger.Fatalln("IssuerURL must use HTTPS unless --dangerous-force-http is passed. To find out more, use `hydra help host`.")
		}
	}

	n := negroni.New()
	n.Use(negronilogrus.NewMiddlewareFromLogger(logger, c.Issuer))
	n.Use(c.GetPrometheusMetrics())

	n.UseFunc(serverHandler.rejectInsecureRequests)
	n.UseHandler(router)
	corsHandler := cors.New(corsOpts).Handler(n)

	c.BuildVersion = "hydra-gcp"

	return corsHandler
}
