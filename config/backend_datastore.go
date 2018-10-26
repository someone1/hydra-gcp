// Copyright © 2018 Prateek Malhotra <someone1@gmail.com>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// Based on https://github.com/ory/hydra/blob/master/config/backend_sql.go

package config

import (
	"context"
	"net/url"
	"os"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/ory/fosite"
	"github.com/ory/hydra/client"
	"github.com/ory/hydra/config"
	"github.com/ory/hydra/consent"
	"github.com/ory/hydra/jwk"
	"github.com/ory/hydra/pkg"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"google.golang.org/api/option"

	dclient "github.com/someone1/hydra-gcp/client"
	dconsent "github.com/someone1/hydra-gcp/consent"
	djwk "github.com/someone1/hydra-gcp/jwk"
	"github.com/someone1/hydra-gcp/oauth2"
)

const (
	datastoreScheme = "datastore"
)

// Datastore URLs should be in the format of datastore://<projectid>?namespace=&credentialsFile=
// Just using datastore:// will be sufficient if running on GCP wiith an DATASTORE_PROJECT_ID env var set

// DatastoreConnection enables the use of Google's Datastore as a backend.
type DatastoreConnection struct {
	client *datastore.Client
	url    *url.URL
	l      logrus.FieldLogger
}

// Namespace will return the configured namespace for this backend, if any.
func (d *DatastoreConnection) Namespace() string {
	return d.url.Query().Get("namespace")
}

func (d *DatastoreConnection) Init(urlStr string, l logrus.FieldLogger, _ ...config.ConnectorOptions) error {
	ctx := context.Background()

	URL, err := url.Parse(urlStr)
	if err != nil {
		return err
	}

	d.url = URL
	d.l = l

	var opts []option.ClientOption
	if d.url.Scheme != datastoreScheme {
		return errors.New("incorrect scheme provided in URL")
	}
	urlOpts := d.url.Query()
	emulated := os.Getenv("DATASTORE_EMULATOR_HOST")
	if urlOpts.Get("credentialsFile") != "" && emulated == "" {
		opts = append(opts, option.WithCredentialsFile(urlOpts.Get("credentialsFile")))
	}

	if d.client, err = datastore.NewClient(ctx, d.url.Host, opts...); err != nil {
		return errors.Wrap(err, "Could not Connect to Datastore")
	}
	return nil
}

func (d *DatastoreConnection) NewConsentManager(clientManager client.Manager, fs pkg.FositeStorer) consent.Manager {
	return dconsent.NewDatastoreManager(d.client, d.Namespace(), clientManager, fs)
}

func (d *DatastoreConnection) NewOAuth2Manager(clientManager client.Manager, accessTokenLifespan time.Duration, _ string) pkg.FositeStorer {
	return oauth2.NewFositeDatastoreStore(clientManager, d.client, d.Namespace(), d.l, accessTokenLifespan)
}

func (d *DatastoreConnection) NewClientManager(hasher fosite.Hasher) client.Manager {
	return dclient.NewDatastoreManager(d.client, d.Namespace(), hasher)
}

func (d *DatastoreConnection) NewJWKManager(cipher *jwk.AEAD) jwk.Manager {
	return djwk.NewDatastoreManager(d.client, d.Namespace(), cipher)
}

func (d *DatastoreConnection) Prefixes() []string {
	return []string{datastoreScheme}
}

func (d *DatastoreConnection) Ping() error {
	return nil
}
