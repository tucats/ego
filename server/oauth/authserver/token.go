package authserver

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/i18n"
	"github.com/tucats/ego/router"
	"github.com/tucats/ego/server/auth"
	"github.com/tucats/ego/util"
)

// asGlobalConfig holds the resolved configuration loaded at startup.
// It is accessible to all handlers in the package without being re-read on
// every request.
var asGlobalConfig asConfig

// TokenHandler processes POST /oauth2/token requests.
//
// It dispatches to the appropriate sub-handler based on the "grant_type" form
// parameter:
//   - "authorization_code" — exchange a code for tokens
//   - "client_credentials" — server-to-server token without user login
//   - "refresh_token"      — obtain a new access token from a refresh token
func TokenHandler(session *router.Session, w http.ResponseWriter, r *http.Request) int {
	if err := r.ParseForm(); err != nil {
		return util.ErrorResponse(w, session.ID,
			"could not parse request body", http.StatusBadRequest)
	}

	grantType := r.FormValue("grant_type")

	switch grantType {
	case "authorization_code":
		return handleAuthorizationCodeGrant(session, w, r)
	case "client_credentials":
		return handleClientCredentialsGrant(session, w, r)
	case "refresh_token":
		return handleRefreshTokenGrant(session, w, r)
	default:
		return util.ErrorResponse(w, session.ID,
			i18n.T("oauth.as.invalid.grant"), http.StatusBadRequest)
	}
}

// handleAuthorizationCodeGrant exchanges a short-lived authorization code for
// an access token (and optionally an ID token and refresh token).
//
// Required form fields: grant_type, code, redirect_uri, client_id
// Optional: client_secret (for confidential clients), code_verifier (for PKCE)
func handleAuthorizationCodeGrant(session *router.Session, w http.ResponseWriter, r *http.Request) int {
	clientID := r.FormValue("client_id")
	clientSecret := r.FormValue("client_secret")
	code := r.FormValue("code")
	redirectURI := r.FormValue("redirect_uri")
	codeVerifier := r.FormValue("code_verifier")

	client := findClient(clientID)
	if client == nil || !validateClientSecret(client, clientSecret) {
		return util.ErrorResponse(w, session.ID,
			i18n.T("oauth.as.invalid.client"), http.StatusUnauthorized)
	}

	if !clientAllowsGrant(client, "authorization_code") {
		return util.ErrorResponse(w, session.ID,
			i18n.T("oauth.as.invalid.grant"), http.StatusBadRequest)
	}

	pending, found := consumeCode(code)
	if !found {
		return util.ErrorResponse(w, session.ID,
			i18n.T("oauth.as.invalid.code"), http.StatusBadRequest)
	}

	// The redirect_uri in the token request must match the one used in the
	// authorization request (RFC 6749 §4.1.3).
	if pending.RedirectURI != redirectURI {
		return util.ErrorResponse(w, session.ID,
			i18n.T("oauth.as.invalid.redirect"), http.StatusBadRequest)
	}

	// Verify PKCE if the authorization request included a code_challenge.
	if err := verifyPKCE(pending, codeVerifier); err != nil {
		return util.ErrorResponse(w, session.ID,
			i18n.T("oauth.as.invalid.pkce"), http.StatusBadRequest)
	}

	scope := strings.Join(pending.Scopes, " ")
	accessToken, _, err := createAccessToken(asGlobalConfig, clientID, pending.Username, clientID, scope)

	if err != nil {
		return util.ErrorResponse(w, session.ID,
			"could not create access token", http.StatusInternalServerError)
	}

	resp := tokenResponse{
		AccessToken: accessToken,
		TokenType:   "Bearer",
		ExpiresIn:   int64(asGlobalConfig.TokenExpiration.Seconds()),
		Scope:       scope,
	}

	// Issue an ID token when the "openid" scope was granted.
	if strings.Contains(scope, "openid") {
		idToken, idErr := createIDToken(asGlobalConfig, clientID, pending.Username)
		if idErr == nil {
			resp.IDToken = idToken
		}
	}

	// Issue a refresh token when the client supports it.
	if clientAllowsGrant(client, "refresh_token") {
		refreshToken, refreshErr := generateRefreshToken(clientID, pending.Username, pending.Scopes)
		if refreshErr == nil {
			resp.RefreshToken = refreshToken
		}
	}

	return writeTokenResponse(w, session.ID, resp)
}

