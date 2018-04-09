package hydragcp

import (
	"context"
	"net/http"
	"net/url"

	"github.com/julienschmidt/httprouter"
	negronilogrus "github.com/meatballhat/negroni-logrus"
	"github.com/ory/herodot"
	"github.com/ory/hydra/config"
	"github.com/rs/cors"
	"github.com/urfave/negroni"

	dconfig "github.com/someone1/hydra-gcp/config"
	dgroup "github.com/someone1/hydra-gcp/warden/group"
	ldatastore "github.com/someone1/ladon-datastore"
)

// GenerateHydraHandler will bootstrap Hydra and return a http.Handler for you to use.
func GenerateHydraHandler(ctx context.Context, c *config.Config, disableTelemetry bool, corsOpts cors.Options) http.Handler {
	router := httprouter.New()
	logger := c.GetLogger()
	serverHandler := &Handler{
		Config: c,
		H:      herodot.NewJSONWriter(logger),
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
			gctx.GroupManager = dgroup.NewDatastoreManager(dm.Context(), dm.Client(), dm.Namespace())
			gctx.LadonManager = ldatastore.NewManager(dm.Context(), dm.Client(), dm.Namespace())
			c.GetLogger().Infof("Switched from memory database to datastore")
		}
	}

	serverHandler.registerRoutes(ctx, router)

	n := negroni.New()

	if !disableTelemetry {
		metrics := c.GetMetrics()
		go metrics.RegisterSegment()
		go metrics.CommitMemoryStatistics()
		n.Use(metrics)
	}

	n.Use(negronilogrus.NewMiddlewareFromLogger(logger, c.Issuer))
	n.UseFunc(serverHandler.rejectInsecureRequests)
	n.UseHandler(router)
	corsHandler := cors.New(corsOpts).Handler(n)

	c.BuildVersion = "hydra-gcp"

	return corsHandler
}
