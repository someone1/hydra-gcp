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

	"cloud.google.com/go/datastore"
	"github.com/sirupsen/logrus"
	"google.golang.org/api/option"
)

// Datastore URLs should be in the format of datastore://<projectid>?namespace=&credentialsFile=
// Just using datastore:// will be sufficient if running on GCP wiith an DATASTORE_PROJECT_ID env var set

// DatastoreConnection enables the use of Google's Datastore as a backend.
type DatastoreConnection struct {
	client  *datastore.Client
	context context.Context
	url     *url.URL
	l       logrus.FieldLogger
}

// NewDatastoreConnection initializes and returns a DatastoreConnection
func NewDatastoreConnection(ctx context.Context, URL *url.URL, l logrus.FieldLogger) *DatastoreConnection {
	return &DatastoreConnection{
		context: ctx,
		url:     URL,
		l:       l,
	}
}

// Client will initialize and return a datastore.Client on first call and return the same datastore.Client on subseqeunt calls.
func (d *DatastoreConnection) Client() *datastore.Client {
	if d.client != nil {
		return d.client
	}
	var err error
	var opts []option.ClientOption
	urlOpts := d.url.Query()
	if urlOpts.Get("credentialsFile") != "" {
		opts = append(opts, option.WithCredentialsFile(urlOpts.Get("credentialsFile")))
	}

	if d.client, err = datastore.NewClient(d.context, d.url.Host, opts...); err != nil {
		d.l.Fatalf("Could not Connect to Datastore: %s", err)
	}

	return d.client
}

// Namespace will return the configured namespace for this backend, if any.
func (d *DatastoreConnection) Namespace() string {
	return d.url.Query().Get("namespace")
}

// Context will return the context.Context used to create this DatastoreConnection
func (d *DatastoreConnection) Context() context.Context {
	return d.context
}