// handleClientCredentialsGrant issues an access token directly to a registered
// client based on its credentials, without any user login.
//
// This grant is used for server-to-server (machine-to-machine) access.
// Required form fields: grant_type, client_id, client_secret, scope
func handleClientCredentialsGrant(session *router.Session, w http.ResponseWriter, r *http.Request) int {
	clientID := r.FormValue("client_id")
	clientSecret := r.FormValue("client_secret")
	scopeStr := r.FormValue("scope")

	client := findClient(clientID)
	if client == nil || !validateClientSecret(client, clientSecret) {
		return util.ErrorResponse(w, session.ID,
			i18n.T("oauth.as.invalid.client"), http.StatusUnauthorized)
	}

	if !clientAllowsGrant(client, "client_credentials") {
		return util.ErrorResponse(w, session.ID,
			i18n.T("oauth.as.invalid.grant"), http.StatusBadRequest)
	}

	// Validate requested scopes against the client's allowed scopes.
	requested := splitScope(scopeStr)
	if len(requested) > 0 && !clientAllowsScope(client, requested) {
		return util.ErrorResponse(w, session.ID,
			i18n.T("oauth.as.invalid.scope"), http.StatusBadRequest)
	}

	// If no scope was requested, grant all of the client's allowed scopes.
	if len(requested) == 0 {
		requested = client.Scopes
	}

	scope := strings.Join(requested, " ")

	// For client_credentials the "sub" (subject) is empty — there is no user.
	accessToken, _, err := createAccessToken(asGlobalConfig, clientID, "", clientID, scope)
	if err != nil {
		return util.ErrorResponse(w, session.ID,
			"could not create access token", http.StatusInternalServerError)
	}

	ui.Log(ui.ServerLogger, "oauth.as.client.credentials.issued", ui.A{
		"client": clientID,
		"scope":  scope,
	})

	return writeTokenResponse(w, session.ID, tokenResponse{
		AccessToken: accessToken,
		TokenType:   "Bearer",
		ExpiresIn:   int64(asGlobalConfig.TokenExpiration.Seconds()),
		Scope:       scope,
	})
}

