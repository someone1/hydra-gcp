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
 * @Copyright 	2017-2018 Aeneas Rekkas <aeneas+oss@aeneas.io>
 * @license 	Apache-2.0
 *
 * Modified for testing datastore only
 */

package consent

import (
	"context"
	"log"
	"os"
	"testing"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/ory/fosite"
	"github.com/ory/hydra/client"
	. "github.com/ory/hydra/consent"
	"github.com/ory/hydra/oauth2"
)

var clientManager = client.NewMemoryManager(&fosite.BCrypt{WorkFactor: 8})
var fositeManager = oauth2.NewFositeMemoryStore(clientManager, time.Hour)
var managers = map[string]Manager{}

func connectToDatastore(managers map[string]Manager, c client.Manager) {
	ctx := context.Background()

	client, err := datastore.NewClient(ctx, "consent-test")
	if err != nil {
		log.Fatalf("could not connect to database: %v", err)
	}

	s := NewDatastoreManager(client, "consent-test", c, fositeManager)

	managers["datastore"] = s
}

func TestMain(m *testing.M) {
	if !testing.Short() {
		connectToDatastore(managers, clientManager)
	}

	os.Exit(m.Run())
}

func TestManagers(t *testing.T) {
	for k, m := range managers {
		t.Run("manager="+k, ManagerTests(m, clientManager, fositeManager))
	}
}
