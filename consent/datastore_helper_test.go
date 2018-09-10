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
 * @Copyright 	2017-2018 Aeneas Rekkas <aeneas+oss@aeneas.io>
 * @license 	Apache-2.0

  Based on https://github.com/ory/hydra/blob/master/consent/sql_helper_test.go and updated for Datastore
*/

package consent

import (
	"testing"
	"time"

	"github.com/ory/hydra/client"
	"github.com/ory/hydra/consent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDateHacks(t *testing.T) {
	now := time.Now().UTC()
	assert.EqualValues(t, now, fromDateHack(toDateHack(now)))
	assert.EqualValues(t, time.Time{}, fromDateHack(toDateHack(time.Time{})))
}

func TestConsentAuthenticationConverter(t *testing.T) {
	a := &consent.AuthenticationRequest{
		OpenIDConnectContext: &consent.OpenIDConnectContext{
			ACRValues:         []string{"1", "2"},
			UILocales:         []string{"fr", "de"},
			LoginHint:         "popup",
			IDTokenHintClaims: map[string]interface{}{"foo": "bar"},
			Display:           "popup",
		},
		AuthenticatedAt: time.Now().UTC().Add(-time.Minute),
		RequestedAt:     time.Now().UTC().Add(-time.Hour),
		Client:          &client.Client{ClientID: "client"},
		Subject:         "subject",
		RequestURL:      "https://request-url/path",
		Skip:            true,
		Challenge:       "challenge",
		RequestedScope:  []string{"scopea", "scopeb"},
		Verifier:        "verifier",
		CSRF:            "csrf",
	}

	b := &consent.HandledAuthenticationRequest{
		AuthenticationRequest: a,
		RememberFor:           120,
		Remember:              true,
		Challenge:             "challenge",
		RequestedAt:           time.Now().UTC().Add(-time.Minute),
		AuthenticatedAt:       time.Now().UTC().Add(-time.Minute),
		Error: &consent.RequestDeniedError{
			Name:        "error_name",
			Description: "error_description",
			Hint:        "error_hint,omitempty",
			Code:        100,
			Debug:       "error_debug,omitempty",
		},
		Subject:                "subject2",
		ForceSubjectIdentifier: "foo-id",
		ACR:     "acr",
		WasUsed: true,
	}

	a1, err := authenticationDataFromRequest(a)
	require.NoError(t, err)

	b1, err := handledAuthenticationRequest(b)
	require.NoError(t, err)

	a2, err := a1.toAuthenticationRequest(a.Client)
	require.NoError(t, err)
	assert.EqualValues(t, a, a2)

	b2, err := b1.toHandledAuthenticationRequest(a)
	require.NoError(t, err)
	assert.EqualValues(t, b, b2)
	assert.EqualValues(t, b.Subject, b2.Subject)
}

func TestSQLConsentConverter(t *testing.T) {
	a := &consent.ConsentRequest{
		OpenIDConnectContext: &consent.OpenIDConnectContext{
			ACRValues:         []string{"1", "2"},
			UILocales:         []string{"fr", "de"},
			Display:           "popup",
			LoginHint:         "popup",
			IDTokenHintClaims: map[string]interface{}{"foo": "bar"},
		},
		ForceSubjectIdentifier: "foo-id",
		RequestedAt:            time.Now().UTC().Add(-time.Hour),
		Client:                 &client.Client{ClientID: "client"},
		Subject:                "subject",
		RequestURL:             "https://request-url/path",
		Skip:                   true,
		Challenge:              "challenge",
		RequestedScope:         []string{"scopea", "scopeb"},
		Verifier:               "verifier",
		CSRF:                   "csrf",
		AuthenticatedAt:        time.Now().UTC().Add(-time.Minute),
	}

	b := &consent.HandledConsentRequest{
		ConsentRequest:  a,
		RememberFor:     10,
		Remember:        true,
		GrantedScope:    []string{"asdf", "fdsa"},
		AuthenticatedAt: time.Now().UTC().Add(-time.Minute),
		Challenge:       "challenge",
		Session: &consent.ConsentRequestSessionData{
			AccessToken: map[string]interface{}{"asdf": "fdsa"},
			IDToken:     map[string]interface{}{"foo": "fab"},
		},
		RequestedAt: time.Now().UTC().Add(-time.Minute),
		Error: &consent.RequestDeniedError{
			Name:        "error_name",
			Description: "error_description",
			Hint:        "error_hint,omitempty",
			Code:        100,
			Debug:       "error_debug,omitempty",
		},
	}

	a1, err := consentDataFromRequest(a)
	require.NoError(t, err)

	b1, err := handledConsentRequest(b)
	require.NoError(t, err)

	a2, err := a1.toConsentRequest(a.Client)
	require.NoError(t, err)
	assert.EqualValues(t, a, a2)

	b2, err := b1.toHandledConsentRequest(a)
	require.NoError(t, err)
	assert.EqualValues(t, b, b2)
}
