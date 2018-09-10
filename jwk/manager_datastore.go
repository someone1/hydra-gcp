package jwk

import (
	"context"

	"cloud.google.com/go/datastore"
	"github.com/ory/hydra/jwk"
	"gopkg.in/square/go-jose.v2"
)

var (
	// TypeCheck
	_ jwk.Manager = (*DatastoreManager)(nil)
)

type DatastoreManager struct {
	client    *datastore.Client
	context   context.Context
	namespace string
}

func (d *DatastoreManager) AddKey(set string, key *jose.JSONWebKey) error {
	return nil
}

func (d *DatastoreManager) AddKeySet(set string, keys *jose.JSONWebKeySet) error {
	return nil
}

func (d *DatastoreManager) GetKey(set, kid string) (*jose.JSONWebKeySet, error) {
	return nil, nil
}

func (d *DatastoreManager) GetKeySet(set string) (*jose.JSONWebKeySet, error) {
	return nil, nil
}

func (d *DatastoreManager) DeleteKey(set, kid string) error {
	return nil
}

func (d *DatastoreManager) DeleteKeySet(set string) error {
	return nil
}
