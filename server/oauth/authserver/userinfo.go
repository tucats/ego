package authserver

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/i18n"
	"github.com/tucats/ego/router"
	"github.com/tucats/ego/tokens"
	"github.com/tucats/ego/util"
)

// UserinfoResponse is the JSON body returned from GET /oauth2/userinfo.
// It follows the OIDC Core §5.3 UserInfo Response format.
type UserinfoResponse struct {
	// Sub is the subject identifier — the Ego username of the authenticated user.
	Sub string `json:"sub"`

	// Name is the user's display name. Ego uses the username.
	Name string `json:"name,omitempty"`

	// Email is the user's email address. Ego uses the username as a stand-in.
	Email string `json:"email,omitempty"`

	// UpdatedAt is intentionally omitted — Ego has no profile update timestamps.
}

// UserinfoHandler handles GET /oauth2/userinfo.
//
// The request must include a valid Bearer token in the Authorization header.
// The handler validates the token, confirms the "openid" scope is present,
// and returns the identity claims for the authenticated subject.
//
// This endpoint does NOT return any claims that require database lookups beyond
// the username — Ego has no user-profile database.
func UserinfoHandler(session *router.Session, w http.ResponseWriter, r *http.Request) int {
	// Extract the Bearer token from the Authorization header.
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
		w.Header().Set("WWW-Authenticate", `Bearer realm="Ego OAuth2 AS"`)

		return util.ErrorResponse(w, session.ID,
			i18n.TLang(session.Language, "error.oauth.as.missing.bearer"), http.StatusUnauthorized)
	}

	tokenString := strings.TrimPrefix(authHeader, "Bearer ")

	claims, err := parseToken(tokenString)
	if err != nil {
		w.Header().Set("WWW-Authenticate", `Bearer realm="Ego OAuth2 AS", error="invalid_token"`)

		return util.ErrorResponse(w, session.ID,
			i18n.TLang(session.Language, "error.oauth.as.invalid.code"), http.StatusUnauthorized)
	}

	// Reject tokens that have been explicitly revoked via POST /oauth2/revoke.
	// This check is critical for the revocation endpoint to be effective: without
	// it a revoked token would still return UserInfo claims until expiry.
	if claims.ID != "" {
		if blacklisted, blErr := tokens.IsIDBlacklisted(claims.ID); blErr == nil && blacklisted {
			w.Header().Set("WWW-Authenticate", `Bearer realm="Ego OAuth2 AS", error="invalid_token"`)

			return util.ErrorResponse(w, session.ID,
				i18n.TLang(session.Language, "error.oauth.as.token.revoked.error"), http.StatusUnauthorized)
		}
	}

	// The UserInfo endpoint is only meaningful for user tokens.
	// client_credentials tokens have an empty "sub" and should not reach here.
	if claims.Subject == "" {
		return util.ErrorResponse(w, session.ID,
			i18n.TLang(session.Language, "error.oauth.as.no.subject"), http.StatusForbidden)
	}

	resp := UserinfoResponse{
		Sub: claims.Subject,
	}

	// Include name and email only when the corresponding scopes were granted.
	if strings.Contains(claims.Scope, "profile") {
		resp.Name = claims.Subject
	}

	if strings.Contains(claims.Scope, "email") {
		resp.Email = claims.Subject
	}

	b, err := json.Marshal(resp)
	if err != nil {
		return util.ErrorResponse(w, session.ID,
			i18n.TLang(session.Language, "error.oauth.as.userinfo.serialize"), http.StatusInternalServerError)
	}

	w.Header().Set(defs.ContentTypeHeader, defs.JSONMediaType)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(b)

	return http.StatusOK
}
