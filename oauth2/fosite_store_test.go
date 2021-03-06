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
	"os"
	"testing"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/ory/fosite"
	"github.com/ory/hydra/client"
	. "github.com/ory/hydra/oauth2"
	"github.com/ory/hydra/pkg"
	"github.com/sirupsen/logrus"
)

var fositeStores = map[string]pkg.FositeStorer{}
var clientManager = &client.MemoryManager{
	Clients: []client.Client{{ClientID: "foobar"}},
	Hasher:  &fosite.BCrypt{},
}

func connectToDatastore() {
	ctx := context.Background()

	client, err := datastore.NewClient(ctx, "fosite-store-test")
	if err != nil {
		log.Fatalf("could not connect to database: %v", err)
	}

	s := NewFositeDatastoreStore(clientManager, client, "fosite-store-test", logrus.New(), time.Hour)

	fositeStores["datastore"] = s
}

func TestMain(m *testing.M) {
	if !testing.Short() {
		connectToDatastore()
	}

	os.Exit(m.Run())
}

func TestUniqueConstraints(t *testing.T) {
	t.Parallel()
	for storageType, store := range fositeStores {
		if storageType == "memory" {
			// memory store does not deal with unique constraints
			continue
		}
		t.Run(fmt.Sprintf("case=%s", storageType), TestHelperUniqueConstraints(store, storageType))
	}
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
