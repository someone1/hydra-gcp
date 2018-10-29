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
 * Based on https://github.com/ory/hydra/blob/master/oauth2/fosite_store_sql.go
 */

package oauth2

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/ory/fosite"
	"github.com/ory/hydra/client"
	"github.com/ory/hydra/pkg"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/someone1/hydra-gcp/dscon"
)

const (
	hydraOauth2OpenIDKind   = "HydraOauth2OIDC"
	hydraOauth2AccessKind   = "HydraOauth2Access"
	hydraOauth2RefreshKind  = "HydraOauth2Refresh"
	hydraOauth2AuthCodeKind = "HydraOauth2Code"
	hydraOauth2PKCEKind     = "HydraOauth2PKCE"
	uniqueTableKind         = "Unique"
	oauth2Version           = 2
)

type uniqueConstraint struct{}

var (
	// TypeCheck
	_ pkg.FositeStorer = (*FositeDatastoreStore)(nil)
)

type hydraOauth2Data struct {
	Key           *datastore.Key `datastore:"-"`
	Signature     string         `datastore:"-"`
	Request       string         `datastore:"rid"`
	RequestedAt   time.Time      `datastore:"rat"`
	Client        string         `datastore:"cid"`
	Scopes        string         `datastore:"scp"`
	GrantedScopes string         `datastore:"gscps"`
	Form          string         `datastore:"fd"`
	Subject       string         `datastore:"sub"`
	Active        bool           `datastore:"act"`
	Session       []byte         `datastore:"sess"`

	Version int `datastore:"v"`
	update  bool
}

// LoadKey is implemented for the KeyLoader interface
func (h *hydraOauth2Data) LoadKey(k *datastore.Key) error {
	h.Key = k
	h.Signature = k.Name

	return nil
}

// Load is implemented for the PropertyLoadSaver interface, and performs schema migration if necessary
func (h *hydraOauth2Data) Load(ps []datastore.Property) error {
	err := datastore.LoadStruct(h, ps)
	if _, ok := err.(*datastore.ErrFieldMismatch); err != nil && !ok {
		return errors.WithStack(err)
	}

	switch h.Version {
	case oauth2Version:
		// Up to date, nothing to do
		break
	case 1:
		// Update to version 2 here
		h.Active = true
		fallthrough
	// case 2:
	// 	//update to version 3 here
	// 	fallthrough
	case -1:
		// This is here to complete saving the entity should we need to udpate it
		if h.Version == -1 {
			return errors.Errorf("unexpectedly got to version update trigger with incorrect version -1")
		}
		h.Version = oauth2Version
		h.update = true
	default:
		return errors.Errorf("got unexpected version %d when loading entity", h.Version)
	}
	return nil
}

// Save is implemented for the PropertyLoadSaver interface, and adds the requested_at value if needed
func (h *hydraOauth2Data) Save() ([]datastore.Property, error) {
	h.Version = oauth2Version
	if h.RequestedAt.IsZero() {
		h.RequestedAt = time.Now()
	}
	return datastore.SaveStruct(h)
}

// FositeDatastoreStore is a Google Datastore implementation for pkg.FositeStorer to store policies persistently.
type FositeDatastoreStore struct {
	client.Manager
	L                   logrus.FieldLogger
	AccessTokenLifespan time.Duration

	client    *datastore.Client
	namespace string
}

// NewFositeDatastoreStore initializes a new FositeDatastoreStore with the given client
func NewFositeDatastoreStore(m client.Manager,
	client *datastore.Client,
	namespace string,
	l logrus.FieldLogger,
	accessTokenLifespan time.Duration,
) *FositeDatastoreStore {
	return &FositeDatastoreStore{
		Manager:             m,
		L:                   l,
		AccessTokenLifespan: accessTokenLifespan,
		client:              client,
		namespace:           namespace,
	}
}

func (f *FositeDatastoreStore) createOIDCKey(sig string) *datastore.Key {
	return f.createKeyForKind(sig, hydraOauth2OpenIDKind)
}
func (f *FositeDatastoreStore) createAccessKey(sig string) *datastore.Key {
	return f.createKeyForKind(sig, hydraOauth2AccessKind)
}
func (f *FositeDatastoreStore) createRefreshKey(sig string) *datastore.Key {
	return f.createKeyForKind(sig, hydraOauth2RefreshKind)
}
func (f *FositeDatastoreStore) createCodeKey(sig string) *datastore.Key {
	return f.createKeyForKind(sig, hydraOauth2AuthCodeKind)
}
func (f *FositeDatastoreStore) createPKCEKey(sig string) *datastore.Key {
	return f.createKeyForKind(sig, hydraOauth2PKCEKind)
}

func (f *FositeDatastoreStore) createKeyForKind(sig, kind string) *datastore.Key {
	key := datastore.NameKey(kind, sig, nil)
	key.Namespace = f.namespace
	return key
}

