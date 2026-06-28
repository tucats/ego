package authserver

import (
	"net/http"

	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/i18n"
	"github.com/tucats/ego/internal/router"
	"github.com/tucats/ego/internal/util"
)

// JWKSHandler serves the JSON Web Key Set at GET /.well-known/jwks.json.
//
// Resource Servers fetch this endpoint to obtain the AS's public signing key
// so they can verify the signature on JWT access tokens without contacting the
// AS for every request.
//
// The response body is the pre-computed jwksJSON byte slice that was built once
// from signingKey at startup.  Serving a static blob avoids repeated marshaling
// and keeps latency to a minimum.
func JWKSHandler(session *router.Session, w http.ResponseWriter, r *http.Request) int {
	if jwksJSON == nil {
		return util.ErrorResponse(w, session.ID,
			i18n.Text(session.Language, "error.oauth.as.key.not.initialized"), http.StatusServiceUnavailable)
	}

	w.Header().Set(defs.ContentTypeHeader, "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(jwksJSON)

	return http.StatusOK
}
