/*
 * Copyright © 2018 Prateek Malhotra <someone1@gmail.com>
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
 * Based on https://github.com/ory/hydra/blob/master/jwk/manager_sql.go
 */
package jwk

import (
	"context"
	"encoding/json"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/ory/hydra/jwk"
	"github.com/ory/hydra/pkg"
	"github.com/pkg/errors"
	"gopkg.in/square/go-jose.v2"
)

var (
	// TypeCheck
	_ jwk.Manager = (*DatastoreManager)(nil)
)

const (
	hydraJWKKind = "HydraJWK"
	jwkVersion   = 1
)

type jwkData struct {
	Key              *datastore.Key `datastore:"-"`
	Set              string         `datastore:"-"`
	KID              string         `datastore:"-"`
	Version          int            `datastore:"version"`
	CreatedAt        time.Time      `datastore:"created_at"`
	KeyData          string         `datastore:"keydata,noindex"`
	DatastoreVersion int            `datastore:"v"`

	update bool `datastore:"-"`
}

// LoadKey is implemented for the KeyLoader interface
func (j *jwkData) LoadKey(k *datastore.Key) error {
	j.Key = k
	j.Set = k.Parent.Name
	j.KID = k.Name

	return nil
}

// Load is implemented for the PropertyLoadSaver interface, and performs schema migration if necessary
func (j *jwkData) Load(ps []datastore.Property) error {
	err := datastore.LoadStruct(j, ps)
	if _, ok := err.(*datastore.ErrFieldMismatch); err != nil && !ok {
		return errors.WithStack(err)
	}

	switch j.DatastoreVersion {
	case jwkVersion:
		// Up to date, nothing to do
		break
	// case 1:
	// 	// Update to version 2 here
	// 	h.Active = true
	// 	fallthrough
	// case 2:
	// 	//update to version 3 here
	// 	fallthrough
	case -1:
		// This is here to complete saving the entity should we need to udpate it
		if j.DatastoreVersion == -1 {
			return errors.Errorf("unexpectedly got to version update trigger with incorrect version -1")
		}
		j.DatastoreVersion = jwkVersion
		j.update = true
	default:
		return errors.Errorf("got unexpected version %d when loading entity", j.DatastoreVersion)
	}
	return nil
}

// Save is implemented for the PropertyLoadSaver interface, and adds the requested_at value if needed
func (j *jwkData) Save() ([]datastore.Property, error) {
	j.DatastoreVersion = jwkVersion
	if j.CreatedAt.IsZero() {
		j.CreatedAt = time.Now()
	}

	if j.Set == "" || j.KID == "" || j.KeyData == "" {
		return nil, errors.New("Missing sid, kid, or keydata")
	}

	return datastore.SaveStruct(j)
}

func NewDatastoreManager(client *datastore.Client, namespace string, cipher *jwk.AEAD) *DatastoreManager {
	return &DatastoreManager{
		Cipher:    cipher,
		client:    client,
		namespace: namespace,
	}
}

type DatastoreManager struct {
	client    *datastore.Client
	namespace string
	Cipher    *jwk.AEAD
}

func (d *DatastoreManager) generateJWKParentKey(sid string) *datastore.Key {
	key := datastore.NameKey(hydraJWKKind, sid, nil)
	key.Namespace = d.namespace
	return key
}

func (d *DatastoreManager) generateJWKKey(sid, kid string) *datastore.Key {
	parent := d.generateJWKParentKey(sid)
	key := datastore.NameKey(hydraJWKKind, kid, parent)
	key.Namespace = d.namespace
	return key
}

func (d *DatastoreManager) generateKeyInsertMutation(set string, key *jose.JSONWebKey, cipher *jwk.AEAD) (*datastore.Mutation, error) {
	out, err := json.Marshal(key)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	encrypted, err := cipher.Encrypt(out)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	datastoreKey := d.generateJWKKey(set, key.KeyID)

	entity := &jwkData{
		Set:     set,
		KID:     key.KeyID,
		Version: 0,
		KeyData: encrypted,
	}

	return datastore.NewInsert(datastoreKey, entity), nil
}