func (f *FositeDatastoreStore) newQueryForKind(kind string) *datastore.Query {
	return datastore.NewQuery(kind).Namespace(f.namespace)
}

func oauth2DataFromRequest(signature string, r fosite.Requester, logger logrus.FieldLogger) (*hydraOauth2Data, error) {
	subject := ""
	if r.GetSession() == nil {
		logger.Debugf("Got an empty session in oauth2DataFromRequest")
	} else {
		subject = r.GetSession().GetSubject()
	}

	session, err := json.Marshal(r.GetSession())
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return &hydraOauth2Data{
		Request:       r.GetID(),
		Signature:     signature,
		RequestedAt:   r.GetRequestedAt(),
		Client:        r.GetClient().GetID(),
		Scopes:        strings.Join([]string(r.GetRequestedScopes()), "|"),
		GrantedScopes: strings.Join([]string(r.GetGrantedScopes()), "|"),
		Form:          r.GetRequestForm().Encode(),
		Session:       session,
		Subject:       subject,
		Active:        true,
	}, nil
}

func (h *hydraOauth2Data) toRequest(session fosite.Session, cm client.Manager, logger logrus.FieldLogger) (*fosite.Request, error) {
	if session != nil {
		if err := json.Unmarshal(h.Session, session); err != nil {
			return nil, errors.WithStack(err)
		}
	} else {
		logger.Debugf("Got an empty session in toRequest")
	}

	c, err := cm.GetClient(context.Background(), h.Client)
	if err != nil {
		return nil, err
	}

	val, err := url.ParseQuery(h.Form)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	r := &fosite.Request{
		ID:            h.Request,
		RequestedAt:   h.RequestedAt,
		Client:        c,
		Scopes:        fosite.Arguments(strings.Split(h.Scopes, "|")),
		GrantedScopes: fosite.Arguments(strings.Split(h.GrantedScopes, "|")),
		Form:          val,
		Session:       session,
	}

	return r, nil
}

func (f *FositeDatastoreStore) createSession(ctx context.Context, key *datastore.Key, requester fosite.Requester, unique bool) error {
	data, err := oauth2DataFromRequest(key.Name, requester, f.L)
	if err != nil {
		return err
	}

	if unique {
		// Unique Constraint for RequestID
		uniqueKey := datastore.NameKey(uniqueTableKind, key.Kind+data.Request, nil)
		uniqueMutation := datastore.NewInsert(uniqueKey, &uniqueConstraint{})
		if _, err := f.client.Mutate(ctx, uniqueMutation); err != nil {
			return dscon.HandleError(err)
		}
	}

	mutation := datastore.NewInsert(key, data)
	if _, err := f.client.Mutate(ctx, mutation); err != nil {
		return dscon.HandleError(err)
	}

	return nil
}

func (f *FositeDatastoreStore) findSessionBySignature(ctx context.Context, key *datastore.Key, session fosite.Session) (fosite.Requester, error) {
	var d hydraOauth2Data

	if err := f.client.Get(ctx, key, &d); err != nil {
		return nil, dscon.HandleError(err)
	} else if !d.Active && key.Kind == hydraOauth2AuthCodeKind {
		if r, err := d.toRequest(session, f.Manager, f.L); err != nil {
			return nil, err
		} else {
			return r, errors.WithStack(fosite.ErrInvalidatedAuthorizeCode)
		}
	} else if !d.Active {
		return nil, errors.WithStack(fosite.ErrInactiveToken)
	}

	if d.update {
		mutation := datastore.NewUpdate(key, &d)
		if _, err := f.client.Mutate(ctx, mutation); err != nil {
			return nil, errors.WithStack(err)
		}
		d.update = false
	}

	return d.toRequest(session, f.Manager, f.L)
}

func (f *FositeDatastoreStore) deleteSession(ctx context.Context, key *datastore.Key) error {
	if err := f.client.Delete(ctx, key); err != nil {
		return dscon.HandleError(err)
	}

	return nil
}

func (f *FositeDatastoreStore) revokeSession(ctx context.Context, id, kind string) error {
	query := f.newQueryForKind(kind).Filter("rid=", id).KeysOnly()
	keys, err := f.client.GetAll(ctx, query, nil)
	if err != nil {
		return dscon.HandleError(err)
	}
	if len(keys) == 0 {
		return errors.Wrap(fosite.ErrNotFound, "")
	}
	if err = f.client.DeleteMulti(ctx, keys); err != nil {
		return dscon.HandleError(err)
	}
	return nil
}

func (f *FositeDatastoreStore) CreateOpenIDConnectSession(ctx context.Context, signature string, requester fosite.Requester) error {
	return f.createSession(ctx, f.createOIDCKey(signature), requester, false)
}

