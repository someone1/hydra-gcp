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
	nctx "context"
	"fmt"
	"net/http"

	"github.com/ory/hydra/jwk"
	"github.com/someone1/gcp-jwt-go"

	"github.com/julienschmidt/httprouter"
	"github.com/ory/herodot"
	"github.com/ory/hydra/client"
	"github.com/ory/hydra/config"
	"github.com/ory/hydra/oauth2"
	"github.com/ory/hydra/policy"
	"github.com/ory/hydra/warden"
	"github.com/ory/hydra/warden/group"
	"github.com/ory/ladon"
	"github.com/pkg/errors"
)

type Handler struct {
	Clients *client.Handler
	OAuth2  *oauth2.Handler
	Consent *oauth2.ConsentSessionHandler
	Policy  *policy.Handler
	Groups  *group.Handler
	Warden  *warden.WardenHandler
	Config  *config.Config
	H       herodot.Writer
}

func (h *Handler) registerRoutes(ctxx nctx.Context, router *httprouter.Router) {
	c := h.Config
	ctx := c.Context()
	jwtConfig, ok := gcp_jwt.FromContextJWT(ctxx)
	if !ok {
		panic("must send a context with a IAMSignJWTConfig embedded")
	}

	// Set up dependencies
	injectConsentManager(c)
	clientsManager := newClientManager(c)
	injectFositeStore(c, clientsManager)
	oauth2Provider := newOAuth2Provider(ctxx, c)

	// set up warden
	ctx.Warden = &warden.LocalWarden{
		Warden: &ladon.Ladon{
			Manager: ctx.LadonManager,
		},
		OAuth2:              oauth2Provider,
		Issuer:              c.Issuer,
		AccessTokenLifespan: c.GetAccessTokenLifespan(),
		Groups:              ctx.GroupManager,
		L:                   c.GetLogger(),
	}

	// Set up handlers
	h.Clients = newClientHandler(c, router, clientsManager)
	h.Policy = newPolicyHandler(c, router)
	h.Consent = newConsentHanlder(c, router)
	h.OAuth2 = newOAuth2Handler(c, router, ctx.ConsentManager, oauth2Provider)
	h.Warden = warden.NewHandler(c, router)
	h.Groups = &group.Handler{
		H:              herodot.NewJSONWriter(c.GetLogger()),
		W:              ctx.Warden,
		Manager:        ctx.GroupManager,
		ResourcePrefix: c.AccessControlResourcePrefix,
	}
	h.Groups.SetRoutes(router)
	_ = newHealthHandler(c, router)

	// JWK is handled by Google
	router.GET(jwk.WellKnownKeysPath, func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		http.Redirect(w, r, fmt.Sprintf("https://www.googleapis.com/service_accounts/v1/jwk/%s", jwtConfig.ServiceAccount), http.StatusTemporaryRedirect)
	})

	h.createRootIfNewInstall(c)
}

func (h *Handler) rejectInsecureRequests(rw http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	if r.TLS != nil || h.Config.ForceHTTP {
		next.ServeHTTP(rw, r)
		return
	}

	if err := h.Config.DoesRequestSatisfyTermination(r); err == nil {
		next.ServeHTTP(rw, r)
		return
	} else {
		h.Config.GetLogger().WithError(err).Warnln("Could not serve http connection")
	}

	h.H.WriteErrorCode(rw, r, http.StatusBadGateway, errors.New("Can not serve request over insecure http"))
}
