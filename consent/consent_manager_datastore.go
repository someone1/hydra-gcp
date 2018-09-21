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
 * Based on https://github.com/ory/hydra/blob/master/consent/manager_sql.go
 */

package consent

import (
	"context"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/ory/fosite"
	"github.com/ory/hydra/client"
	"github.com/ory/hydra/consent"
	"github.com/ory/hydra/pkg"
	"github.com/ory/pagination"
	"github.com/pkg/errors"
)

var (
	// TypeCheck
	_ consent.Manager = (*DatastoreManager)(nil)
)

// DatastoreManager is a Google Datastore implementation for oauth.ConsentRequestManager.
type DatastoreManager struct {
	client    *datastore.Client
	namespace string
	manager   client.Manager
	store     pkg.FositeStorer
}

func (d *DatastoreManager) createAncestorKeyForKind(kind string) *datastore.Key {
	key := datastore.NameKey(kind, hydraAncestorName, nil)
	key.Namespace = d.namespace
	return key
}

func (d *DatastoreManager) createKeyForKind(id, kind string) *datastore.Key {
	key := datastore.NameKey(kind, id, d.createAncestorKeyForKind(kind))
	key.Namespace = d.namespace
	return key
}

func (d *DatastoreManager) newQueryForKind(kind string) *datastore.Query {
	return datastore.NewQuery(kind).Ancestor(d.createAncestorKeyForKind(kind)).Namespace(d.namespace)
}

func (d *DatastoreManager) createConsentReqKey(id string) *datastore.Key {
	return d.createKeyForKind(id, hydraConsentRequestKind)
}

func (d *DatastoreManager) createConsentAuthReqKey(id string) *datastore.Key {
	return d.createKeyForKind(id, hydraConsentAunthenticationRequestKind)
}

func (d *DatastoreManager) createhandleConsentRequestKey(id string) *datastore.Key {
	return d.createKeyForKind(id, hydraConsentRequestHandledKind)
}

func (d *DatastoreManager) createhandleConsentAuthenticationRequestKey(id string) *datastore.Key {
	return d.createKeyForKind(id, hydraConsentAunthenticationRequestHandledKind)
}

func (d *DatastoreManager) createAuthSessionKey(id string) *datastore.Key {
	return d.createKeyForKind(id, hydraConsentAunthenticationSessionKind)
}

func (d *DatastoreManager) createObfuscatedAuthSessionKey(client, subject string) *datastore.Key {
	return d.createKeyForKind(client+subject, hydraConsentObfuscatedAuthenticationSessionKind)
}

// NewDatastoreManager initializes a new DatastoreManager with the given client
func NewDatastoreManager(client *datastore.Client, namespace string, c client.Manager, store pkg.FositeStorer) *DatastoreManager {
	return &DatastoreManager{
		client:    client,
		namespace: namespace,
		manager:   c,
		store:     store,
	}
}

func (d *DatastoreManager) RevokeUserConsentSession(ctx context.Context, user string) error {
	return d.revokeConsentSession(ctx, user, "")
}

func (d *DatastoreManager) RevokeUserClientConsentSession(ctx context.Context, user, client string) error {
	return d.revokeConsentSession(ctx, user, client)
}

func (d *DatastoreManager) revokeConsentSession(ctx context.Context, user, client string) error {
	query := d.newQueryForKind(hydraConsentRequestKind).Filter("sub=", user).KeysOnly()
	if client != "" {
		query = query.Filter("cid=", client)
	}

	keys, err := d.client.GetAll(ctx, query, nil)
	if err != nil {
		return err
	} else if len(keys) == 0 {
		return errors.WithStack(pkg.ErrNotFound)
	}

	handledKeys := make([]*datastore.Key, len(keys))
	handledRequests := make([]handledConsentRequestData, len(keys))
	for idx, key := range keys {
		handledKeys[idx] = d.createhandleConsentRequestKey(key.Name)
	}

	err = d.client.GetMulti(ctx, handledKeys, handledRequests)
	var merr datastore.MultiError = nil
	var ok bool
	if err != nil {
		if merr, ok = err.(datastore.MultiError); !ok {
			return err
		}
	}

	var toDelete []*datastore.Key

	for idx, handledRequest := range handledRequests {
		if merr != nil && merr[idx] != nil && merr[idx] != datastore.ErrNoSuchEntity {
			return merr[idx]
		} else if merr != nil && merr[idx] == datastore.ErrNoSuchEntity {
			continue
		}

		challenge := handledRequest.Challenge
		if err := d.store.RevokeAccessToken(nil, challenge); errors.Cause(err) == fosite.ErrNotFound {
			// do nothing
		} else if err != nil {
			return err
		}
		if err := d.store.RevokeRefreshToken(nil, challenge); errors.Cause(err) == fosite.ErrNotFound {
			// do nothing
		} else if err != nil {
			return err
		}
		toDelete = append(toDelete, keys[idx], handledKeys[idx])
	}

	if len(toDelete) == 0 {
		return errors.WithStack(pkg.ErrNotFound)
	}

	err = d.client.DeleteMulti(ctx, toDelete)
	return err
}

