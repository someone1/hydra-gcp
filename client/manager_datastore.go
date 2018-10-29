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
 *
 * Based on https://github.com/ory/hydra/blob/master/client/manager_sql.go
 */

package client

import (
	"context"
	"strings"

	"cloud.google.com/go/datastore"
	"github.com/ory/fosite"
	"github.com/ory/go-convenience/stringsx"
	"github.com/ory/hydra/client"
	"github.com/pkg/errors"
	"gopkg.in/square/go-jose.v2"
	"gopkg.in/square/go-jose.v2/json"

	"github.com/someone1/hydra-gcp/dscon"
)

var (
	// TypeCheck
	_ client.Manager = (*DatastoreManager)(nil)
)

const (
	hydraClientKind    = "HydraClient"
	hydraClientVersion = 3
)

type clientData struct {
	Key                           *datastore.Key `datastore:"-"`
	ID                            string         `datastore:"-"`
	Name                          string         `datastore:"cn"`
	Secret                        string         `datastore:"cs"`
	RedirectURIs                  string         `datastore:"ruris"`
	GrantTypes                    string         `datastore:"gt"`
	ResponseTypes                 string         `datastore:"rt"`
	Scope                         string         `datastore:"scp"`
	Owner                         string         `datastore:"owner"`
	PolicyURI                     string         `datastore:"puri"`
	TermsOfServiceURI             string         `datastore:"turi"`
	ClientURI                     string         `datastore:"curi"`
	LogoURI                       string         `datastore:"luri"`
	Contacts                      string         `datastore:"conts"`
	SecretExpiresAt               int            `datastore:"csea"`
	SectorIdentifierURI           string         `datastore:"siuri"`
	JSONWebKeysURI                string         `datastore:"jwks_uri"`
	JSONWebKeys                   string         `datastore:"jwks"`
	TokenEndpointAuthMethod       string         `datastore:"team"`
	RequestURIs                   string         `datastore:"ruri"`
	SubjectType                   string         `datastore:"subt"`
	RequestObjectSigningAlgorithm string         `datastore:"rosa"`
	UserinfoSignedResponseAlg     string         `datastore:"usra"`
	AllowedCORSOrigins            string         `datastore:"acorso"`

	Version int `datastore:"v"`
	update  bool
}

// LoadKey is implemented for the KeyLoader interface
func (c *clientData) LoadKey(k *datastore.Key) error {
	c.Key = k
	c.ID = k.Name
	return nil
}

// Load is implemented for the PropertyLoadSaver interface, and performs schema migration if necessary
func (c *clientData) Load(ps []datastore.Property) error {
	var public = false
	for idx := range ps {
		if ps[idx].Name == "pub" {
			if isPublic, ok := ps[idx].Value.(bool); ok && isPublic {
				public = true
			}
			ps = append(ps[:idx], ps[idx+1:]...)
			break
		}
	}

	err := datastore.LoadStruct(c, ps)
	if _, ok := err.(*datastore.ErrFieldMismatch); err != nil && !ok {
		return errors.WithStack(err)
	}

	switch c.Version {
	case hydraClientVersion:
		// Up to date, nothing to do
		break
	case 1:
		// Update to version 2 here
		c.SecretExpiresAt = 0
		fallthrough
	case 2:
		if public {
			c.TokenEndpointAuthMethod = "none"
		}
		fallthrough
	case -1:
		// This is here to complete saving the entity should we need to udpate it
		if c.Version == -1 {
			return errors.Errorf("unexpectedly got to version update trigger with incorrect version -1")
		}
		c.Version = hydraClientVersion
		c.update = true
	default:
		return errors.Errorf("got unexpected version %d when loading entity", c.Version)
	}
	return nil
}

// Save is implemented for the PropertyLoadSaver interface
func (c *clientData) Save() ([]datastore.Property, error) {
	c.Version = hydraClientVersion
	return datastore.SaveStruct(c)
}

// DatastoreManager is a Google Datastore implementation for client.Manager.
type DatastoreManager struct {
	hasher    fosite.Hasher
	client    *datastore.Client
	context   context.Context
	namespace string
}

// NewDatastoreManager initializes a new DatastoreManager with the given client
func NewDatastoreManager(client *datastore.Client, namespace string, h fosite.Hasher) *DatastoreManager {
	return &DatastoreManager{
		hasher:    h,
		client:    client,
		namespace: namespace,
	}
}

func (d *DatastoreManager) createClientKey(id string) *datastore.Key {
	key := datastore.NameKey(hydraClientKind, id, nil)
	key.Namespace = d.namespace
	return key
}

func (d *DatastoreManager) newClientQuery() *datastore.Query {
	return datastore.NewQuery(hydraClientKind).Namespace(d.namespace)
}

func clientDataFromClient(d *client.Client) (*clientData, error) {
	jwks := ""

	if d.JSONWebKeys != nil {
		out, err := json.Marshal(d.JSONWebKeys)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		jwks = string(out)
	}
	return &clientData{
		ID:                            d.GetID(),
		Name:                          d.Name,
		Secret:                        d.Secret,
		RedirectURIs:                  strings.Join(d.RedirectURIs, "|"),
		GrantTypes:                    strings.Join(d.GrantTypes, "|"),
		ResponseTypes:                 strings.Join(d.ResponseTypes, "|"),
		Scope:                         d.Scope,
		Owner:                         d.Owner,
		PolicyURI:                     d.PolicyURI,
		TermsOfServiceURI:             d.TermsOfServiceURI,
		ClientURI:                     d.ClientURI,
		LogoURI:                       d.LogoURI,
		Contacts:                      strings.Join(d.Contacts, "|"),
		SecretExpiresAt:               d.SecretExpiresAt,
		SectorIdentifierURI:           d.SectorIdentifierURI,
		JSONWebKeysURI:                d.JSONWebKeysURI,
		JSONWebKeys:                   jwks,
		TokenEndpointAuthMethod:       d.TokenEndpointAuthMethod,
		RequestURIs:                   strings.Join(d.RequestURIs, "|"),
		RequestObjectSigningAlgorithm: d.RequestObjectSigningAlgorithm,
		UserinfoSignedResponseAlg:     d.UserinfoSignedResponseAlg,
		SubjectType:                   d.SubjectType,
		AllowedCORSOrigins:            strings.Join(d.AllowedCORSOrigins, "|"),
	}, nil
}

