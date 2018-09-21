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
 * Modified to test datastore only.
 */

package client

import (
	"context"
	"fmt"
	"log"
	"os"
	"testing"

	"cloud.google.com/go/datastore"
	"github.com/ory/fosite"
	. "github.com/ory/hydra/client"
)

var clientManagers = map[string]Manager{}

func connectToDatastore() {
	ctx := context.Background()
	client, err := datastore.NewClient(ctx, "client-test")
	if err != nil {
		log.Fatalf("could not connect to database: %v", err)
	}

	s := NewDatastoreManager(client, "client-test", &fosite.BCrypt{WorkFactor: 4})

	clientManagers["datastore"] = s
}

func TestMain(m *testing.M) {
	if !testing.Short() {
		connectToDatastore()
	}

	os.Exit(m.Run())
}

func TestCreateGetDeleteClient(t *testing.T) {
	for k, m := range clientManagers {
		t.Run(fmt.Sprintf("case=%s", k), TestHelperCreateGetDeleteClient(k, m))
	}
}

func TestClientAutoGenerateKey(t *testing.T) {
	for k, m := range clientManagers {
		t.Run(fmt.Sprintf("case=%s", k), TestHelperClientAutoGenerateKey(k, m))
	}
}

func TestAuthenticateClient(t *testing.T) {
	for k, m := range clientManagers {
		t.Run(fmt.Sprintf("case=%s", k), TestHelperClientAuthenticate(k, m))
	}
}
