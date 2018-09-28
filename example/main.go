package main

import (
	"context"
	"net/http"
	"os"
	"sync"

	"github.com/ory/graceful"
	"github.com/ory/herodot"
	"github.com/ory/hydra/config"
	"github.com/someone1/gcp-jwt-go"
	"github.com/someone1/hydra-gcp"
	"github.com/spf13/viper"
)

func main() {
	viper.AutomaticEnv()

	c := &config.Config{
		DatabaseURL:               os.Getenv("DATABASE_URL"),
		SystemSecret:              os.Getenv("SYSTEM_SECRET"),
		CookieSecret:              os.Getenv("SYSTEM_SECRET"),
		Issuer:                    os.Getenv("ISSUER"),
		ConsentURL:                os.Getenv("CONSENT_URL"),
		LoginURL:                  os.Getenv("LOGIN_URL"),
		BCryptWorkFactor:          viper.GetInt("BCRYPT_COST"),
		FrontendBindPort:          viper.GetInt("PUBLIC_PORT"),
		BackendBindPort:           viper.GetInt("ADMIN_PORT"),
		OAuth2AccessTokenStrategy: "jwt",
		SubjectTypesSupported:     "public",
		LogLevel:                  "info",
		AccessTokenLifespan:       "5m",
		AuthCodeLifespan:          "10m",
		IDTokenLifespan:           "1h",
		AllowTLSTermination:       "0.0.0.0/0",
		ForceHTTP:                 true,
	}

	gcpconfig := &gcpjwt.IAMConfig{ServiceAccount: os.Getenv("API_SERVICE_ACCOUNT")}
	ctx := context.Background()

	logger := c.GetLogger()
	w := herodot.NewJSONWriter(logger)

	frontend, backend := hydragcp.GenerateIAMHydraHandler(ctx, c, gcpconfig, w, true)
	// Protect the backend with something like "github.com/someone1/gcp-jwt-go/jwtmiddleware"

	var wg sync.WaitGroup
	wg.Add(2)
	go serve(c, frontend, c.GetFrontendAddress(), &wg)
	go serve(c, backend, c.GetBackendAddress(), &wg)

	wg.Wait()
}

// Taken from Hydra - modified to remove HTTPS bits
func serve(c *config.Config, handler http.Handler, address string, wg *sync.WaitGroup) {
	defer wg.Done()

	var srv = graceful.WithDefaults(&http.Server{
		Addr:    address,
		Handler: handler,
	})

	err := graceful.Graceful(func() error {
		var err error
		c.GetLogger().Infof("Setting up http server on %s", address)
		if c.ForceHTTP {
			c.GetLogger().Warnln("HTTPS disabled. Never do this in production.")
			err = srv.ListenAndServe()
		} else if c.AllowTLSTermination != "" {
			c.GetLogger().Infoln("TLS termination enabled, disabling https.")
			err = srv.ListenAndServe()
		} else {
			err = srv.ListenAndServeTLS("", "")
		}

		return err
	}, srv.Shutdown)
	if err != nil {
		c.GetLogger().WithError(err).Fatal("Could not gracefully run server")
	}
}
