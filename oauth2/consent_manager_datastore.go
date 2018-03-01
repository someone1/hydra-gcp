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
 * Based on https://github.com/ory/hydra/blob/master/oauth2/consent_manager_sql.go
 */

package oauth2

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/ory/hydra/oauth2"
	"github.com/ory/hydra/pkg"
	"github.com/pborman/uuid"
	"github.com/pkg/errors"
)

const (
	hydraConsentRequestKind = "HydraConsentRequest"
	consentVersion          = 1
)

type consentRequestData struct {
	Key              *datastore.Key `datastore:"-"`
	ID               string         `datastore:"-"`
	RequestedScopes  string         `datastore:"rscps"`
	ClientID         string         `datastore:"cid"`
	ExpiresAt        time.Time      `datastore:"eat"`
	RedirectURL      string         `datastore:"rurl"`
	CSRF             string         `datastore:"csrf"`
	GrantedScopes    string         `datastore:"gscps"`
	AccessTokenExtra string         `datastore:"acctknex"`
	IDTokenExtra     string         `datastore:"idtknex"`
	Consent          string         `datastore:"consent"`
	DenyReason       string         `datastore:"dr"`
	Subject          string         `datastore:"sub"`
	Version          int            `datastore:"v"`

	update bool
}

// LoadKey is implemented for the KeyLoader interface
func (c *consentRequestData) LoadKey(k *datastore.Key) error {
	c.Key = k
	c.ID = k.Name

	return nil
}

// Load is implemented for the PropertyLoadSaver interface, and performs schema migration if necessary
func (c *consentRequestData) Load(ps []datastore.Property) error {
	err := datastore.LoadStruct(c, ps)
	if _, ok := err.(*datastore.ErrFieldMismatch); err != nil && !ok {
		return errors.WithStack(err)
	}

	switch c.Version {
	case consentVersion:
		// Up to date, nothing to do
		break
	// case 1:
	// 	// Update to version 2 here
	// 	fallthrough
	// case 2:
	// 	//update to version 3 here
	// 	fallthrough
	case -1:
		// This is here to complete saving the entity should we need to udpate it
		if c.Version == -1 {
			return errors.New(fmt.Sprintf("unexpectedly got to version update trigger with incorrect version -1"))
		}
		c.Version = consentVersion
		c.update = true
	default:
		return errors.New(fmt.Sprintf("got unexpected version %d when loading entity", c.Version))
	}
	return nil
}

// Save is implemented for the PropertyLoadSaver interface
func (c *consentRequestData) Save() ([]datastore.Property, error) {
	c.Version = consentVersion
	return datastore.SaveStruct(c)
}

// ConsentRequestDatastoreManager is a Google Datastore implementation for oauth.ConsentRequestManager.
type ConsentRequestDatastoreManager struct {
	client    *datastore.Client
	context   context.Context
	namespace string
}

// NewConsentRequestDatastoreManager initializes a new ConsentRequestDatastoreManager with the given client
func NewConsentRequestDatastoreManager(ctx context.Context, client *datastore.Client, namespace string) *ConsentRequestDatastoreManager {
	return &ConsentRequestDatastoreManager{
		client:    client,
		context:   ctx,
		namespace: namespace,
	}
}

func (c *ConsentRequestDatastoreManager) createConsentReqKey(id string) *datastore.Key {
	key := datastore.NameKey(hydraConsentRequestKind, id, nil)
	key.Namespace = c.namespace
	return key
}

func consentDataFromRequest(request *oauth2.ConsentRequest) (*consentRequestData, error) {
	for k, scope := range request.RequestedScopes {
		request.RequestedScopes[k] = strings.Replace(scope, " ", "", -1)
	}
	for k, scope := range request.GrantedScopes {
		request.GrantedScopes[k] = strings.Replace(scope, " ", "", -1)
	}

	atext := ""
	idtext := ""

	if request.AccessTokenExtra != nil {
		if out, err := json.Marshal(request.AccessTokenExtra); err != nil {
			return nil, errors.WithStack(err)
		} else {
			atext = string(out)
		}
	}

	if request.IDTokenExtra != nil {
		if out, err := json.Marshal(request.IDTokenExtra); err != nil {
			return nil, errors.WithStack(err)
		} else {
			idtext = string(out)
		}
	}

	return &consentRequestData{
		ID:               request.ID,
		RequestedScopes:  strings.Join(request.RequestedScopes, " "),
		GrantedScopes:    strings.Join(request.GrantedScopes, " "),
		ClientID:         request.ClientID,
		ExpiresAt:        request.ExpiresAt,
		RedirectURL:      request.RedirectURL,
		CSRF:             request.CSRF,
		AccessTokenExtra: atext,
		IDTokenExtra:     idtext,
		Consent:          request.Consent,
		DenyReason:       request.DenyReason,
		Subject:          request.Subject,
	}, nil
}

