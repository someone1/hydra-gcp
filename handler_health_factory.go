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
	"github.com/ory/hydra/config"
	"github.com/ory/hydra/health"
	"github.com/ory/sqlcon"

	dconfig "github.com/someone1/hydra-gcp/config"
)

func newHealthHandler(c *config.Config, router *httprouter.Router) *health.Handler {
	ctx := c.Context()
	var rc health.ReadyChecker

	switch con := ctx.Connection.(type) {
	case *config.MemoryConnection:
		rc = func() error {
			return nil
		}
		break
	case *sqlcon.SQLConnection:
		rc = func() error {
			return con.GetDatabase().Ping()
		}
		break
	case *config.PluginConnection:
		rc = func() error {
			return con.Ping()
		}
		break
	case *dconfig.DatastoreConnection:
		rc = func() error {
			return nil
		}
	default:
		panic("Unknown connection type.")
	}

	w := herodot.NewJSONWriter(c.GetLogger())
	w.ErrorEnhancer = writerErrorEnhancer

	h := &health.Handler{
		H:             w,
		VersionString: c.BuildVersion,
		ReadyChecks: map[string]health.ReadyChecker{
			"database": rc,
		},
	}

	h.SetRoutes(router)
	return h
}
