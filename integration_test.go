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
	"testing"

	"github.com/ory/hydra/config"
	"github.com/ory/hydra/health"
	"github.com/rs/cors"
	"github.com/someone1/gcp-jwt-go"
	"golang.org/x/crypto/bcrypt"
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
	ctx := gcp_jwt.NewContextJWT(context.Background(), &gcp_jwt.IAMSignJWTConfig{ServiceAccount: "test@serviceaccount.com"})

	return GenerateHydraHandler(ctx, c, false, cors.Options{})
}

func TestIntegration(t *testing.T) {
	hydra := generateGCPHydraHandler(t)
	ts := httptest.NewTLSServer(hydra)
	defer ts.Close()
	client := ts.Client()

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

	})
}
