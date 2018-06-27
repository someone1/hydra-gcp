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
 * Based on https://github.com/ory/hydra/blob/master/consent/sql_helper.go
 */

package consent

import (
	"encoding/json"
	"strings"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/ory/go-convenience/stringsx"
	"github.com/ory/hydra/client"
	"github.com/ory/hydra/consent"
	"github.com/pkg/errors"
)

const (
	hydraConsentRequestKind                       = "HydraConsentRequest"
	hydraConsentAunthenticationRequestKind        = "HydraConsentAuthenticationRequest"
	hydraConsentRequestHandledKind                = "HydraConsentRequestHandled"
	hydraConsentAunthenticationRequestHandledKind = "HydraConsentAuthenticationRequestHandled"
	hydraConsentAunthenticationSessionKind        = "HydraConsentAuthenticationSession"
	hydraAncestorName                             = "default"
	consentVersion                                = 1
	handleVersion                                 = 1
	handleAuthVersion                             = 1
	sessionVersion                                = 1
)

func toDateHack(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

func fromDateHack(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return *t
}

type consentRequestData struct {
	Key                  *datastore.Key `datastore:"-"`
	Challenge            string         `datastore:"-"`
	Verifier             string         `datastore:"vfr"`
	ClientID             string         `datastore:"cid"`
	Subject              string         `datastore:"sub"`
	RequestURL           string         `datastore:"rurl"`
	Skip                 bool           `datastore:"skip"`
	RequestedScope       string         `datastore:"rscp"`
	CSRF                 string         `datastore:"csrf"`
	AuthenticatedAt      *time.Time     `datastore:"aat"`
	RequestedAt          time.Time      `datastore:"ra"`
	OpenIDConnectContext string         `datastore:"oidcctx"`

	Version int `datastore:"v"`
	update  bool
}

// LoadKey is implemented for the KeyLoader interface
func (c *consentRequestData) LoadKey(k *datastore.Key) error {
	c.Key = k
	c.Challenge = k.Name

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
			return errors.Errorf("unexpectedly got to version update trigger with incorrect version -1")
		}
		c.Version = consentVersion
		c.update = true
	default:
		return errors.Errorf("got unexpected version %d when loading entity", c.Version)
	}
	return nil
}

// Save is implemented for the PropertyLoadSaver interface
func (c *consentRequestData) Save() ([]datastore.Property, error) {
	c.Version = consentVersion
	if c.RequestedAt.IsZero() {
		c.RequestedAt = time.Now()
	}
	return datastore.SaveStruct(c)
}

func consentDataFromRequest(c *consent.ConsentRequest) (*consentRequestData, error) {
	oidc, err := json.Marshal(c.OpenIDConnectContext)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return &consentRequestData{
		OpenIDConnectContext: string(oidc),
		ClientID:             c.Client.GetID(),
		Subject:              c.Subject,
		RequestURL:           c.RequestURL,
		Skip:                 c.Skip,
		Challenge:            c.Challenge,
		RequestedScope:       strings.Join(c.RequestedScope, "|"),
		Verifier:             c.Verifier,
		CSRF:                 c.CSRF,
		AuthenticatedAt:      toDateHack(c.AuthenticatedAt),
		RequestedAt:          c.RequestedAt,
	}, nil
}

func consentDataFromAuthenticationRequest(c *consent.AuthenticationRequest) (*consentRequestData, error) {
	var cc consent.ConsentRequest
	cc = consent.ConsentRequest(*c)
	return consentDataFromRequest(&cc)
}

func (c *consentRequestData) toConsentRequest(client *client.Client) (*consent.ConsentRequest, error) {
	var oidc consent.OpenIDConnectContext
	if err := json.Unmarshal([]byte(c.OpenIDConnectContext), &oidc); err != nil {
		return nil, errors.WithStack(err)
	}

	return &consent.ConsentRequest{
		OpenIDConnectContext: &oidc,
		Client:               client,
		Subject:              c.Subject,
		RequestURL:           c.RequestURL,
		Skip:                 c.Skip,
		Challenge:            c.Challenge,
		RequestedScope:       stringsx.Splitx(c.RequestedScope, "|"),
		Verifier:             c.Verifier,
		CSRF:                 c.CSRF,
		AuthenticatedAt:      fromDateHack(c.AuthenticatedAt),
		RequestedAt:          c.RequestedAt,
	}, nil
}

func (c *consentRequestData) toAuthenticationRequest(client *client.Client) (*consent.AuthenticationRequest, error) {
	cr, err := c.toConsentRequest(client)
	if err != nil {
		return nil, err
	}

	var ar consent.AuthenticationRequest
	ar = consent.AuthenticationRequest(*cr)
	return &ar, nil
}

