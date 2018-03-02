// Copyright © 2018 Prateek Malhotra (someone1@gmail.com)
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
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/ory/hydra/config"
	"github.com/ory/hydra/health"
	sdk "github.com/ory/hydra/sdk/go/hydra"
	"github.com/rs/cors"
	"github.com/someone1/gcp-jwt-go"
	"golang.org/x/crypto/bcrypt"
	goauth2 "golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

func generateGCPHydraHandler(t *testing.T) http.Handler {
	t.Helper()

	c := &config.Config{
		DatabaseURL:         os.Getenv("DATABASE_URL"),
		SystemSecret:        os.Getenv("SYSTEM_SECRET"),
		CookieSecret:        os.Getenv("SYSTEM_SECRET"),
		Issuer:              os.Getenv("ISSUER"),
		ConsentURL:          os.Getenv("CONSENT_URL"),
		BCryptWorkFactor:    bcrypt.DefaultCost,
		AccessTokenLifespan: "5m",
	}

	credsFile, err := ioutil.ReadFile(os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"))
	if err != nil {
		t.Fatalf("could not read credentialsFile: %v", err)
	}

	jwtConfig, err := google.JWTConfigFromJSON(credsFile)
	if err != nil {
		t.Fatalf("could not get jwt config: %v", err)
	}

	ctx := gcp_jwt.NewContextJWT(context.Background(), &gcp_jwt.IAMSignJWTConfig{ServiceAccount: jwtConfig.Email})

	return GenerateHydraHandler(ctx, c, false, cors.Options{})
}

func TestIntegration(t *testing.T) {
	hydra := generateGCPHydraHandler(t)
	ts := httptest.NewTLSServer(hydra)
	defer ts.Close()
	client := ts.Client()
	ctx := context.WithValue(context.Background(), goauth2.HTTPClient, client)

	forcedCreds := os.Getenv("FORCE_ROOT_CLIENT_CREDENTIALS")
	credsParts := strings.Split(forcedCreds, ":")
	if len(credsParts) != 2 {
		t.Fatalf("FORCE_ROOT_CLIENT_CREDENTIALS not found or incorrect")
	}

	// Let's get Hydra SDK
	hydraConfig := &sdk.Configuration{
		EndpointURL:  ts.URL,
		ClientID:     credsParts[0],
		ClientSecret: credsParts[1],
	}
	hydraClient, err := sdk.NewSDK(hydraConfig)
	if err != nil {
		t.Fatalf("could not get hydra sdk client: %v", err)
	}
	oauth2Config := hydraClient.GetOAuth2ClientConfig()
	// TODO: Come up with some tests...

	t.Run("Health", func(t *testing.T) {
		res, err := client.Get(ts.URL + health.HealthStatusPath)
		if err != nil {
			t.Fatalf("could not get to endpoint %s due to error %v", health.HealthStatusPath, err)
		}
		response, err := ioutil.ReadAll(res.Body)
		res.Body.Close()
		if err != nil {
			t.Fatalf("could not get body of request due to error %v", err)
		}
		if string(response) != `{"status": "ok"}` {
			t.Errorf(`expected {"status": "ok"} but got %s instead`, response)
		}
	})

	t.Run("Oauth2", func(t *testing.T) {
		_, err := oauth2Config.Token(ctx)
		if err != nil {
			t.Fatalf("could not get token: %v", err)
		}
	})
}
