/*
 * Copyright © 2015-2018 Aeneas Rekkas <aeneas+oss@aeneas.io>
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * @author		Aeneas Rekkas <aeneas+oss@aeneas.io>
 * @copyright 	2015-2018 Aeneas Rekkas <aeneas+oss@aeneas.io>
 * @license 	Apache-2.0
 *
 * Modified for datastore only tests.
 */

package oauth2

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/ory/fosite"
	"github.com/ory/hydra/client"
	. "github.com/ory/hydra/oauth2"
	"github.com/ory/hydra/pkg"
	"github.com/sirupsen/logrus"

	dconfig "github.com/someone1/hydra-gcp/config"
)

var fositeStores = map[string]pkg.FositeStorer{}
var clientManager = &client.MemoryManager{
	Clients: []client.Client{{ID: "foobar"}},
	Hasher:  &fosite.BCrypt{},
}

func connectToDatastore() {
	u, err := url.Parse("datastore://fosite-store-test?namespace=fosite-store-test")
	if err != nil {
		log.Fatalf("Could not parse DATABASE_URL: %s", err)
	}

	con, err := dconfig.NewDatastoreConnection(context.Background(), u, nil)
	if err != nil {
		log.Fatalf("could not connect to database: %v", err)
	}
	s := NewFositeDatastoreStore(clientManager, con.Client(), con.Namespace(), logrus.New(), time.Hour)

	fositeStores["datastore"] = s
}

func TestMain(m *testing.M) {
	if !testing.Short() {
		connectToDatastore()
	}

	os.Exit(m.Run())
}

func TestCreateGetDeleteAuthorizeCodes(t *testing.T) {
	t.Parallel()
	for k, m := range fositeStores {
		t.Run(fmt.Sprintf("case=%s", k), TestHelperCreateGetDeleteAuthorizeCodes(m))
	}
}

func TestCreateGetDeleteAccessTokenSession(t *testing.T) {
	t.Parallel()
	for k, m := range fositeStores {
		t.Run(fmt.Sprintf("case=%s", k), TestHelperCreateGetDeleteAccessTokenSession(m))
	}
}

func TestCreateGetDeleteOpenIDConnectSession(t *testing.T) {
	t.Parallel()
	for k, m := range fositeStores {
		t.Run(fmt.Sprintf("case=%s", k), TestHelperCreateGetDeleteOpenIDConnectSession(m))
	}
}

func TestCreateGetDeleteRefreshTokenSession(t *testing.T) {
	t.Parallel()
	for k, m := range fositeStores {
		t.Run(fmt.Sprintf("case=%s", k), TestHelperCreateGetDeleteRefreshTokenSession(m))
	}
}

func TestRevokeRefreshToken(t *testing.T) {
	t.Parallel()
	for k, m := range fositeStores {
		t.Run(fmt.Sprintf("case=%s", k), TestHelperRevokeRefreshToken(m))
	}
}

func TestPKCEReuqest(t *testing.T) {
	t.Parallel()
	for k, m := range fositeStores {
		t.Run(fmt.Sprintf("case=%s", k), TestHelperCreateGetDeletePKCERequestSession(m))
	}
}

func TestFlushAccessTokens(t *testing.T) {
	t.Parallel()
	for k, m := range fositeStores {
		t.Run(fmt.Sprintf("case=%s", k), TestHelperFlushTokens(m, time.Hour))
	}
}