func (d *DatastoreManager) RevokeUserAuthenticationSession(ctx context.Context, subject string) error {
	query := d.newQueryForKind(hydraConsentAunthenticationSessionKind).Filter("sub=", subject).KeysOnly()
	keys, err := d.client.GetAll(ctx, query, nil)
	if err != nil {
		return err
	} else if len(keys) == 0 {
		return errors.WithStack(pkg.ErrNotFound)
	}

	err = d.client.DeleteMulti(ctx, keys)

	return err
}

func (d *DatastoreManager) CreateForcedObfuscatedAuthenticationSession(ctx context.Context, s *consent.ForcedObfuscatedAuthenticationSession) error {
	key := d.createObfuscatedAuthSessionKey(s.ClientID, s.Subject)
	mutation := datastore.NewUpsert(key, s)
	_, err := d.client.Mutate(ctx, mutation)
	return err
}

func (d *DatastoreManager) GetForcedObfuscatedAuthenticationSession(ctx context.Context, client, obfuscated string) (*consent.ForcedObfuscatedAuthenticationSession, error) {
	var o []consent.ForcedObfuscatedAuthenticationSession
	query := d.newQueryForKind(hydraConsentObfuscatedAuthenticationSessionKind).Filter("ClientID=", client).Filter("SubjectObfuscated=", obfuscated)
	_, err := d.client.GetAll(ctx, query, &o)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if len(o) == 0 {
		return nil, errors.WithStack(pkg.ErrNotFound)
	}

	return &o[0], nil
}

func (d *DatastoreManager) CreateConsentRequest(ctx context.Context, c *consent.ConsentRequest) error {
	data, err := consentDataFromRequest(c)
	if err != nil {
		return err
	}

	key := d.createConsentReqKey(data.Challenge)
	mutation := datastore.NewInsert(key, data)

	if _, err := d.client.Mutate(ctx, mutation); err != nil {
		return errors.WithStack(err)
	}
	return nil
}

func (d *DatastoreManager) GetConsentRequest(ctx context.Context, challenge string) (*consent.ConsentRequest, error) {
	var c consentRequestData
	key := d.createConsentReqKey(challenge)

	if err := d.client.Get(ctx, key, &c); err == datastore.ErrNoSuchEntity {
		return nil, errors.WithStack(pkg.ErrNotFound)
	} else if err != nil {
		return nil, errors.WithStack(err)
	}

	if c.update {
		mutation := datastore.NewUpdate(key, &c)
		if _, err := d.client.Mutate(ctx, mutation); err != nil {
			return nil, errors.WithStack(err)
		}
		c.update = false
	}

	m, err := d.manager.GetConcreteClient(ctx, c.ClientID)
	if err != nil {
		return nil, err
	}

	return c.toConsentRequest(m)
}

func (d *DatastoreManager) CreateAuthenticationRequest(ctx context.Context, c *consent.AuthenticationRequest) error {
	data, err := authenticationDataFromRequest(c)
	if err != nil {
		return err
	}

	key := d.createConsentAuthReqKey(data.Challenge)
	mutation := datastore.NewInsert(key, data)

	if _, err := d.client.Mutate(ctx, mutation); err != nil {
		return errors.WithStack(err)
	}
	return nil
}

func (d *DatastoreManager) GetAuthenticationRequest(ctx context.Context, challenge string) (*consent.AuthenticationRequest, error) {
	var c consentRequestData
	key := d.createConsentAuthReqKey(challenge)

	if err := d.client.Get(ctx, key, &c); err == datastore.ErrNoSuchEntity {
		return nil, errors.WithStack(pkg.ErrNotFound)
	} else if err != nil {
		return nil, errors.WithStack(err)
	}

	if c.update {
		mutation := datastore.NewUpdate(key, &c)
		if _, err := d.client.Mutate(ctx, mutation); err != nil {
			return nil, errors.WithStack(err)
		}
		c.update = false
	}

	m, err := d.manager.GetConcreteClient(ctx, c.ClientID)
	if err != nil {
		return nil, err
	}

	return c.toAuthenticationRequest(m)
}

