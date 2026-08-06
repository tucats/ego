package authserver

import (
	"encoding/json"
	"net/http"

	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/defs"
)

// oauthError is the error response shape RFC 6749 §5.2 defines for the token
// endpoint, and RFC 7009 §2.2.1 reuses for the revocation endpoint. This is
// deliberately not defs.RestStatusResponse: a conformant OAuth2/OIDC client
// library branches on the "error" field of exactly this shape (e.g. silently
// retrying on "invalid_grant", prompting re-login on "invalid_client"), and
// Ego's generic {server,status,msg} envelope has no such field at all, so a
// standard client cannot parse it as an OAuth2 error. The body deliberately
// carries no Ego-specific fields (no server info, no internal status
// duplicate) -- RFC clients expect exactly {error, error_description}.
type oauthError struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description,omitempty"`
}

// oauthErrorCode is one of the fixed enum values RFC 6749 §5.2 defines for
// the "error" field. Keeping this as a distinct type rather than a bare
// string stops a call site from passing an arbitrary, non-RFC value by
// accident -- the whole point of this shape is that a client can match on a
// known, finite set of values.
type oauthErrorCode string

const (
	// oauthInvalidRequest: the request is missing a required parameter,
	// includes an unsupported parameter value, or is otherwise malformed.
	oauthInvalidRequest oauthErrorCode = "invalid_request"

	// oauthInvalidClient: client authentication failed (unknown client, no
	// client authentication included, or unsupported authentication method).
	oauthInvalidClient oauthErrorCode = "invalid_client"

	// oauthInvalidGrant: the provided authorization grant (authorization
	// code, refresh token) is invalid, expired, revoked, does not match the
	// redirection URI used in the authorization request, or was issued to
	// another client. Also used for a PKCE code_verifier that does not match
	// the code_challenge (RFC 7636 §4.6).
	oauthInvalidGrant oauthErrorCode = "invalid_grant"

	// oauthUnauthorizedClient: the authenticated client is not authorized to
	// use this authorization grant type.
	oauthUnauthorizedClient oauthErrorCode = "unauthorized_client"

	// oauthUnsupportedGrantType: the authorization grant type is not
	// supported by this authorization server.
	oauthUnsupportedGrantType oauthErrorCode = "unsupported_grant_type"

	// oauthInvalidScope: the requested scope is invalid, unknown, malformed,
	// or exceeds the scope the client is registered for.
	oauthInvalidScope oauthErrorCode = "invalid_scope"

	// oauthServerError: the authorization server encountered an unexpected
	// condition. Not in §5.2's own list (that list only covers the client's
	// mistakes), but reused from §4.1.2.1's authorization-endpoint list --
	// the de facto convention most OAuth2 AS implementations follow for a
	// token-endpoint 500, since §5.2 does not otherwise define one.
	oauthServerError oauthErrorCode = "server_error"
)

// setClientAuthChallenge sets the WWW-Authenticate header RFC 7235 §3.1
// requires on a 401. The token and revocation endpoints authenticate the
// client via HTTP Basic (RFC 6749 §2.3.1), which is the scheme this
// advertises -- a different scheme from userinfo.go's Bearer challenge,
// which authenticates the resource owner's access token, not the client.
func setClientAuthChallenge(w http.ResponseWriter) {
	w.Header().Set(defs.AuthenticateHeader, `Basic realm="Ego OAuth2 AS"`)
}

// writeOAuthError sends an RFC 6749 §5.2-shaped error response and returns
// the status code. It is the OAuth AS's counterpart to util.ErrorResponse --
// authserver/token.go and revoke.go use this instead of util.ErrorResponse
// for every error response, so that a standard OAuth2/OIDC client library
// can parse them. Deliberately matches util.ErrorResponse's calling
// convention (sessionID, then the message, then status) so converting a call
// site is a mechanical, low-risk change.
//
// A caller that needs a WWW-Authenticate header on a 401 (RFC 7235 §3.1) must
// set it before calling this, the same way util.ErrorResponse callers already
// do elsewhere in this package (see userinfo.go) -- this function only
// decides the body shape, not the headers around it.
func writeOAuthError(w http.ResponseWriter, sessionID int, code oauthErrorCode, description string, status int) int {
	resp := oauthError{
		Error:            string(code),
		ErrorDescription: description,
	}

	b, _ := json.MarshalIndent(resp, ui.JSONIndentPrefix, ui.JSONIndentSpacer)

	w.Header().Set(defs.ContentTypeHeader, defs.JSONMediaType)
	w.WriteHeader(status)
	_, _ = w.Write(b)

	if ui.IsActive(ui.RestLogger) {
		ui.Log(ui.RestLogger, "rest.error", ui.A{
			"session": sessionID,
			"error":   description,
			"status":  status,
		})
		ui.WriteLog(ui.RestLogger, "rest.response.payload", ui.A{
			"session": sessionID,
			"body":    string(b),
		})
	}

	return status
}
