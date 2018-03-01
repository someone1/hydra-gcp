// Copyright © 2017 Aeneas Rekkas <aeneas+oss@aeneas.io>
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

package hydragcp

import (
	"github.com/julienschmidt/httprouter"
	"github.com/ory/herodot"
	"github.com/ory/hydra/client"
	"github.com/ory/hydra/config"

	dclient "github.com/someone1/hydra-gcp/client"
	dconfig "github.com/someone1/hydra-gcp/config"
)

func newClientManager(c *config.Config) client.Manager {
	ctx := c.Context()

	switch con := ctx.Connection.(type) {
	case *config.MemoryConnection:
		return client.NewMemoryManager(ctx.Hasher)
	case *config.SQLConnection:
		return &client.SQLManager{
			DB:     con.GetDatabase(),
			Hasher: ctx.Hasher,
		}
	case *config.PluginConnection:
		if m, err := con.NewClientManager(); err != nil {
			c.GetLogger().Fatalf("Could not load client manager plugin %s", err)
		} else {
			return m
		}
		break
	case *dconfig.DatastoreConnection:
		return dclient.NewDatastoreManager(con.Context(), con.Client(), con.Namespace(), ctx.Hasher)
	default:
		panic("Unknown connection type.")
	}
	return nil
}

func newClientHandler(c *config.Config, router *httprouter.Router, manager client.Manager) *client.Handler {
	ctx := c.Context()
	h := &client.Handler{
		H: herodot.NewJSONWriter(c.GetLogger()),
		W: ctx.Warden, Manager: manager,
		ResourcePrefix: c.AccessControlResourcePrefix,
	}

	h.SetRoutes(router)
	return h
}
