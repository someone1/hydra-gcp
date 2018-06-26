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
	"github.com/ory/hydra/consent"
	"github.com/ory/sqlcon"

	dconfig "github.com/someone1/hydra-gcp/config"
	dconsent "github.com/someone1/hydra-gcp/consent"
)

func injectConsentManager(c *config.Config, cm client.Manager) {
	var ctx = c.Context()
	var manager consent.Manager

	switch con := ctx.Connection.(type) {
	case *config.MemoryConnection:
		manager = consent.NewMemoryManager()
		break
	case *sqlcon.SQLConnection:
		manager = consent.NewSQLManager(con.GetDatabase(), cm)
		break
	case *config.PluginConnection:
		var err error
		if manager, err = con.NewConsentManager(); err != nil {
			c.GetLogger().Fatalf("Could not load client manager plugin %s", err)
		}
		break
	case *dconfig.DatastoreConnection:
		manager = dconsent.NewDatastoreManager(con.Context(), con.Client(), con.Namespace(), cm)
	default:
		panic("Unknown connection type.")
	}

	ctx.ConsentManager = manager

}

func newConsentHandler(c *config.Config, router *httprouter.Router) *consent.Handler {
	ctx := c.Context()

	w := herodot.NewJSONWriter(c.GetLogger())
	w.ErrorEnhancer = writerErrorEnhancer

	h := &consent.Handler{
		H: w,
		M: ctx.ConsentManager,
	}

	h.SetRoutes(router)
	return h
}