// handleRefreshTokenGrant exchanges a refresh token for a new access token.
// The old refresh token is consumed and a new one is issued (token rotation).
//
// Required form fields: grant_type, refresh_token, client_id
// Optional: client_secret (for confidential clients), scope (may narrow)
func handleRefreshTokenGrant(session *router.Session, w http.ResponseWriter, r *http.Request) int {
	clientID := r.FormValue("client_id")
	clientSecret := r.FormValue("client_secret")
	refreshTokenStr := r.FormValue("refresh_token")

	client := findClient(clientID)
	if client == nil || !validateClientSecret(client, clientSecret) {
		return util.ErrorResponse(w, session.ID,
			i18n.T("oauth.as.invalid.client"), http.StatusUnauthorized)
	}

	if !clientAllowsGrant(client, "refresh_token") {
		return util.ErrorResponse(w, session.ID,
			i18n.T("oauth.as.invalid.grant"), http.StatusBadRequest)
	}

	data, found := consumeRefreshToken(refreshTokenStr)
	if !found {
		return util.ErrorResponse(w, session.ID,
			i18n.T("oauth.as.invalid.refresh"), http.StatusBadRequest)
	}

	// The refresh token must belong to the client presenting it.
	if data.ClientID != clientID {
		return util.ErrorResponse(w, session.ID,
			i18n.T("oauth.as.invalid.client"), http.StatusUnauthorized)
	}

	scope := strings.Join(data.Scopes, " ")

	accessToken, jti, err := createAccessToken(asGlobalConfig, clientID, data.Username, clientID, scope)
	if err != nil {
		return util.ErrorResponse(w, session.ID,
			"could not create access token", http.StatusInternalServerError)
	}

	ui.Log(ui.ServerLogger, "oauth.as.token.refreshed", ui.A{
		"client": clientID,
		"jti":    jti,
	})

	// Issue a new refresh token (rotation — the old one was deleted by consumeRefreshToken).
	newRefreshToken, refreshErr := generateRefreshToken(clientID, data.Username, data.Scopes)

	resp := tokenResponse{
		AccessToken: accessToken,
		TokenType:   "Bearer",
		ExpiresIn:   int64(asGlobalConfig.TokenExpiration.Seconds()),
		Scope:       scope,
	}

	if refreshErr == nil {
		resp.RefreshToken = newRefreshToken
	}

	// Re-issue an ID token when openid is in scope.
	if strings.Contains(scope, "openid") && data.Username != "" {
		idToken, idErr := createIDToken(asGlobalConfig, clientID, data.Username)
		if idErr == nil {
			resp.IDToken = idToken
		}
	}

	return writeTokenResponse(w, session.ID, resp)
}

// writeTokenResponse serializes a tokenResponse as JSON and writes it to w.
// Returns the HTTP status code (200 on success).
func writeTokenResponse(w http.ResponseWriter, sessionID int, resp tokenResponse) int {
	b, err := json.Marshal(resp)
	if err != nil {
		return util.ErrorResponse(w, sessionID,
			"could not serialize token response", http.StatusInternalServerError)
	}

	w.Header().Set(defs.ContentTypeHeader, defs.JSONMediaType)
	// RFC 6749 requires Cache-Control: no-store on token responses to prevent
	// browsers and proxies from caching tokens.
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(b)

	return http.StatusOK
}

// validateBasicAuth extracts and validates HTTP Basic Auth credentials from a
// token endpoint request.  Basic Auth is the preferred mechanism for confidential
// clients (RFC 6749 §2.3.1).  Returns the clientID and clientSecret, or empty
// strings if no Basic Auth header was present.
func validateBasicAuth(r *http.Request) (clientID, clientSecret string) {
	cid, csec, ok := r.BasicAuth()
	if ok {
		return cid, csec
	}

	// Fall back to form-encoded credentials if no Basic Auth header is present.
	return r.FormValue("client_id"), r.FormValue("client_secret")
}

// userScopesForPermissions converts an Ego user's permission list into the
// set of OAuth2 scopes they are entitled to.  This is the same mapping used
// in authorize.go / intersectScopes but expressed as a slice for use in the
// client_credentials flow where we build scopes from permissions.
func userScopesForPermissions(permissions []string) []string {
	scopes := map[string]bool{
		"openid":  true,
		"profile": true,
		"email":   true,
	}

	for _, p := range permissions {
		switch p {
		case defs.RootPermission:
			scopes["ego:admin"] = true
		case defs.TableWritePermission, defs.TableUpdatePermission, defs.TableDeletePermission, defs.TableAdminPermission:
			scopes["ego:write"] = true
		case defs.TableReadPermission, defs.LogonPermission:
			scopes["ego:read"] = true
		case defs.CodeRunPermission:
			scopes["ego:code"] = true
		}
	}

	result := make([]string, 0, len(scopes))
	for s := range scopes {
		result = append(result, s)
	}

	return result
}

// validatePassword wraps auth.ValidatePassword but accepts empty username/password,
// returning false immediately for blank values to avoid a needless DB lookup.
func validatePassword(sessionID int, username, password string) bool {
	if username == "" || password == "" {
		return false
	}

	return auth.ValidatePassword(sessionID, username, password)
}