func (d *DatastoreManager) AddKey(ctx context.Context, set string, key *jose.JSONWebKey) error {
	mutation, err := d.generateKeyInsertMutation(set, key, d.Cipher)
	if err != nil {
		return err
	}

	_, err = d.client.Mutate(ctx, mutation)

	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func (d *DatastoreManager) AddKeySet(ctx context.Context, set string, keys *jose.JSONWebKeySet) error {
	_, err := d.client.RunInTransaction(ctx, func(tx *datastore.Transaction) error {
		return d.addKeySet(ctx, tx, d.Cipher, set, keys)
	})

	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func (d *DatastoreManager) addKeySet(ctx context.Context, tx *datastore.Transaction, cipher *jwk.AEAD, set string, keys *jose.JSONWebKeySet) error {
	var mutations []*datastore.Mutation

	for _, key := range keys.Keys {
		mutation, err := d.generateKeyInsertMutation(set, &key, cipher)
		if err != nil {
			return err
		}
		mutations = append(mutations, mutation)
	}

	_, err := tx.Mutate(mutations...)
	return err
}

func (d *DatastoreManager) GetKey(ctx context.Context, set, kid string) (*jose.JSONWebKeySet, error) {
	var entity jwkData
	datastoreKey := d.generateJWKKey(set, kid)

	err := d.client.Get(ctx, datastoreKey, &entity)
	if err == datastore.ErrNoSuchEntity {
		return nil, errors.WithStack(pkg.ErrNotFound)
	}

	key, err := d.Cipher.Decrypt(entity.KeyData)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	var c jose.JSONWebKey
	if err := json.Unmarshal(key, &c); err != nil {
		return nil, errors.WithStack(err)
	}

	return &jose.JSONWebKeySet{
		Keys: []jose.JSONWebKey{c},
	}, nil
}

func (d *DatastoreManager) GetKeySet(ctx context.Context, set string) (*jose.JSONWebKeySet, error) {
	var ds []jwkData
	parentKey := d.generateJWKParentKey(set)

	qry := datastore.NewQuery(hydraJWKKind).Namespace(d.namespace).Ancestor(parentKey).Order("-created_at")
	if _, err := d.client.GetAll(ctx, qry, &ds); err != nil {
		return nil, errors.WithStack(err)
	}

	if len(ds) == 0 {
		return nil, errors.Wrap(pkg.ErrNotFound, "")
	}

	keys := &jose.JSONWebKeySet{Keys: []jose.JSONWebKey{}}
	for _, j := range ds {
		key, err := d.Cipher.Decrypt(j.KeyData)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		var c jose.JSONWebKey
		if err := json.Unmarshal(key, &c); err != nil {
			return nil, errors.WithStack(err)
		}
		keys.Keys = append(keys.Keys, c)
	}

	if len(keys.Keys) == 0 {
		return nil, errors.WithStack(pkg.ErrNotFound)
	}

	return keys, nil
}

func (d *DatastoreManager) DeleteKey(ctx context.Context, set, kid string) error {
	datastoreKey := d.generateJWKKey(set, kid)
	if err := d.client.Delete(ctx, datastoreKey); err == datastore.ErrNoSuchEntity {
		return errors.WithStack(pkg.ErrNotFound)
	} else if err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func (d *DatastoreManager) DeleteKeySet(ctx context.Context, set string) error {
	_, err := d.client.RunInTransaction(ctx, func(tx *datastore.Transaction) error {
		return d.deleteKeySet(ctx, tx, set)
	})

	if err != nil {
		return errors.WithStack(err)
	}
	return nil
}

func (d *DatastoreManager) deleteKeySet(ctx context.Context, tx *datastore.Transaction, set string) error {
	parentKey := d.generateJWKParentKey(set)
	qry := datastore.NewQuery(hydraJWKKind).Namespace(d.namespace).Ancestor(parentKey).KeysOnly().Transaction(tx)
	keys, err := d.client.GetAll(ctx, qry, nil)
	if err != nil {
		return errors.WithStack(err)
	}

	return tx.DeleteMulti(keys)
}

// This is iffy - this is a special hydra case from the cli only - will it even work?
// func (d *DatastoreManager) RotateKeys(new *AEAD) error {
// 	sids := make([]string, 0)
// 	if err := m.DB.Select(&sids, "SELECT sid FROM hydra_jwk GROUP BY sid"); err != nil {
// 		return sqlcon.HandleError(err)
// 	}

// 	sets := make([]jose.JSONWebKeySet, 0)
// 	for _, sid := range sids {
// 		set, err := m.GetKeySet(context.TODO(), sid)
// 		if err != nil {
// 			return errors.WithStack(err)
// 		}
// 		sets = append(sets, *set)
// 	}

// 	tx, err := m.DB.Beginx()
// 	if err != nil {
// 		return errors.WithStack(err)
// 	}

// 	for k, set := range sets {
// 		if err := m.deleteKeySet(context.TODO(), tx, sids[k]); err != nil {
// 			if re := tx.Rollback(); re != nil {
// 				return errors.Wrap(err, re.Error())
// 			}
// 			return sqlcon.HandleError(err)
// 		}

// 		if err := m.addKeySet(context.TODO(), tx, new, sids[k], &set); err != nil {
// 			if re := tx.Rollback(); re != nil {
// 				return errors.Wrap(err, re.Error())
// 			}
// 			return sqlcon.HandleError(err)
// 		}
// 	}

// 	if err := tx.Commit(); err != nil {
// 		if re := tx.Rollback(); re != nil {
// 			return errors.Wrap(err, re.Error())
// 		}
// 		return sqlcon.HandleError(err)
// 	}
// 	return nil
// }