func (c *consentRequestData) toConsentRequest() (*oauth2.ConsentRequest, error) {
	var atext, idtext map[string]interface{}

	if c.IDTokenExtra != "" {
		if err := json.Unmarshal([]byte(c.IDTokenExtra), &idtext); err != nil {
			return nil, errors.WithStack(err)
		}
	}

	if c.AccessTokenExtra != "" {
		if err := json.Unmarshal([]byte(c.AccessTokenExtra), &atext); err != nil {
			return nil, errors.WithStack(err)
		}
	}

	return &oauth2.ConsentRequest{
		ID:               c.ID,
		ClientID:         c.ClientID,
		ExpiresAt:        c.ExpiresAt,
		RedirectURL:      c.RedirectURL,
		CSRF:             c.CSRF,
		Consent:          c.Consent,
		DenyReason:       c.DenyReason,
		RequestedScopes:  strings.Split(c.RequestedScopes, " "),
		GrantedScopes:    strings.Split(c.GrantedScopes, " "),
		AccessTokenExtra: atext,
		IDTokenExtra:     idtext,
		Subject:          c.Subject,
	}, nil
}

func (c *ConsentRequestDatastoreManager) PersistConsentRequest(request *oauth2.ConsentRequest) error {
	if request.ID == "" {
		request.ID = uuid.New()
	}

	data, err := consentDataFromRequest(request)
	if err != nil {
		return errors.WithStack(err)
	}

	key := c.createConsentReqKey(data.ID)
	mutation := datastore.NewInsert(key, data)

	if _, err := c.client.Mutate(c.context, mutation); err != nil {
		return errors.WithStack(err)
	}
	return nil
}

func (c *ConsentRequestDatastoreManager) AcceptConsentRequest(id string, payload *oauth2.AcceptConsentRequestPayload) error {
	r, err := c.GetConsentRequest(id)
	if err != nil {
		return errors.WithStack(err)
	}

	r.Subject = payload.Subject
	r.AccessTokenExtra = payload.AccessTokenExtra
	r.IDTokenExtra = payload.IDTokenExtra
	r.Consent = oauth2.ConsentRequestAccepted
	r.GrantedScopes = payload.GrantScopes

	return c.updateConsentRequest(r)
}

func (c *ConsentRequestDatastoreManager) RejectConsentRequest(id string, payload *oauth2.RejectConsentRequestPayload) error {
	r, err := c.GetConsentRequest(id)
	if err != nil {
		return errors.WithStack(err)
	}

	r.Consent = oauth2.ConsentRequestRejected
	r.DenyReason = payload.Reason

	return c.updateConsentRequest(r)
}

func (c *ConsentRequestDatastoreManager) updateConsentRequest(request *oauth2.ConsentRequest) error {
	d, err := consentDataFromRequest(request)
	if err != nil {
		return errors.WithStack(err)
	}

	key := c.createConsentReqKey(d.ID)
	mutation := datastore.NewUpdate(key, d)

	if _, err := c.client.Mutate(c.context, mutation); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func (c *ConsentRequestDatastoreManager) GetConsentRequest(id string) (*oauth2.ConsentRequest, error) {
	var d consentRequestData
	key := c.createConsentReqKey(id)

	if err := c.client.Get(c.context, key, &d); err == datastore.ErrNoSuchEntity {
		return nil, errors.WithStack(pkg.ErrNotFound)
	} else if err != nil {
		return nil, errors.WithStack(err)
	}

	if d.update {
		mutation := datastore.NewUpdate(key, &d)
		if _, err := c.client.Mutate(c.context, mutation); err != nil {
			return nil, errors.WithStack(err)
		}
		d.update = false
	}

	r, err := d.toConsentRequest()
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return r, nil
}

func consenttypecheck() {
	var _ oauth2.ConsentRequestManager = (*ConsentRequestDatastoreManager)(nil)
}
