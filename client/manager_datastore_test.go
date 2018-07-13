// Copyright © 2018 Prateek Malhotra <someone1@gmail.com>
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

package client

import (
	"context"
	"testing"

	"github.com/ory/hydra/client"
)

type mockClientData struct {
	SecretExpiresAt int `datastore:"csea"`
	Version         int `datastore:"v"`
}

func TestInterfaceType(t *testing.T) {
	var m interface{} = &DatastoreManager{}
	if _, ok := m.(client.Manager); !ok {
		t.Fatalf("DatastoreManager does not satisfy client.Manager interface")
	}
}

func TestClientDataLoad(t *testing.T) {
	t.Parallel()
	if m, ok := clientManagers["datastore"].(*DatastoreManager); ok {
		key := m.createClientKey("client-upgrade-test")
		mock := mockClientData{SecretExpiresAt: -1, Version: 1}
		if _, err := m.client.Put(context.Background(), key, &mock); err != nil {
			t.Errorf("could not store dummy data - %v", err)
			return
		}
		defer m.client.Delete(context.Background(), key)

		if client, err := m.GetConcreteClient(key.Name); err != nil {
			t.Errorf("error getting data - %v", err)
			return
		} else if client.SecretExpiresAt != 0 {
			t.Errorf("client data was not upgraded succesfully")
			return
		}

		var d clientData
		if err := m.client.Get(context.Background(), key, &d); err != nil {
			t.Errorf("cloud not get client data - %v", err)
			return
		} else if d.Version != hydraClientVersion {
			t.Errorf("data not upgraded correctly")
		}
	} else {
		t.Error("could not get datastore connection")
	}
}