type handledConsentRequestData struct {
	Key                *datastore.Key `datastore:"-"`
	GrantedScope       string         `datastore:"gscp"`
	SessionIDToken     string         `datastore:"sidt"`
	SessionAccessToken string         `datastore:"sact"`
	Remember           bool           `datastore:"rmbr"`
	RememberFor        int            `datatstore:"rmbrf"`
	Error              string         `datastore:"err"`
	Challenge          string         `datastore:"-"`
	RequestedAt        time.Time      `datastore:"rat"`
	WasUsed            bool           `datastore:"wsu"`
	AuthenticatedAt    *time.Time     `db:"aat"`

	Version int `datastore:"v"`
	update  bool
}

// LoadKey is implemented for the KeyLoader interface
func (h *handledConsentRequestData) LoadKey(k *datastore.Key) error {
	h.Key = k
	h.Challenge = k.Name

	return nil
}

// Load is implemented for the PropertyLoadSaver interface, and performs schema migration if necessary
func (h *handledConsentRequestData) Load(ps []datastore.Property) error {
	err := datastore.LoadStruct(h, ps)
	if _, ok := err.(*datastore.ErrFieldMismatch); err != nil && !ok {
		return errors.WithStack(err)
	}

	switch h.Version {
	case handleVersion:
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
		if h.Version == -1 {
			return errors.Errorf("unexpectedly got to version update trigger with incorrect version -1")
		}
		h.Version = handleVersion
		h.update = true
	default:
		return errors.Errorf("got unexpected version %d when loading entity", h.Version)
	}
	return nil
}

// Save is implemented for the PropertyLoadSaver interface
func (h *handledConsentRequestData) Save() ([]datastore.Property, error) {
	h.Version = handleVersion
	if h.RequestedAt.IsZero() {
		h.RequestedAt = time.Now()
	}
	return datastore.SaveStruct(h)
}

func handledConsentRequest(c *consent.HandledConsentRequest) (*handledConsentRequestData, error) {
	sidt := "{}"
	sat := "{}"
	e := "{}"

	if c.Session != nil {
		if len(c.Session.IDToken) > 0 {
			if out, err := json.Marshal(c.Session.IDToken); err != nil {
				return nil, errors.WithStack(err)
			} else {
				sidt = string(out)
			}
		}

		if len(c.Session.AccessToken) > 0 {
			if out, err := json.Marshal(c.Session.AccessToken); err != nil {
				return nil, errors.WithStack(err)
			} else {
				sat = string(out)
			}
		}
	}

	if c.Error != nil {
		if out, err := json.Marshal(c.Error); err != nil {
			return nil, errors.WithStack(err)
		} else {
			e = string(out)
		}
	}

	return &handledConsentRequestData{
		GrantedScope:       strings.Join(c.GrantedScope, "|"),
		SessionIDToken:     sidt,
		SessionAccessToken: sat,
		Remember:           c.Remember,
		RememberFor:        c.RememberFor,
		Error:              e,
		Challenge:          c.Challenge,
		RequestedAt:        c.RequestedAt,
		WasUsed:            c.WasUsed,
		AuthenticatedAt:    toDateHack(c.AuthenticatedAt),
	}, nil
}

func (h *handledConsentRequestData) toHandledConsentRequest(r *consent.ConsentRequest) (*consent.HandledConsentRequest, error) {
	var idt map[string]interface{}
	var at map[string]interface{}
	var e *consent.RequestDeniedError

	if err := json.Unmarshal([]byte(h.SessionIDToken), &idt); err != nil {
		return nil, errors.WithStack(err)
	}
	if err := json.Unmarshal([]byte(h.SessionAccessToken), &at); err != nil {
		return nil, errors.WithStack(err)
	}

	if len(h.Error) > 0 && h.Error != "{}" {
		e = new(consent.RequestDeniedError)
		if err := json.Unmarshal([]byte(h.Error), &e); err != nil {
			return nil, errors.WithStack(err)
		}
	}

	return &consent.HandledConsentRequest{
		GrantedScope: stringsx.Splitx(h.GrantedScope, "|"),
		RememberFor:  h.RememberFor,
		Remember:     h.Remember,
		Challenge:    h.Challenge,
		RequestedAt:  h.RequestedAt,
		WasUsed:      h.WasUsed,
		Session: &consent.ConsentRequestSessionData{
			IDToken:     idt,
			AccessToken: at,
		},
		Error:           e,
		ConsentRequest:  r,
		AuthenticatedAt: fromDateHack(h.AuthenticatedAt),
	}, nil
}

type handledAuthenticationConsentRequestData struct {
	Key             *datastore.Key `datastore:"-"`
	Remember        bool           `datastore:"rmbr"`
	RememberFor     int            `datatstore:"rmbrf"`
	ACR             string         `datastore:"acr"`
	Subject         string         `datastore:"sub"`
	Error           string         `datastore:"err"`
	Challenge       string         `datastore:"-"`
	RequestedAt     time.Time      `datastore:"rat"`
	WasUsed         bool           `datastore:"wsu"`
	AuthenticatedAt *time.Time     `db:"aat"`

	Version int `datastore:"v"`
	update  bool
}