func (f *FositeDatastoreStore) GetOpenIDConnectSession(ctx context.Context, signature string, requester fosite.Requester) (fosite.Requester, error) {
	return f.findSessionBySignature(ctx, f.createOIDCKey(signature), requester.GetSession())
}

func (f *FositeDatastoreStore) DeleteOpenIDConnectSession(ctx context.Context, signature string) error {
	return f.deleteSession(ctx, f.createOIDCKey(signature))
}

func (f *FositeDatastoreStore) CreateAuthorizeCodeSession(ctx context.Context, signature string, requester fosite.Requester) error {
	return f.createSession(ctx, f.createCodeKey(signature), requester, false)
}

func (f *FositeDatastoreStore) GetAuthorizeCodeSession(ctx context.Context, signature string, session fosite.Session) (fosite.Requester, error) {
	return f.findSessionBySignature(ctx, f.createCodeKey(signature), session)
}

func (f *FositeDatastoreStore) InvalidateAuthorizeCodeSession(ctx context.Context, signature string) error {
	var data hydraOauth2Data
	key := f.createCodeKey(signature)

	err := f.client.Get(ctx, key, &data)
	if err != nil {
		return dscon.HandleError(err)
	}
	data.Active = false
	mutation := datastore.NewUpdate(key, &data)
	if _, err := f.client.Mutate(ctx, mutation); err != nil {
		return dscon.HandleError(err)
	}

	return nil
}

func (f *FositeDatastoreStore) DeleteAuthorizeCodeSession(ctx context.Context, signature string) error {
	return f.deleteSession(ctx, f.createCodeKey(signature))
}

func (f *FositeDatastoreStore) CreateAccessTokenSession(ctx context.Context, signature string, requester fosite.Requester) error {
	return f.createSession(ctx, f.createAccessKey(signature), requester, true)
}

func (f *FositeDatastoreStore) GetAccessTokenSession(ctx context.Context, signature string, session fosite.Session) (fosite.Requester, error) {
	return f.findSessionBySignature(ctx, f.createAccessKey(signature), session)
}

func (f *FositeDatastoreStore) DeleteAccessTokenSession(ctx context.Context, signature string) error {
	return f.deleteSession(ctx, f.createAccessKey(signature))
}

func (f *FositeDatastoreStore) CreateRefreshTokenSession(ctx context.Context, signature string, requester fosite.Requester) error {
	return f.createSession(ctx, f.createRefreshKey(signature), requester, true)
}

func (f *FositeDatastoreStore) GetRefreshTokenSession(ctx context.Context, signature string, session fosite.Session) (fosite.Requester, error) {
	return f.findSessionBySignature(ctx, f.createRefreshKey(signature), session)
}

func (f *FositeDatastoreStore) DeleteRefreshTokenSession(ctx context.Context, signature string) error {
	return f.deleteSession(ctx, f.createRefreshKey(signature))
}

func (f *FositeDatastoreStore) CreatePKCERequestSession(ctx context.Context, signature string, requester fosite.Requester) error {
	return f.createSession(ctx, f.createPKCEKey(signature), requester, false)
}

func (f *FositeDatastoreStore) GetPKCERequestSession(ctx context.Context, signature string, session fosite.Session) (fosite.Requester, error) {
	return f.findSessionBySignature(ctx, f.createPKCEKey(signature), session)
}

func (f *FositeDatastoreStore) DeletePKCERequestSession(ctx context.Context, signature string) error {
	return f.deleteSession(ctx, f.createPKCEKey(signature))
}

func (f *FositeDatastoreStore) CreateImplicitAccessTokenSession(ctx context.Context, signature string, requester fosite.Requester) error {
	return f.CreateAccessTokenSession(ctx, signature, requester)
}

func (f *FositeDatastoreStore) RevokeRefreshToken(ctx context.Context, id string) error {
	return f.revokeSession(ctx, id, hydraOauth2RefreshKind)
}

func (f *FositeDatastoreStore) RevokeAccessToken(ctx context.Context, id string) error {
	return f.revokeSession(ctx, id, hydraOauth2AccessKind)
}

func (f *FositeDatastoreStore) FlushInactiveAccessTokens(ctx context.Context, notAfter time.Time) error {
	expireTime := time.Now().Add(-f.AccessTokenLifespan)
	query := f.newQueryForKind(hydraOauth2AccessKind).KeysOnly()
	if expireTime.Before(notAfter) {
		query = query.Filter("rat<", expireTime)
	} else {
		query = query.Filter("rat<", notAfter)
	}

	keys, err := f.client.GetAll(ctx, query, nil)
	if err != nil {
		return dscon.HandleError(err)
	}

	if err = f.client.DeleteMulti(ctx, keys); err != nil {
		return errors.WithStack(err)
	}
	return nil
}
