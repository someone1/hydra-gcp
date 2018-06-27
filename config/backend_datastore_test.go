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
	"testing"

	"github.com/sirupsen/logrus"
)

func mustParseURL(t *testing.T, urlStr string) *url.URL {
	t.Helper()
	u, err := url.Parse(urlStr)
	if err != nil {
		t.Fatalf("could not parse database url: %v", err)
		return nil
	}
	return u
}

func TestNewDatastoreConnection(t *testing.T) {
	validURL := mustParseURL(t, "datastore://project?namespace=namespace")
	type args struct {
		ctx context.Context
		URL *url.URL
		l   logrus.FieldLogger
	}
	type fields struct {
		namespace string
		ctx       context.Context
	}
	tests := []struct {
		name    string
		args    args
		fields  fields
		wantErr bool
	}{
		{
			"invalid",
			args{
				context.Background(),
				&url.URL{Scheme: "invalid"},
				nil,
			},
			fields{
				"",
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
			fields{
				"namespace",
				context.Background(),
			},
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			con, err := NewDatastoreConnection(tt.args.ctx, tt.args.URL, tt.args.l)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewDatastoreConnection() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				if want := con.Namespace(); want != tt.fields.namespace {
					t.Errorf("DatastoreConnection.Namespace() = %s, want %s", want, tt.fields.namespace)
					return
				}
				if want := con.Context(); want != tt.fields.ctx {
					t.Errorf("DatastoreConnection.Context() = %s, want %s", want, tt.fields.ctx)
					return
				}
				if want := con.l; want != tt.args.l {
					t.Errorf("DatastoreConnection.l = %v, want %v", want, tt.args.l)
					return
				}
				if want := con.url.String(); want != tt.args.URL.String() {
					t.Errorf("DatastoreConnection.url = %s, want %s", want, tt.args.URL)
					return
				}
			}
		})
	}
}
