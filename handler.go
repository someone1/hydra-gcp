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

	"github.com/julienschmidt/httprouter"
	"github.com/ory/herodot"
	"github.com/ory/hydra/client"
	"github.com/ory/hydra/config"
	"github.com/ory/hydra/consent"
	"github.com/ory/hydra/jwk"
	"github.com/ory/hydra/oauth2"
	"github.com/pkg/errors"

	"github.com/someone1/gcp-jwt-go"
)

type Handler struct {
	Clients *client.Handler
	OAuth2  *oauth2.Handler
	Consent *consent.Handler
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
	clientsManager := newClientManager(c)
	injectConsentManager(c, clientsManager)
	injectFositeStore(c, clientsManager)
	oauth2Provider := newOAuth2Provider(ctxx, c)

	// Set up handlers
	h.Clients = newClientHandler(c, router, clientsManager)
	h.Consent = newConsentHandler(c, router)
	h.OAuth2 = newOAuth2Handler(ctxx, c, router, ctx.ConsentManager, oauth2Provider)
	_ = newHealthHandler(c, router)

	// JWK is handled by Google
	router.GET(jwk.WellKnownKeysPath, func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		http.Redirect(w, r, fmt.Sprintf("https://www.googleapis.com/service_accounts/v1/jwk/%s", jwtConfig.ServiceAccount), http.StatusTemporaryRedirect)
	})
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