func (d *DatastoreManager) HandleConsentRequest(ctx context.Context, challenge string, r *consent.HandledConsentRequest) (*consent.ConsentRequest, error) {
	data, err := handledConsentRequest(r)
	if err != nil {
		return nil, err
	}

	key := d.createhandleConsentRequestKey(data.Challenge)
	mutation := datastore.NewInsert(key, data)

	if _, err := d.client.Mutate(ctx, mutation); err != nil {
		return nil, errors.WithStack(err)
	}
	return d.GetConsentRequest(ctx, challenge)
}

func (d *DatastoreManager) VerifyAndInvalidateConsentRequest(ctx context.Context, verifier string) (*consent.HandledConsentRequest, error) {
	var consentRequest consentRequestData
	var queryResults []consentRequestData
	var handledRequest handledConsentRequestData

	query := d.newQueryForKind(hydraConsentRequestKind).Filter("vfr=", verifier)
	_, err := d.client.GetAll(ctx, query, &queryResults)
	if err != nil {
		return nil, err
	} else if len(queryResults) != 1 {
		return nil, errors.Errorf("expected 1 consent request, got %d instead", len(queryResults))
	}
	consentRequest = queryResults[0]

	key := d.createhandleConsentRequestKey(consentRequest.Challenge)
	if err := d.client.Get(ctx, key, &handledRequest); err == datastore.ErrNoSuchEntity {
		return nil, errors.WithStack(pkg.ErrNotFound)
	} else if err != nil {
		return nil, errors.WithStack(err)
	}

	if handledRequest.WasUsed {
		return nil, errors.WithStack(fosite.ErrInvalidRequest.WithDebug("Consent verifier has been used already"))
	}

	r, err := d.GetConsentRequest(ctx, consentRequest.Challenge)
	if err != nil {
		return nil, err
	}

	handledRequest.WasUsed = true
	mutation := datastore.NewUpdate(key, &handledRequest)

	if _, err := d.client.Mutate(ctx, mutation); err != nil {
		return nil, errors.WithStack(err)
	}

	return handledRequest.toHandledConsentRequest(r)
}

func (d *DatastoreManager) HandleAuthenticationRequest(ctx context.Context, challenge string, r *consent.HandledAuthenticationRequest) (*consent.AuthenticationRequest, error) {
	data, err := handledAuthenticationRequest(r)
	if err != nil {
		return nil, err
	}

	key := d.createhandleConsentAuthenticationRequestKey(challenge)
	mutation := datastore.NewInsert(key, data)

	if _, err := d.client.Mutate(ctx, mutation); err != nil {
		return nil, errors.WithStack(err)
	}

	return d.GetAuthenticationRequest(ctx, challenge)
}

func (d *DatastoreManager) VerifyAndInvalidateAuthenticationRequest(ctx context.Context, verifier string) (*consent.HandledAuthenticationRequest, error) {
	var authReqData consentRequestData
	var queryResults []consentRequestData
	var handledAuthReqData handledAuthenticationConsentRequestData

	query := d.newQueryForKind(hydraConsentAunthenticationRequestKind).Filter("vfr=", verifier)
	_, err := d.client.GetAll(ctx, query, &queryResults)
	if err != nil {
		return nil, err
	} else if len(queryResults) != 1 {
		return nil, errors.Errorf("expected 1 consent auth request, got %d instead", len(queryResults))
	}
	authReqData = queryResults[0]

	key := d.createhandleConsentAuthenticationRequestKey(authReqData.Challenge)
	if err := d.client.Get(ctx, key, &handledAuthReqData); err == datastore.ErrNoSuchEntity {
		return nil, errors.WithStack(pkg.ErrNotFound)
	} else if err != nil {
		return nil, errors.WithStack(err)
	}

	if handledAuthReqData.WasUsed {
		return nil, errors.WithStack(fosite.ErrInvalidRequest.WithDebug("Consent verifier has been used already"))
	}

	handledAuthReqData.WasUsed = true
	mutation := datastore.NewUpdate(key, &handledAuthReqData)

	if _, err := d.client.Mutate(ctx, mutation); err != nil {
		return nil, errors.WithStack(err)
	}

	r, err := d.GetAuthenticationRequest(ctx, authReqData.Challenge)
	if err != nil {
		return nil, err
	}

	return handledAuthReqData.toHandledAuthenticationRequest(r)
}

