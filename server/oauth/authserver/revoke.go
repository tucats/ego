package authserver

import (
	"net/http"
	"strings"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/i18n"
	"github.com/tucats/ego/router"
	"github.com/tucats/ego/tokens"
	"github.com/tucats/ego/util"
)

// RevokeHandler handles POST /oauth2/revoke (RFC 7009).
//
// Clients post a token here to invalidate it.  The handler extracts the JWT,
// parses the "jti" (JWT ID) claim, and adds the JTI to Ego's token blacklist
// so that no subsequent request using that token will be accepted.
//
// Per RFC 7009 §2.2, the server always responds 200 OK regardless of whether
// the token existed — this prevents information leakage about which tokens are
// valid.
func RevokeHandler(session *router.Session, w http.ResponseWriter, r *http.Request) int {
	if err := r.ParseForm(); err != nil {
		return util.ErrorResponse(w, session.ID,
			i18n.Text(session.Language, "error.oauth.as.body.parse"), http.StatusBadRequest)
	}

	// The client must authenticate itself to revoke a token.
	//
	// OAUTH-M3: use validateBasicAuth so that confidential clients can supply
	// their credentials in the standard HTTP Basic Authorization header as well
	// as in the form body.  RFC 7009 §2.1 requires the revocation endpoint to
	// support the same client-authentication mechanism as the token endpoint.
	//
	// validateBasicAuth (defined in token.go) checks the Authorization header
	// first; if no Basic Auth header is present it falls back to the
	// "client_id" / "client_secret" form fields.  This keeps backward
	// compatibility with clients that post credentials in the body.
	clientID, clientSecret := validateBasicAuth(r)

	if clientID == "" {
		return util.ErrorResponse(w, session.ID,
			i18n.Text(session.Language, "error.oauth.as.missing.client_id"), http.StatusBadRequest)
	}

	client := findClient(clientID)
	if client == nil || !validateClientSecret(client, clientSecret) {
		return util.ErrorResponse(w, session.ID,
			i18n.Text(session.Language, "error.oauth.as.invalid.client"), http.StatusUnauthorized)
	}

	tokenStr := r.FormValue("token")
	if tokenStr == "" {
		// RFC 7009 says to respond 200 even if the token parameter is missing,
		// but logging a warning is helpful for debugging.
		w.WriteHeader(http.StatusOK)

		return http.StatusOK
	}

	// Determine if this looks like a refresh token (opaque) or an access token (JWT).
	// Refresh tokens are stored in OAuthRefreshCache, not as JWTs.
	if !strings.Contains(tokenStr, ".") {
		// No dots → not a JWT → treat as a refresh token.  Consuming it is
		// sufficient to revoke it (single-use semantics already apply).
		consumeRefreshToken(tokenStr)

		w.WriteHeader(http.StatusOK)

		return http.StatusOK
	}

	// Parse the JWT to extract the JTI (JWT ID) — without the JTI we cannot
	// blacklist the token.  We do NOT return an error if parsing fails because
	// that would reveal whether the token was valid.
	claims, err := parseToken(tokenStr)
	if err != nil {
		// Token is already invalid (expired, bad signature, etc.); nothing to do.
		w.WriteHeader(http.StatusOK)

		return http.StatusOK
	}

	jti := claims.ID
	sub := claims.Subject

	if jti != "" {
		// tokens.Blacklist persists the JTI so that future requests carrying
		// this token are rejected even after a server restart.
		if blacklistErr := tokens.Blacklist(jti); blacklistErr != nil {
			ui.Log(ui.ServerLogger, "server.error", ui.A{
				"error": blacklistErr.Error(),
			})
		} else {
			ui.Log(ui.ServerLogger, "oauth.as.token.revoked", ui.A{
				"jti":  jti,
				"user": sub,
			})
		}
	}

	// RFC 7009 §2.2 mandates a 200 OK response for any revocation attempt,
	// even when the token was not found or had already expired.
	w.WriteHeader(http.StatusOK)

	return http.StatusOK
}
