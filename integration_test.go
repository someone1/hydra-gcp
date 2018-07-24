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
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/ory/hydra/config"
	"github.com/ory/hydra/health"
	"github.com/ory/hydra/jwk"
	sdk "github.com/ory/hydra/sdk/go/hydra"
	swagger "github.com/ory/hydra/sdk/go/hydra/swagger"
	"github.com/pkg/errors"
	"github.com/rs/cors"
	"github.com/someone1/gcp-jwt-go"
	"golang.org/x/crypto/bcrypt"
	goauth2 "golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
	"golang.org/x/oauth2/google"
)

func generateGCPHydraHandler(t *testing.T) (context.Context, http.Handler) {
	t.Helper()

	c := &config.Config{
		DatabaseURL:               os.Getenv("DATABASE_URL"),
		SystemSecret:              os.Getenv("SYSTEM_SECRET"),
		CookieSecret:              os.Getenv("SYSTEM_SECRET"),
		Issuer:                    os.Getenv("ISSUER"),
		ConsentURL:                os.Getenv("CONSENT_URL"),
		OAuth2AccessTokenStrategy: "jwt",
		AllowTLSTermination:       "0.0.0.0/0",
		BCryptWorkFactor:          bcrypt.DefaultCost,
		LogLevel:                  "debug",
		AccessTokenLifespan:       "5m",
	}

	credsFile, err := ioutil.ReadFile(os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"))
	if err != nil {
		t.Fatalf("could not read credentialsFile: %v", err)
	}

	jwtConfig, err := google.JWTConfigFromJSON(credsFile)
	if err != nil {
		t.Fatalf("could not get jwt config: %v", err)
	}
	ctx := context.WithValue(context.Background(), goauth2.HTTPClient, http.DefaultClient)
	ctx = gcp_jwt.NewContextJWT(ctx, &gcp_jwt.IAMSignJWTConfig{ServiceAccount: jwtConfig.Email})

	return ctx, GenerateHydraHandler(ctx, c, cors.Options{})
}

func getHydraSDKClient(t *testing.T, client *http.Client, basePath string) *sdk.CodeGenSDK {
	t.Helper()
	// Let's get Hydra SDK
	hydraConfig := &sdk.Configuration{
		EndpointURL: basePath,
	}
	hydraClient, err := sdk.NewSDK(hydraConfig)
	if err != nil {
		t.Fatalf("could not get hydra sdk client: %v", err)
	}

	hydraClient.OAuth2Api.Configuration.Transport = client.Transport
	hydraClient.JsonWebKeyApi.Configuration.Transport = client.Transport
	return hydraClient
}

func TestIntegration(t *testing.T) {
	ctx, hydra := generateGCPHydraHandler(t)
	ts := httptest.NewTLSServer(hydra)
	defer ts.Close()
	client := ts.Client()
	ctx = context.WithValue(ctx, goauth2.HTTPClient, client)
	hydraClient := getHydraSDKClient(t, client, ts.URL)

	// TODO: Come up with some tests...

	t.Run("Health", func(t *testing.T) {
		for _, path := range []string{health.ReadyCheckPath, health.AliveCheckPath} {
			res, err := client.Get(ts.URL + path)
			if err != nil {
				t.Fatalf("could not get to endpoint %s due to error %v", path, err)
			}
			response, err := ioutil.ReadAll(res.Body)
			res.Body.Close()
			if err != nil {
				t.Fatalf("could not get body of request due to error %v", err)
			}
			if string(response) != `{"status":"ok"}` {
				t.Errorf(`expected {"status":"ok"} but got %s instead`, response)
			}
		}
	})

	t.Run("TLS", func(t *testing.T) {
		// Normal HTTP Request
		req := httptest.NewRequest("", "/test", nil)
		w := httptest.NewRecorder()
		hydra.ServeHTTP(w, req)
		if w.Code != http.StatusBadGateway {
			t.Errorf("expected status code %d, got %d", http.StatusBadGateway, w.Code)
		}

		// TLS Terminated
		req = httptest.NewRequest("", "/test", nil)
		req.Header.Set("X-Forwarded-Proto", "https")
		w = httptest.NewRecorder()
		hydra.ServeHTTP(w, req)
		if w.Code != http.StatusNotFound {
			t.Errorf("expected status code %d, got %d", http.StatusNotFound, w.Code)
		}
	})

	t.Run("JWK", func(t *testing.T) {
		var lastRedirect string
		var errRedirect = errors.New("cancel redirect")
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			lastRedirect = req.URL.String()
			return errRedirect
		}
		defer func() {
			client.CheckRedirect = nil
		}()
		jconfig, ok := gcp_jwt.FromContextJWT(ctx)
		if !ok {
			t.Errorf("could not get JWT config from context")
			return
		}

		for _, path := range []string{jwk.WellKnownKeysPath} {
			_, err := client.Get(ts.URL + path)
			if err == nil || !strings.Contains(err.Error(), errRedirect.Error()) {
				t.Fatalf("could not get to endpoint %s due to error %v", path, err)
			}
			if want := fmt.Sprintf("https://www.googleapis.com/service_accounts/v1/jwk/%s", jconfig.ServiceAccount); lastRedirect != want {
				t.Errorf("wanted %s, got %s", want, lastRedirect)
			}
		}
	})

	t.Run("Clients", func(t *testing.T) {
		clients, _, err := hydraClient.ListOAuth2Clients(100, 0)
		if err != nil {
			t.Errorf("got an error while listing clients: %v", err)
		}

		if len(clients) != 0 {
			t.Errorf("expected no clients, got %d", len(clients))
		}

		client, _, err := hydraClient.CreateOAuth2Client(swagger.OAuth2Client{
			ClientName:   "test",
			ClientId:     "test",
			ClientSecret: "password",
		})

		if err != nil {
			t.Errorf("got an error while creating a client: %v", err)
		}

		clients, _, err = hydraClient.ListOAuth2Clients(100, 0)
		if err != nil {
			t.Errorf("got an error while listing clients: %v", err)
		}

		if len(clients) != 1 {
			t.Fatalf("expected 1 client, got %d", len(clients))
		}

		if clients[0].ClientId != client.ClientId {
			t.Errorf("expected client id %s, got %s", client.ClientId, clients[0].ClientId)
			t.Errorf("%#v", client)
			t.Errorf("%#v", clients)
		}

		client, _, err = hydraClient.GetOAuth2Client(client.ClientId)
		if err != nil {
			t.Errorf("got an error while getting clients: %v", err)
		}

		if clients[0].ClientId != client.ClientId {
			t.Errorf("expected client id %s, got %s", client.ClientId, clients[0].ClientId)
		}

		client.Owner = "test"
		_, _, err = hydraClient.UpdateOAuth2Client(client.ClientId, *client)
		if err != nil {
			t.Errorf("got an error while updating clients: %v", err)
		}

		_, err = hydraClient.DeleteOAuth2Client(client.ClientId)
		if err != nil {
			t.Errorf("got an error while deleting clients: %v", err)
		}

		clients, _, err = hydraClient.ListOAuth2Clients(100, 0)
		if err != nil {
			t.Errorf("got an error while listing clients: %v", err)
		}

		if len(clients) != 0 {
			t.Errorf("expected no clients, got %d", len(clients))
		}
	})

	t.Run("Oauth2", func(t *testing.T) {
		client, _, err := hydraClient.CreateOAuth2Client(swagger.OAuth2Client{
			ClientName:    "test",
			ClientId:      "test",
			ClientSecret:  "password",
			GrantTypes:    []string{"authorize_code", "client_credentials"},
			ResponseTypes: []string{"code", "token", "id_token"},
		})
		if err != nil {
			t.Errorf("got an error while creating a client: %v", err)
		}

		oauth2Config := &clientcredentials.Config{
			ClientID:     "test",
			ClientSecret: "password",
			TokenURL:     hydraClient.Configuration.EndpointURL + "/oauth2/token",
		}

		tkn, err := oauth2Config.Token(ctx)
		if err != nil {
			t.Fatalf("could not get token: %v", err)
		}
		oauthClient := oauth2Config.Client(ctx)
		hydraClient.OAuth2Api.Configuration.Transport = oauthClient.Transport

		resp, _, err := hydraClient.IntrospectOAuth2Token(tkn.AccessToken, "")
		if err != nil {
			t.Fatalf("could not introspect token: %v", err)
		}
		if resp.ClientId != client.ClientId {
			t.Errorf("expected client id %s, got %s instead", client.ClientId, resp.ClientId)
		}
	})
}
