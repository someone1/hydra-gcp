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
	"github.com/pkg/errors"
)

var (
	// TypeCheck
	_ consent.Manager = (*DatastoreManager)(nil)

	errNoPreviousConsentFound = errors.New("No previous OAuth 2.0 Consent could be found for this access request")
)

// DatastoreManager is a Google Datastore implementation for oauth.ConsentRequestManager.
type DatastoreManager struct {
	client    *datastore.Client
	context   context.Context
	namespace string
	manager   client.Manager
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

// NewDatastoreManager initializes a new DatastoreManager with the given client
func NewDatastoreManager(ctx context.Context, client *datastore.Client, namespace string, c client.Manager) *DatastoreManager {
	return &DatastoreManager{
		client:    client,
		context:   ctx,
		namespace: namespace,
	}
}

func (d *DatastoreManager) CreateConsentRequest(c *consent.ConsentRequest) error {
	data, err := consentDataFromRequest(c)
	if err != nil {
		return err
	}

	key := d.createConsentReqKey(data.Challenge)
	mutation := datastore.NewInsert(key, data)

	if _, err := d.client.Mutate(d.context, mutation); err != nil {
		return errors.WithStack(err)
	}
	return nil
}

func (d *DatastoreManager) GetConsentRequest(challenge string) (*consent.ConsentRequest, error) {
	var c consentRequestData
	key := d.createConsentReqKey(challenge)

	if err := d.client.Get(d.context, key, &d); err == datastore.ErrNoSuchEntity {
		return nil, errors.WithStack(pkg.ErrNotFound)
	} else if err != nil {
		return nil, errors.WithStack(err)
	}

	if c.update {
		mutation := datastore.NewUpdate(key, &c)
		if _, err := d.client.Mutate(d.context, mutation); err != nil {
			return nil, errors.WithStack(err)
		}
		c.update = false
	}

	m, err := d.manager.GetConcreteClient(c.ClientID)
	if err != nil {
		return nil, err
	}

	return c.toConsentRequest(m)
}

func (d *DatastoreManager) CreateAuthenticationRequest(c *consent.AuthenticationRequest) error {
	data, err := consentDataFromAuthenticationRequest(c)
	if err != nil {
		return err
	}

	key := d.createConsentAuthReqKey(data.Challenge)
	mutation := datastore.NewInsert(key, data)

	if _, err := d.client.Mutate(d.context, mutation); err != nil {
		return errors.WithStack(err)
	}
	return nil
}

func (d *DatastoreManager) GetAuthenticationRequest(challenge string) (*consent.AuthenticationRequest, error) {
	var c consentRequestData
	key := d.createConsentAuthReqKey(challenge)

	if err := d.client.Get(d.context, key, &d); err == datastore.ErrNoSuchEntity {
		return nil, errors.WithStack(pkg.ErrNotFound)
	} else if err != nil {
		return nil, errors.WithStack(err)
	}

	if c.update {
		mutation := datastore.NewUpdate(key, &c)
		if _, err := d.client.Mutate(d.context, mutation); err != nil {
			return nil, errors.WithStack(err)
		}
		c.update = false
	}

	m, err := d.manager.GetConcreteClient(c.ClientID)
	if err != nil {
		return nil, err
	}

	return c.toAuthenticationRequest(m)
}

func (d *DatastoreManager) HandleConsentRequest(challenge string, r *consent.HandledConsentRequest) (*consent.ConsentRequest, error) {
	data, err := handledConsentRequest(r)
	if err != nil {
		return nil, err
	}

	key := d.createhandleConsentRequestKey(data.Challenge)
	mutation := datastore.NewInsert(key, data)

	if _, err := d.client.Mutate(d.context, mutation); err != nil {
		return nil, errors.WithStack(err)
	}
	return d.GetConsentRequest(challenge)
}

func (d *DatastoreManager) VerifyAndInvalidateConsentRequest(verifier string) (*consent.HandledConsentRequest, error) {
	var consentRequest consentRequestData
	var queryResults []consentRequestData
	var handledRequest handledConsentRequestData

	query := d.newQueryForKind(hydraConsentRequestKind).Filter("vfr=", verifier)
	_, err := d.client.GetAll(d.context, query, &queryResults)
	if err != nil {
		return nil, err
	} else if len(queryResults) != 1 {
		return nil, errors.Errorf("expected 1 consent request, got %d instead", len(queryResults))
	}
	consentRequest = queryResults[0]

	key := d.createhandleConsentRequestKey(consentRequest.Challenge)
	if err := d.client.Get(d.context, key, &handledRequest); err == datastore.ErrNoSuchEntity {
		return nil, errors.WithStack(pkg.ErrNotFound)
	} else if err != nil {
		return nil, errors.WithStack(err)
	}

	if handledRequest.WasUsed {
		return nil, errors.WithStack(fosite.ErrInvalidRequest.WithDebug("Consent verifier has been used already"))
	}

	r, err := d.GetConsentRequest(consentRequest.Challenge)
	if err != nil {
		return nil, err
	}

	handledRequest.WasUsed = true
	mutation := datastore.NewUpdate(key, &handledRequest)

	if _, err := d.client.Mutate(d.context, mutation); err != nil {
		return nil, errors.WithStack(err)
	}

	return handledRequest.toHandledConsentRequest(r)
}

func (d *DatastoreManager) HandleAuthenticationRequest(challenge string, r *consent.HandledAuthenticationRequest) (*consent.AuthenticationRequest, error) {
	data, err := handledAuthenticationRequest(r)
	if err != nil {
		return nil, err
	}

	key := d.createhandleConsentAuthenticationRequestKey(challenge)
	mutation := datastore.NewInsert(key, data)

	if _, err := d.client.Mutate(d.context, mutation); err != nil {
		return nil, errors.WithStack(err)
	}

	return d.GetAuthenticationRequest(challenge)
}

func (d *DatastoreManager) VerifyAndInvalidateAuthenticationRequest(verifier string) (*consent.HandledAuthenticationRequest, error) {
	var authReqData consentRequestData
	var queryResults []consentRequestData
	var handledAuthReqData handledAuthenticationConsentRequestData

	query := d.newQueryForKind(hydraConsentAunthenticationRequestKind).Filter("vfr=", verifier)
	_, err := d.client.GetAll(d.context, query, &queryResults)
	if err != nil {
		return nil, err
	} else if len(queryResults) != 1 {
		return nil, errors.Errorf("expected 1 consent auth request, got %d instead", len(queryResults))
	}
	authReqData = queryResults[0]

	key := d.createhandleConsentAuthenticationRequestKey(authReqData.Challenge)
	if err := d.client.Get(d.context, key, &handledAuthReqData); err == datastore.ErrNoSuchEntity {
		return nil, errors.WithStack(pkg.ErrNotFound)
	} else if err != nil {
		return nil, errors.WithStack(err)
	}

	if handledAuthReqData.WasUsed {
		return nil, errors.WithStack(fosite.ErrInvalidRequest.WithDebug("Consent verifier has been used already"))
	}

	handledAuthReqData.WasUsed = true
	mutation := datastore.NewUpdate(key, &handledAuthReqData)

	if _, err := d.client.Mutate(d.context, mutation); err != nil {
		return nil, errors.WithStack(err)
	}

	r, err := d.GetAuthenticationRequest(authReqData.Challenge)
	if err != nil {
		return nil, err
	}

	return handledAuthReqData.toHandledAuthenticationRequest(r)
}

func (d *DatastoreManager) GetAuthenticationSession(id string) (*consent.AuthenticationSession, error) {
	var a authenticationSession

	key := d.createAuthSessionKey(id)
	if err := d.client.Get(d.context, key, &a); err == datastore.ErrNoSuchEntity {
		return nil, errors.WithStack(pkg.ErrNotFound)
	} else if err != nil {
		return nil, errors.WithStack(err)
	}

	return a.toAuthenticationSession(), nil
}

func (d *DatastoreManager) CreateAuthenticationSession(a *consent.AuthenticationSession) error {
	data := fromAuthenticationSession(a)

	key := d.createAuthSessionKey(data.ID)
	mutation := datastore.NewInsert(key, data)

	if _, err := d.client.Mutate(d.context, mutation); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func (d *DatastoreManager) DeleteAuthenticationSession(id string) error {
	key := d.createAuthSessionKey(id)
	mutation := datastore.NewDelete(key)

	if _, err := d.client.Mutate(d.context, mutation); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func (d *DatastoreManager) FindPreviouslyGrantedConsentRequests(client string, subject string) ([]consent.HandledConsentRequest, error) {
	var a []handledConsentRequestData
	var consentReqs []consentRequestData

	query := d.newQueryForKind(hydraConsentRequestKind).Filter("cid=", client).Filter("sub=", subject).Filter("skip=", false)
	_, err := d.client.GetAll(d.context, query, &consentReqs)
	if err != nil {
		return nil, errors.WithStack(err)
	} else if len(consentReqs) == 0 {
		return nil, errors.WithStack(errNoPreviousConsentFound)
	}

	var keys []*datastore.Key
	for _, consentReq := range consentReqs {
		keys = append(keys, d.createhandleConsentRequestKey(consentReq.Challenge))
	}

	handledReqs := make([]handledConsentRequestData, 0, len(keys))
	err = d.client.GetMulti(d.context, keys, &handledReqs)
	if err != nil {
		return nil, errors.WithStack(err)
	} else if len(a) == 0 {
		return nil, errors.WithStack(errNoPreviousConsentFound)
	}

	for _, handledReq := range handledReqs {
		if handledReq.Remember && handledReq.Error == "{}" {
			a = append(a, handledReq)
		}
	}

	var aa []consent.HandledConsentRequest
	for _, v := range a {
		r, err := d.GetConsentRequest(v.Challenge)
		if err != nil {
			return nil, err
		} else if errors.Cause(err) == pkg.ErrNotFound {
			return nil, errors.WithStack(errNoPreviousConsentFound)
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
		return []consent.HandledConsentRequest{}, nil
	}

	return aa, nil
}
