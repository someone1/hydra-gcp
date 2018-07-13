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

package oauth2

import (
	"context"
	"testing"

	"github.com/ory/hydra/pkg"
)

type mockHydraOauth2Data struct {
	Active  bool   `datastore:"act"`
	Version int    `datastore:"v"`
	Client  string `datastore:"cid"`
}

func TestFositeInterfaceType(t *testing.T) {
	var m interface{} = &FositeDatastoreStore{}
	if _, ok := m.(pkg.FositeStorer); !ok {
		t.Fatalf("FositeDatastoreStore does not satisfy pkg.FositeStorer interface")
	}
}

func TestHydraOauth2DataLoad(t *testing.T) {
	t.Parallel()
	if m, ok := fositeStores["datastore"].(*FositeDatastoreStore); ok {
		key := m.createAccessKey("oauth2-upgrade-test")
		key.Parent.Namespace = "upgrade-test"
		key.Namespace = "upgrade-test"
		mock := mockHydraOauth2Data{Version: 1, Active: false, Client: "foobar"}
		if _, err := m.client.Put(context.Background(), key, &mock); err != nil {
			t.Errorf("could not store dummy data - %v", err)
			return
		}

		if _, err := m.findSessionBySignature(context.Background(), key, nil); err != nil {
			t.Errorf("error getting data - %v", err)
			return
		}

		var d hydraOauth2Data
		if err := m.client.Get(context.Background(), key, &d); err != nil {
			t.Errorf("error getting data - %v", err)
			return
		}

		if d.Active != true || d.Version != oauth2Version {
			t.Errorf("oauth2 data was not upgraded succesfully")
		}

	} else {
		t.Error("could not get datastore connection")
	}
}