func (c *clientData) toClient() (*client.Client, error) {
	cli := &client.Client{
		ClientID:                      c.ID,
		Name:                          c.Name,
		Secret:                        c.Secret,
		RedirectURIs:                  stringsx.Splitx(c.RedirectURIs, "|"),
		GrantTypes:                    stringsx.Splitx(c.GrantTypes, "|"),
		ResponseTypes:                 stringsx.Splitx(c.ResponseTypes, "|"),
		Scope:                         c.Scope,
		Owner:                         c.Owner,
		PolicyURI:                     c.PolicyURI,
		TermsOfServiceURI:             c.TermsOfServiceURI,
		ClientURI:                     c.ClientURI,
		LogoURI:                       c.LogoURI,
		Contacts:                      stringsx.Splitx(c.Contacts, "|"),
		SecretExpiresAt:               c.SecretExpiresAt,
		SectorIdentifierURI:           c.SectorIdentifierURI,
		JSONWebKeysURI:                c.JSONWebKeysURI,
		TokenEndpointAuthMethod:       c.TokenEndpointAuthMethod,
		RequestURIs:                   stringsx.Splitx(c.RequestURIs, "|"),
		RequestObjectSigningAlgorithm: c.RequestObjectSigningAlgorithm,
		UserinfoSignedResponseAlg:     c.UserinfoSignedResponseAlg,
		SubjectType:                   c.SubjectType,
		AllowedCORSOrigins:            stringsx.Splitx(c.AllowedCORSOrigins, "|"),
	}

	if c.JSONWebKeys != "" {
		cli.JSONWebKeys = new(jose.JSONWebKeySet)
		if err := json.Unmarshal([]byte(c.JSONWebKeys), &cli.JSONWebKeys); err != nil {
			return nil, errors.WithStack(err)
		}
	}

	return cli, nil
}

func (d *DatastoreManager) GetConcreteClient(ctx context.Context, id string) (*client.Client, error) {
	var cd clientData
	key := d.createClientKey(id)

	if err := d.client.Get(ctx, key, &cd); err != nil {
		return nil, dscon.HandleError(err)
	}

	if cd.update {
		mutation := datastore.NewUpdate(key, &cd)
		if _, err := d.client.Mutate(ctx, mutation); err != nil {
			return nil, errors.WithStack(err)
		}
		cd.update = false
	}

	return cd.toClient()
}

func (d *DatastoreManager) GetClient(ctx context.Context, id string) (fosite.Client, error) {
	return d.GetConcreteClient(ctx, id)
}

func (d *DatastoreManager) UpdateClient(ctx context.Context, c *client.Client) error {
	o, err := d.GetConcreteClient(ctx, c.GetID())
	if err != nil {
		return errors.WithStack(err)
	}

	if c.Secret == "" {
		c.Secret = string(o.GetHashedSecret())
	} else {
		h, err := d.hasher.Hash(ctx, []byte(c.Secret))
		if err != nil {
			return errors.WithStack(err)
		}
		c.Secret = string(h)
	}

	s, err := clientDataFromClient(c)
	if err != nil {
		return errors.WithStack(err)
	}

	key := d.createClientKey(s.ID)
	mutation := datastore.NewUpdate(key, s)

	if _, err := d.client.Mutate(ctx, mutation); err != nil {
		return dscon.HandleError(err)
	}
	return nil
}

func (d *DatastoreManager) Authenticate(ctx context.Context, id string, secret []byte) (*client.Client, error) {
	c, err := d.GetConcreteClient(ctx, id)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if err := d.hasher.Compare(ctx, c.GetHashedSecret(), secret); err != nil {
		return nil, errors.WithStack(err)
	}

	return c, nil
}

func (d *DatastoreManager) CreateClient(ctx context.Context, c *client.Client) error {
	h, err := d.hasher.Hash(ctx, []byte(c.Secret))
	if err != nil {
		return errors.WithStack(err)
	}
	c.Secret = string(h)

	data, err := clientDataFromClient(c)
	if err != nil {
		return errors.WithStack(err)
	}
	key := d.createClientKey(data.ID)
	mutation := datastore.NewInsert(key, data)

	if _, err := d.client.Mutate(ctx, mutation); err != nil {
		return dscon.HandleError(err)
	}
	return nil
}

func (d *DatastoreManager) DeleteClient(ctx context.Context, id string) error {
	key := d.createClientKey(id)
	if err := d.client.Delete(ctx, key); err != nil {
		return dscon.HandleError(err)
	}
	return nil
}

// This follows the implementation from the master branch
func (d *DatastoreManager) GetClients(ctx context.Context, limit, offset int) (map[string]client.Client, error) {
	datas := make([]clientData, 0)
	clients := make(map[string]client.Client)

	query := d.newClientQuery().Order("__key__").Limit(limit).Offset(offset)

	_, err := d.client.GetAll(ctx, query, &datas)

	if err != nil {
		return nil, dscon.HandleError(err)
	}

	for _, k := range datas {
		c, err := k.toClient()
		if err != nil {
			return nil, errors.WithStack(err)
		}

		clients[k.ID] = *c
	}
	return clients, nil
}
