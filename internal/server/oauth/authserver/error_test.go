package authserver

// Tests for the REST-3 audit's OAuth AS section (docs/issues/REST-3.md,
// section 5): the RFC 6749 §5.2-shaped error writer (5.1), corrected RFC
// error-code categories (5.2), WWW-Authenticate on 401s (5.3), and
// authorize.go no longer answering 401 for an unknown client (5.4).

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/tucats/ego/internal/router"
)

// postToTokenRecorder is like postToToken (token_test.go) but returns the
// full recorder so the response body can be inspected, not just the status.
func postToTokenRecorder(t *testing.T, form url.Values) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/oauth2/token",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	w := httptest.NewRecorder()
	sess := &router.Session{ID: 99}

	TokenHandler(sess, w, req)

	return w
}

func decodeOAuthError(t *testing.T, w *httptest.ResponseRecorder) oauthError {
	t.Helper()

	var body oauthError
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("response body did not parse as the RFC 6749 {error, error_description} shape: %v\nbody: %s", err, w.Body.String())
	}

	if body.Error == "" {
		t.Fatalf("body has no \"error\" field -- want an RFC 6749 error code, got: %s", w.Body.String())
	}

	return body
}

func TestTokenHandler_UnsupportedGrantType_RFCShape(t *testing.T) {
	setupTokenTestKey(t)

	form := url.Values{"grant_type": {"not_a_real_grant_type"}}
	w := postToTokenRecorder(t, form)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}

	body := decodeOAuthError(t, w)
	if body.Error != string(oauthUnsupportedGrantType) {
		t.Errorf("error = %q, want %q", body.Error, oauthUnsupportedGrantType)
	}
}

func TestTokenHandler_InvalidClient_HasWWWAuthenticateChallenge(t *testing.T) {
	setupTokenTestKey(t)

	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {"whatever"},
		"client_id":     {"no-such-client"},
		"client_secret": {"wrong"},
	}
	w := postToTokenRecorder(t, form)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}

	body := decodeOAuthError(t, w)
	if body.Error != string(oauthInvalidClient) {
		t.Errorf("error = %q, want %q", body.Error, oauthInvalidClient)
	}

	if got := w.Header().Get("WWW-Authenticate"); !strings.Contains(got, "Basic") {
		t.Errorf("WWW-Authenticate = %q, want it to name the Basic scheme", got)
	}
}

func TestTokenHandler_UnrecognizedCode_IsInvalidGrantNotInvalidCode(t *testing.T) {
	setupTokenTestKey(t)
	setupPublicClient(t, "client1", "https://app.example.com/callback")

	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {"never-issued"},
		"client_id":     {"client1"},
		"redirect_uri":  {"https://app.example.com/callback"},
		"code_verifier": {"whatever"},
	}
	w := postToTokenRecorder(t, form)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}

	body := decodeOAuthError(t, w)
	if body.Error != string(oauthInvalidGrant) {
		t.Errorf("error = %q, want %q (RFC 6749 has no \"invalid_code\" value)", body.Error, oauthInvalidGrant)
	}
}

func TestTokenHandler_ClientNotAllowedGrantType_IsUnauthorizedClient(t *testing.T) {
	setupTokenTestKey(t)

	// Registered for client_credentials only -- authorization_code is not
	// in its GrantTypes list.
	clients = []OAuthClient{{
		ClientID:   "cc-only-client",
		GrantTypes: []string{"client_credentials"},
		Scopes:     []string{"openid"},
	}}
	
	t.Cleanup(func() { clients = nil })

	form := url.Values{
		"grant_type":   {"authorization_code"},
		"code":         {"whatever"},
		"client_id":    {"cc-only-client"},
		"redirect_uri": {"https://app.example.com/callback"},
	}
	w := postToTokenRecorder(t, form)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}

	body := decodeOAuthError(t, w)
	if body.Error != string(oauthUnauthorizedClient) {
		t.Errorf("error = %q, want %q (RFC 6749 §5.2: client not authorized to use this grant type)", body.Error, oauthUnauthorizedClient)
	}

	// The description text must actually match the code -- before this fix,
	// both this case and the unsupported_grant_type case shared one message
	// that literally said "unsupported grant type", which would have been
	// actively misleading paired with an "unauthorized_client" code.
	if !strings.Contains(strings.ToLower(body.ErrorDescription), "authoriz") {
		t.Errorf("error_description = %q, want it to describe an authorization problem, not a grant-type-support problem", body.ErrorDescription)
	}
}

func TestRevokeHandler_InvalidClient_RFCShapeAndChallenge(t *testing.T) {
	setupTokenTestKey(t)

	form := url.Values{
		"token":         {"whatever"},
		"client_id":     {"no-such-client"},
		"client_secret": {"wrong"},
	}

	req := httptest.NewRequest(http.MethodPost, "/oauth2/revoke", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	w := httptest.NewRecorder()
	sess := &router.Session{ID: 99}

	status := RevokeHandler(sess, w, req)

	if status != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", status)
	}

	body := decodeOAuthError(t, w)
	if body.Error != string(oauthInvalidClient) {
		t.Errorf("error = %q, want %q", body.Error, oauthInvalidClient)
	}

	if got := w.Header().Get("WWW-Authenticate"); !strings.Contains(got, "Basic") {
		t.Errorf("WWW-Authenticate = %q, want it to name the Basic scheme", got)
	}
}

// TestAuthorizeGetHandler_UnknownClient_ReturnsBadRequest guards 5.4: an
// unknown client_id at the browser-facing /oauth2/authorize endpoint used to
// answer 401 (mimicking the token endpoint's credential-challenge semantics
// with none of its context -- no WWW-Authenticate, no credential a browser
// could supply). It must answer 400, the same as every sibling validation
// check in this handler.
func TestAuthorizeGetHandler_UnknownClient_ReturnsBadRequest(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/oauth2/authorize?client_id=no-such-client&redirect_uri=https://app.example.com/cb&response_type=code", nil)
	w := httptest.NewRecorder()
	sess := &router.Session{ID: 99}

	status := AuthorizeGetHandler(sess, w, req)

	if status != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 (not 401 -- this is a login form, not a Basic-Auth API)", status)
	}
}