func (d *DatastoreManager) GetAuthenticationSession(ctx context.Context, id string) (*consent.AuthenticationSession, error) {
	var a authenticationSession

	key := d.createAuthSessionKey(id)
	if err := d.client.Get(ctx, key, &a); err == datastore.ErrNoSuchEntity {
		return nil, errors.WithStack(pkg.ErrNotFound)
	} else if err != nil {
		return nil, errors.WithStack(err)
	}

	return a.toAuthenticationSession(), nil
}

func (d *DatastoreManager) CreateAuthenticationSession(ctx context.Context, a *consent.AuthenticationSession) error {
	data := fromAuthenticationSession(a)

	key := d.createAuthSessionKey(data.ID)
	mutation := datastore.NewInsert(key, data)

	if _, err := d.client.Mutate(ctx, mutation); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func (d *DatastoreManager) DeleteAuthenticationSession(ctx context.Context, id string) error {
	key := d.createAuthSessionKey(id)
	mutation := datastore.NewDelete(key)

	if _, err := d.client.Mutate(ctx, mutation); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func (d *DatastoreManager) FindPreviouslyGrantedConsentRequests(ctx context.Context, client string, subject string) ([]consent.HandledConsentRequest, error) {
	var a []handledConsentRequestData
	var consentReqs []consentRequestData

	query := d.newQueryForKind(hydraConsentRequestKind).Filter("cid=", client).Filter("sub=", subject).Filter("skip=", false)
	_, err := d.client.GetAll(ctx, query, &consentReqs)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	var keys []*datastore.Key
	for _, consentReq := range consentReqs {
		keys = append(keys, d.createhandleConsentRequestKey(consentReq.Challenge))
	}

	handledReqs := make([]handledConsentRequestData, len(keys))
	err = d.client.GetMulti(ctx, keys, handledReqs)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	for _, handledReq := range handledReqs {
		if handledReq.Remember && handledReq.Error == "{}" {
			a = append(a, handledReq)
		}
	}

	return d.resolveHandledConsentRequests(ctx, a)
}

func (d *DatastoreManager) FindPreviouslyGrantedConsentRequestsByUser(ctx context.Context, subject string, limit, offset int) ([]consent.HandledConsentRequest, error) {
	var a []handledConsentRequestData
	var consentReqs []consentRequestData

	query := d.newQueryForKind(hydraConsentRequestKind).Filter("sub=", subject).Filter("skip=", false)
	_, err := d.client.GetAll(ctx, query, &consentReqs)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	var keys []*datastore.Key
	for _, consentReq := range consentReqs {
		keys = append(keys, d.createhandleConsentRequestKey(consentReq.Challenge))
	}

	handledReqs := make([]handledConsentRequestData, len(keys))
	err = d.client.GetMulti(ctx, keys, handledReqs)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	for _, handledReq := range handledReqs {
		if handledReq.Remember && handledReq.Error == "{}" {
			a = append(a, handledReq)
		}
	}

	aa, aerr := d.resolveHandledConsentRequests(ctx, a)
	if aerr != nil {
		return nil, aerr
	}

	if limit < 0 && offset < 0 {
		return aa, nil
	}

	start, end := pagination.Index(limit, offset, len(aa))
	return aa[start:end], nil
}

func (d *DatastoreManager) resolveHandledConsentRequests(ctx context.Context, requests []handledConsentRequestData) ([]consent.HandledConsentRequest, error) {
	var aa []consent.HandledConsentRequest
	for _, v := range requests {
		r, err := d.GetConsentRequest(ctx, v.Challenge)
		if err != nil {
			return nil, err
		} else if errors.Cause(err) == pkg.ErrNotFound {
			return nil, errors.WithStack(consent.ErrNoPreviousConsentFound)
		}

		if v.RememberFor > 0 && v.RequestedAt.Add(time.Duration(v.RememberFor)*time.Second).Before(time.Now().UTC()) {
			continue
		}

		va, err := v.toHandledConsentRequest(r)
		if err != nil {
			return nil, err
		}

		aa = append(aa, *va)
	}

	if len(aa) == 0 {
		return nil, errors.WithStack(consent.ErrNoPreviousConsentFound)
	}

	return aa, nil
}
