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
	"reflect"
	"testing"

	"cloud.google.com/go/datastore"
	"github.com/sirupsen/logrus"
	"google.golang.org/api/option"
)

func TestDatastoreConnection_Client(t *testing.T) {
	type fields struct {
		client  *datastore.Client
		context context.Context
		url     *url.URL
		l       logrus.FieldLogger
	}
	tests := []struct {
		name   string
		fields fields
		want   *datastore.Client
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &DatastoreConnection{
				client:  tt.fields.client,
				context: tt.fields.context,
				url:     tt.fields.url,
				l:       tt.fields.l,
			}
			if got := d.Client(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DatastoreConnection.Client() = %v, want %v", got, tt.want)
			}
		})
	}
}

func mustParseURL(t *testing.T, urlStr string) *url.URL {
	t.Helper()
	u, err := url.Parse(urlStr)
	if err != nil {
		t.Fatalf("could not parse database url: %v", err)
		return nil
	}
	return u
}

func mustNewDatastoreClient(ctx context.Context, t *testing.T, projectID string, opts ...option.ClientOption) *datastore.Client {
	client, err := datastore.NewClient(ctx, projectID, opts...)
	if err != nil {
		t.Fatalf("could not create datastore.Client: %v", client)
	}
	return client
}

func TestNewDatastoreConnection(t *testing.T) {
	validURL := mustParseURL(t, "datastore://project?namespace=namspace")
	type args struct {
		ctx context.Context
		URL *url.URL
		l   logrus.FieldLogger
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			"invalid",
			args{
				context.Background(),
				&url.URL{Scheme: "invalid"},
				nil,
			},
			true,
		},
		{
			"valid",
			args{
				context.Background(),
				validURL,
				nil,
			},
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewDatastoreConnection(tt.args.ctx, tt.args.URL, tt.args.l)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewDatastoreConnection() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}
