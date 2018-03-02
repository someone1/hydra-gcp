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
	"github.com/ory/hydra/sdk/go/hydra/swagger"
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
	ctx := context.WithValue(context.Background(), goauth2.HTTPClient, http.DefaultClient)
	ctx = gcp_jwt.NewContextJWT(ctx, &gcp_jwt.IAMSignJWTConfig{ServiceAccount: jwtConfig.Email})

	return GenerateHydraHandler(ctx, c, false, cors.Options{})
}

func getHydraSDKClient(t *testing.T, ctx context.Context, basePath string) *sdk.CodeGenSDK {
	t.Helper()
	forcedCreds := os.Getenv("FORCE_ROOT_CLIENT_CREDENTIALS")
	credsParts := strings.Split(forcedCreds, ":")
	if len(credsParts) != 2 {
		t.Fatalf("FORCE_ROOT_CLIENT_CREDENTIALS not found or incorrect")
	}

	// Let's get Hydra SDK
	hydraConfig := &sdk.Configuration{
		EndpointURL:  basePath,
		ClientID:     credsParts[0],
		ClientSecret: credsParts[1],
	}
	hydraClient, err := sdk.NewSDK(hydraConfig)
	if err != nil {
		t.Fatalf("could not get hydra sdk client: %v", err)
	}

	oauth2Config := hydraClient.GetOAuth2ClientConfig()
	oauth2Client := oauth2Config.Client(ctx)
	hydraClient.OAuth2Api.Configuration.Transport = oauth2Client.Transport
	hydraClient.JsonWebKeyApi.Configuration.Transport = oauth2Client.Transport
	hydraClient.WardenApi.Configuration.Transport = oauth2Client.Transport
	hydraClient.PolicyApi.Configuration.Transport = oauth2Client.Transport
	return hydraClient
}

func TestIntegration(t *testing.T) {
	hydra := generateGCPHydraHandler(t)
	ts := httptest.NewTLSServer(hydra)
	defer ts.Close()
	client := ts.Client()
	ctx := context.WithValue(context.Background(), goauth2.HTTPClient, client)
	hydraClient := getHydraSDKClient(t, ctx, ts.URL)
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
		tkn, err := oauth2Config.Token(ctx)
		if err != nil {
			t.Fatalf("could not get token: %v", err)
		}

		resp, _, err := hydraClient.IntrospectOAuth2Token(tkn.AccessToken, "")
		if err != nil {
			t.Fatalf("could not introspect token: %v", err)
		}
		if resp.ClientId != hydraClient.Configuration.ClientID {
			t.Errorf("expected client id %s, got %s instead", hydraClient.Configuration.ClientID, resp.ClientId)
		}
	})

	t.Run("Warden", func(t *testing.T) {
		// List Groups, ensure nothing exists for our test
		groups, _, err := hydraClient.ListGroups("test", 0, 0)
		if err != nil {
			t.Fatalf("could not list groups: %v", err)
		}

		if len(groups) != 0 {
			t.Fatalf("expected 0 groups, got %d instead", len(groups))
		}

		// Create a Group
		group, _, err := hydraClient.CreateGroup(swagger.Group{Id: "testGroup"})
		if err != nil {
			t.Fatalf("could not create group: %v", err)
		}

		// Add a member to a group
		group.Members = append(group.Members, "test", "test2")
		_, err = hydraClient.AddMembersToGroup(group.Id, swagger.GroupMembers{Members: group.Members})
		if err != nil {
			t.Fatalf("could not add members to a group: %v", err)
		}
	})
}
