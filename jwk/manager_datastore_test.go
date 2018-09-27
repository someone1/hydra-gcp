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

package jwk

import (
	"context"
	"fmt"
	"log"
	"os"
	"testing"

	"cloud.google.com/go/datastore"
	. "github.com/ory/hydra/jwk"
	"github.com/stretchr/testify/require"
)

var managers = map[string]Manager{}
var encryptionKey, _ = RandomBytes(32)
var testGenerator = &RS256Generator{}

func connectToDatastore() {
	ctx := context.Background()
	client, err := datastore.NewClient(ctx, "jwk-test")
	if err != nil {
		log.Fatalf("could not connect to database: %v", err)
	}

	s := NewDatastoreManager(client, "jwk-test", &AEAD{Key: encryptionKey})

	managers["datastore"] = s
}

func TestMain(m *testing.M) {
	if !testing.Short() {
		connectToDatastore()
	}

	os.Exit(m.Run())
}

func TestManagerKey(t *testing.T) {
	ks, err := testGenerator.Generate("TestManagerKey", "sig")
	require.NoError(t, err)

	for name, m := range managers {
		t.Run(fmt.Sprintf("case=%s", name), TestHelperManagerKey(m, ks, "TestManagerKey"))
	}
}

func TestManagerKeySet(t *testing.T) {
	ks, err := testGenerator.Generate("TestManagerKeySet", "sig")
	require.NoError(t, err)
	ks.Key("private")

	for name, m := range managers {
		t.Run(fmt.Sprintf("case=%s", name), TestHelperManagerKeySet(m, ks, "TestManagerKeySet"))
	}
}

// func TestManagerRotate(t *testing.T) {
// 	ks, err := testGenerator.Generate("TestManagerRotate", "sig")
// 	require.NoError(t, err)

// 	newKey, _ := RandomBytes(32)
// 	newCipher := &AEAD{Key: newKey}

// 	for name, m := range managers {
// 		t.Run(fmt.Sprintf("manager=%s", name), func(t *testing.T) {
// 			require.NoError(t, m.AddKeySet(context.TODO(), "TestManagerRotate", ks))

// 			require.NoError(t, m.RotateKeys(newCipher))

// 			_, err = m.GetKeySet(context.TODO(), "TestManagerRotate")
// 			require.Error(t, err)

// 			m.Cipher = newCipher
// 			got, err := m.GetKeySet(context.TODO(), "TestManagerRotate")
// 			require.NoError(t, err)

// 			for _, key := range ks.Keys {
// 				require.EqualValues(t, ks.Key(key.KeyID), got.Key(key.KeyID))
// 			}
// 		})
// 	}
// }
