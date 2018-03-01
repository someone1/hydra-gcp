# Hydra for GCP - WORK IN PROGRESS

## Important Notes (READ FIRST!)

First and foremost, Hydra is capable of being run on GCP **without** modification. This package is an attempt to bootstrap Hydra utilizing GCP's existing authentication and storage infrastructure (IAM API's signJwt/Cloud Datastore) and not meant to be used unless you know what you are doing. Some caveats with this:

* JWTs are utilized for Access Tokens, everything else is using opaque HMAC tokens (hydra default). This has its own implications you should be aware of and comfortable with.
* Hydra has NO knowledge of the keys used for Access Tokens (all keys are managed and roated by GCP). As such, the JWK API features of hydra are disabled (e.g. the entire /key API for managing keys with Hydra) and the well known configuration path redirects to GCP's JWK URL (https://www.googleapis.com/service_accounts/v1/jwk/<service-account>)
* This will NOT start its own server proccess, instead a `http.Handler` is provided to you so you can load your own (e.g. useful when using AppEngine Flexible).
* Access Token max age is 1 hour (limit enforced by the IAM API)

## Why make this?

Well despite the JWT vs opaque token (arguable) con, you get some pros:

* You can use the Access Tokens in conjuction with Cloud Endpoints! You don't have to use Firebase, Auth0, or Oauth2/OpenID providers such as Google. You can place hydra ontop of your existing user authentication systems and go from there.
* You don't manage (creating, storing, rotating, etc.) access token keys/secrets! This is hard to get right so offloading this to a robust, battletested system goes a long way.
* If you already leverage and utilize datastore, you don't need to add another database (MySQL/PostgreSQL)

## Goals

Try and make as FEW changes and copy as LITTLE as possible of the original Hydra bootstrap process. All we really want to do is plug in a different signing mechanism for access tokens and store configurations/sessions in datastore, everything else should work exactly as-is from within Hydra.

Maybe hydra will accept a PR for at least the datastore backend?

## Usage

Onto the good part. There are two things you are responsible for providing to this package in order to boostrap it:

1. A [Hydra Config](https://godoc.org/github.com/ory/hydra/config#Config) - The environmental variables/configuration available out of the box from Hydra is not used in this package. Since we don't run a local server, the server related options are generally ignored (e.g. TLS cert config, host/port, etc.). Note: This package forces it's own BuildVersion
2. A `context.Context` with a configuration to be used with jwt-go (see the [gcp-jwt-go pacakage](https://github.com/someone1/gcp-jwt-go))
3. Whether or not you'd like to disable the telemetry feature
4. CORS options you'd like to use (you can see what Hydra does [here](https://github.com/ory/hydra/blob/master/cmd/server/handler.go#L48)

That's about it. You can continue to use your own web framework so long as you're aware of the handlers already implemented by hydra (basically everything [here](https://www.ory.sh/docs/api/hydra) except for the `/keys.*` endpoints)

example:

```go
package main

import (
    "github.com/guregu/kami"
    //...
)
func main() {
    // You can set these up in an app.yaml if using AppEngine Flexible (STANDARD DOESN'T WORK!)
	c := &config.Config{
		DatabaseURL:         os.Getenv("DATABASE_URL"),
		SystemSecret:        os.Getenv("SYSTEM_SECRET"),
		CookieSecret:        os.Getenv("COOKIE_SECRET"),
		Issuer:              os.Getenv("ISSUER"),
		ConsentURL:          os.Getenv("CONSENT_URL"),
		BCryptWorkFactor:    bcrypt.DefaultCost,
		AccessTokenLifespan: "5m",
		AllowTLSTermination: "0.0.0.0/0", // Might be a better option here?
	}

	if appengine.IsDevAppServer() {
		c.ForceHTTP = true
	}

	ctx := gcp_jwt.NewContextJWT(kami.Context, &gcp_jwt.IAMSignJWTConfig{ServiceAccount: "<name>@<project>.iam.gserviceaccount.com"})

	handler := hydragcp.GenerateHydraHandler(ctx, c, false, cors.Options{})

	kami.Get("/consent", consentHandler)

	http.Handle("/", handler)
	http.Handle("/consent", kami.Handler())

	appengine.Main()
}
```
