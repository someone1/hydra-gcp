# Hydra for GCP - WORK IN PROGRESS [![Go Report Card](https://goreportcard.com/badge/github.com/someone1/hydra-gcp)](https://goreportcard.com/report/github.com/someone1/hydra-gcp) [![Build Status](https://travis-ci.org/someone1/hydra-gcp.svg?branch=master)](https://travis-ci.org/someone1/hydra-gcp) [![Coverage Status](https://coveralls.io/repos/github/someone1/hydra-gcp/badge.svg?branch=master)](https://coveralls.io/github/someone1/hydra-gcp?branch=master)

## Important Notes (READ FIRST!)

First and foremost, Hydra is capable of being run on GCP **without** modification. This package is an attempt to bootstrap Hydra utilizing GCP's existing authentication and storage infrastructure (IAM API's signJwt/Cloud Datastore) and not meant to be used unless you know what you are doing. Some caveats with this:

- JWTs are utilized for Access Tokens, everything else is using opaque HMAC tokens (hydra default). This has its own implications you should be aware of and comfortable with.
- Hydra has NO knowledge of the keys used for Access Tokens (all keys are managed and roated by GCP). As such, the JWK API features of hydra are disabled (e.g. the entire /key API for managing keys with Hydra) and the well known configuration path redirects to GCP's JWK URL (https://www.googleapis.com/service_accounts/v1/jwk/<service-account>)
- This will NOT start its own server proccess, instead two `http.Handler` (frontend, backend) are provided to you so you can load your own (e.g. useful when using AppEngine Flexible, adding your own middleware).
- Access Token max age is 1 hour (limit enforced by the IAM API)

## Why make this?

Well despite the JWT vs opaque token (arguable) con, you get some pros:

- You don't manage (creating, storing, rotating, etc.) access token keys/secrets! This is hard to get right so offloading this to a robust, battletested system goes a long way.
- If you already leverage and utilize datastore, you don't need to add another database (MySQL/PostgreSQL) - and this thing can SCALE

## Goals

Try and make as FEW changes and copy as LITTLE as possible of the original Hydra bootstrap process. All we really want to do is plug in a different signing mechanism for access tokens and store configurations/sessions in datastore, everything else should work exactly as-is from within Hydra.

### Interested in the datastore only? Check out how to [build the plugin](https://github.com/someone1/hydra-gcp/tree/master/plugin)

## Usage

Onto the good part. There are two things you are responsible for providing to this package in order to boostrap it:

1. A [Hydra Config](https://godoc.org/github.com/ory/hydra/config#Config) - The environmental variables/configuration available out of the box from Hydra is not used in this package. Since we don't run a local server, the server related options are generally ignored (e.g. TLS cert config, host/port, etc.). Note: This package forces it's own BuildVersion
2. A `context.Context` with a configuration to be used with jwt-go (see the [gcp-jwt-go pacakage](https://github.com/someone1/gcp-jwt-go))
3. CORS options you'd like to use (you can see what Hydra does [here](https://github.com/ory/hydra/blob/master/cmd/server/handler.go#L48)
4. Add the following to your Datastore indexes (index.yaml):

```yaml
indexes:
  - kind: HydraOauth2Access
    ancestor: yes
    properties:
      - name: rat

  - kind: HydraJWK
    ancestor: yes
    properties:
      - name: created_at
        direction: desc
```

That's about it. You can continue to use your own web framework so long as you're aware of the handlers already implemented by hydra (basically everything [here](https://www.ory.sh/docs/api/hydra). What's not supported:

- JWK related API/services including OAuth 2.0 Client Authentication with RSA private/public keypairs
- System Secret Rotation
- Removed prometheus metrics (you can add this back with your own middleware)

example:

```go
package main

import (
	"github.com/justinas/alice"
	"github.com/ory/herodot"
	"github.com/ory/hydra/config"
	"github.com/someone1/gcp-jwt-go"
	"github.com/someone1/hydra-gcp"
	"google.golang.org/appengine"
	//...
)
func main() {
	// You can set these up in an app.yaml if using AppEngine Flexible (STANDARD DOESN'T WORK!)
	c := &config.Config{
		DatabaseURL:               os.Getenv("DATABASE_URL"),
		SystemSecret:              os.Getenv("SYSTEM_SECRET"),
		CookieSecret:              os.Getenv("SYSTEM_SECRET"),
		Issuer:                    os.Getenv("ISSUER"),
		ConsentURL:                os.Getenv("CONSENT_URL"),
		LoginURL:                  os.Getenv("LOGIN_URL"),
		BCryptWorkFactor:          bcrypt.DefaultCost,
		OAuth2AccessTokenStrategy: "jwt",
		SubjectTypesSupported:     "public",
		LogLevel:                  "info",
		AccessTokenLifespan:       "5m",
		AuthCodeLifespan:          "10m",
		IDTokenLifespan:           "1h",
		AllowTLSTermination:       "0.0.0.0/0",
	}

	if appengine.IsDevAppServer() {
		c.ForceHTTP = true
	}

	gcpconfig := &gcpjwt.IAMConfig{ServiceAccount: os.Getenv("API_SERVICE_ACCOUNT")}
	ctx := context.Background()

	logger := c.GetLogger()
	w := herodot.NewJSONWriter(logger)

	frontend, backend := hydragcp.GenerateIAMHydraHandler(ctx, c, gcpconfig, w, true)
	// Protect the backend with something like "github.com/someone1/gcp-jwt-go/jwtmiddleware"

	// If we want to host both frontend and backend on the same port - PROTECT THE BACKEND!
	combinedMux := http.NewServeMux()
	combinedMux.Handle("/oauth2/", frontend)
	combinedMux.Handle("/oauth2/auth/sessions/login/revoke", frontend)
	combinedMux.Handle("/.well-known/", wellKnownCORSHandler(frontend)) // CORS issue w/ hydra, see #1028
	combinedMux.Handle("/userinfo", frontend)
	combinedMux.Handle(consent.LoginPath+"/", backend)
	combinedMux.Handle(consent.ConsentPath+"/", backend)
	combinedMux.Handle("/oauth2/auth/sessions/", backend)
	combinedMux.Handle(oauth2.IntrospectPath, backend)
	combinedMux.Handle(oauth2.FlushPath, backend)
	combinedMux.Handle("/", backend)

	http.Handle("/", combinedMux)


	appengine.Main() // Or any graceful web server listening on port 8080
}
```