// LoadKey is implemented for the KeyLoader interface
func (h *handledAuthenticationConsentRequestData) LoadKey(k *datastore.Key) error {
	h.Key = k
	h.Challenge = k.Name

	return nil
}

// Load is implemented for the PropertyLoadSaver interface, and performs schema migration if necessary
func (h *handledAuthenticationConsentRequestData) Load(ps []datastore.Property) error {
	err := datastore.LoadStruct(h, ps)
	if _, ok := err.(*datastore.ErrFieldMismatch); err != nil && !ok {
		return errors.WithStack(err)
	}

	switch h.Version {
	case handleAuthVersion:
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
		if h.Version == -1 {
			return errors.Errorf("unexpectedly got to version update trigger with incorrect version -1")
		}
		h.Version = handleAuthVersion
		h.update = true
	default:
		return errors.Errorf("got unexpected version %d when loading entity", h.Version)
	}
	return nil
}

// Save is implemented for the PropertyLoadSaver interface
func (h *handledAuthenticationConsentRequestData) Save() ([]datastore.Property, error) {
	h.Version = handleAuthVersion
	if h.RequestedAt.IsZero() {
		h.RequestedAt = time.Now()
	}
	return datastore.SaveStruct(h)
}

func handledAuthenticationRequest(c *consent.HandledAuthenticationRequest) (*handledAuthenticationConsentRequestData, error) {
	e := "{}"

	if c.Error != nil {
		if out, err := json.Marshal(c.Error); err != nil {
			return nil, errors.WithStack(err)
		} else {
			e = string(out)
		}
	}

	return &handledAuthenticationConsentRequestData{
		ACR:             c.ACR,
		Subject:         c.Subject,
		Remember:        c.Remember,
		RememberFor:     c.RememberFor,
		Error:           e,
		Challenge:       c.Challenge,
		RequestedAt:     c.RequestedAt,
		WasUsed:         c.WasUsed,
		AuthenticatedAt: toDateHack(c.AuthenticatedAt),
	}, nil
}

func (h *handledAuthenticationConsentRequestData) toHandledAuthenticationRequest(a *consent.AuthenticationRequest) (*consent.HandledAuthenticationRequest, error) {
	var e *consent.RequestDeniedError

	if len(h.Error) > 0 && h.Error != "{}" {
		e = new(consent.RequestDeniedError)
		if err := json.Unmarshal([]byte(h.Error), &e); err != nil {
			return nil, errors.WithStack(err)
		}
	}

	return &consent.HandledAuthenticationRequest{
		RememberFor: h.RememberFor,
		Remember:    h.Remember,
		Challenge:   h.Challenge,
		RequestedAt: h.RequestedAt,
		WasUsed:     h.WasUsed,
		ACR:         h.ACR,
		Error:       e,
		AuthenticationRequest: a,
		Subject:               h.Subject,
		AuthenticatedAt:       fromDateHack(h.AuthenticatedAt),
	}, nil
}

type authenticationSession struct {
	Key             *datastore.Key `datastore:"-"`
	ID              string         `datastore:"-"`
	AuthenticatedAt time.Time      `datastore:"aat"`
	Subject         string         `datastore:"sub"`

	Version int `datastore:"v"`
	update  bool
}

// LoadKey is implemented for the KeyLoader interface
func (a *authenticationSession) LoadKey(k *datastore.Key) error {
	a.Key = k
	a.ID = k.Name

	return nil
}

// Load is implemented for the PropertyLoadSaver interface, and performs schema migration if necessary
func (a *authenticationSession) Load(ps []datastore.Property) error {
	err := datastore.LoadStruct(a, ps)
	if _, ok := err.(*datastore.ErrFieldMismatch); err != nil && !ok {
		return errors.WithStack(err)
	}

	switch a.Version {
	case sessionVersion:
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
		if a.Version == -1 {
			return errors.Errorf("unexpectedly got to version update trigger with incorrect version -1")
		}
		a.Version = sessionVersion
		a.update = true
	default:
		return errors.Errorf("got unexpected version %d when loading entity", a.Version)
	}
	return nil
}

// Save is implemented for the PropertyLoadSaver interface
func (a *authenticationSession) Save() ([]datastore.Property, error) {
	a.Version = sessionVersion
	if a.AuthenticatedAt.IsZero() {
		a.AuthenticatedAt = time.Now()
	}
	return datastore.SaveStruct(a)
}

func (a *authenticationSession) toAuthenticationSession() *consent.AuthenticationSession {
	return &consent.AuthenticationSession{
		AuthenticatedAt: a.AuthenticatedAt,
		ID:              a.ID,
		Subject:         a.Subject,
	}
}

func fromAuthenticationSession(a *consent.AuthenticationSession) *authenticationSession {
	return &authenticationSession{
		AuthenticatedAt: a.AuthenticatedAt,
		ID:              a.ID,
		Subject:         a.Subject,
	}
}
