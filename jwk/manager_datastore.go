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

func (d *DatastoreManager) AddKey(ctx context.Context, set string, key *jose.JSONWebKey) error {
	return nil
}

func (d *DatastoreManager) AddKeySet(ctx context.Context, set string, keys *jose.JSONWebKeySet) error {
	return nil
}

func (d *DatastoreManager) GetKey(ctx context.Context, set, kid string) (*jose.JSONWebKeySet, error) {
	return nil, nil
}

func (d *DatastoreManager) GetKeySet(ctx context.Context, set string) (*jose.JSONWebKeySet, error) {
	return nil, nil
}

func (d *DatastoreManager) DeleteKey(ctx context.Context, set, kid string) error {
	return nil
}

func (d *DatastoreManager) DeleteKeySet(ctx context.Context, set string) error {
	return nil
}
