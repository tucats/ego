package authserver

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/tucats/ego/router"
)

// ---- OAUTH-H4: fresh CSRF token on every re-render ----

// TestReRenderWithError_SetsFreshCSRFCookie verifies that reRenderWithError
// always issues a new Set-Cookie header with a non-empty CSRF value so that a
// user who sees an error can resubmit the form (OAUTH-H4).  An empty cookie
// value would make every subsequent POST fail the CSRF check permanently.
func TestReRenderWithError_SetsFreshCSRFCookie(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/oauth2/authorize", nil)
	session := &router.Session{ID: 1}

	status := reRenderWithError(session, w, req,
		"testclient", "https://example.com/cb",
		"openid", "state123", "", "", "Test error message")

	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d", status)
	}

	// A Set-Cookie header for the CSRF nonce must be present.
	var csrfCookie *http.Cookie

	for _, c := range w.Result().Cookies() {
		if c.Name == csrfCookieName {
			csrfCookie = c

			break
		}
	}

	if csrfCookie == nil {
		t.Fatal("expected a CSRF Set-Cookie header in the re-rendered response")
	}

	if csrfCookie.Value == "" {
		t.Error("CSRF cookie value must not be empty")
	}
}

// TestReRenderWithError_EmbedsFreshCSRFInForm verifies that the fresh CSRF
// nonce issued by reRenderWithError is embedded as a hidden input in the HTML
// form body, and that the value matches the Set-Cookie header (OAUTH-H4).
// Before this fix the CSRFToken field was left at its zero value, so the form
// always contained value="" and the user could never resubmit after an error.
func TestReRenderWithError_EmbedsFreshCSRFInForm(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/oauth2/authorize", nil)
	session := &router.Session{ID: 2}

	_ = reRenderWithError(session, w, req,
		"app", "https://app.example.com/cb",
		"openid profile", "st", "", "", "Bad credentials")

	body := w.Body.String()

	// The form must carry a csrf_token input.
	if !strings.Contains(body, `name="csrf_token"`) {
		t.Fatal("re-rendered form is missing the csrf_token hidden input")
	}

	// The csrf_token input value must NOT be empty string (the pre-fix bug).
	if strings.Contains(body, `name="csrf_token" value=""`) {
		t.Error("csrf_token value is empty — OAUTH-H4 fix is not working")
	}

	// The cookie value and the embedded form value must agree.
	var cookieValue string

	for _, c := range w.Result().Cookies() {
		if c.Name == csrfCookieName {
			cookieValue = c.Value

			break
		}
	}

	if cookieValue == "" {
		t.Fatal("no CSRF cookie set — cannot compare with form value")
	}

	if !strings.Contains(body, cookieValue) {
		t.Errorf("form body does not contain cookie value %q", cookieValue)
	}
}

// TestReRenderWithError_ContainsErrorMessage checks that the error string
// passed to reRenderWithError is rendered in the response body so the user
// can see what went wrong.
func TestReRenderWithError_ContainsErrorMessage(t *testing.T) {
	const errMsg = "Account temporarily locked. Please wait before trying again."

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/oauth2/authorize", nil)
	session := &router.Session{ID: 3}

	_ = reRenderWithError(session, w, req,
		"app", "https://app.example.com/cb",
		"openid", "", "", "", errMsg)

	if !strings.Contains(w.Body.String(), errMsg) {
		t.Errorf("error message %q not found in re-rendered body", errMsg)
	}
}

// ---- OAUTH-H1: rate limiting on the AS login form ----

// TestAuthorizePostHandler_LockedAccount verifies that when the rate limiter
// has locked a username, AuthorizePostHandler re-renders the form with a
// lockout message instead of attempting a password check (OAUTH-H1).
//
// The test pre-locks the account by recording enough failures to cross the
// lockout threshold, then drives a POST with a valid CSRF pair.  Because the
// account is locked, validatePassword is never called and no auth service is
// required.
func TestAuthorizePostHandler_LockedAccount(t *testing.T) {
	const lockedUser = "oauth_h1_locked_test_user"

	// Register a minimal public client so the handler does not reject early.
	clients = []OAuthClient{{
		ClientID:     "locktestapp",
		RedirectURIs: []string{"https://locktest.example.com/cb"},
		GrantTypes:   []string{"authorization_code"},
		Scopes:       []string{"openid"},
	}}

	t.Cleanup(func() { clients = nil })

	// Saturate the rate limiter for this username.  The default threshold is 5.
	for i := 0; i < 10; i++ {
		router.RecordFailure(0, lockedUser)
	}

	t.Cleanup(func() { router.RecordSuccess(lockedUser) })

	// Build a valid CSRF cookie / form token pair.
	csrfToken, err := generateCSRFToken()
	if err != nil {
		t.Fatalf("generateCSRFToken: %v", err)
	}

	form := url.Values{}
	form.Set("client_id", "locktestapp")
	form.Set("redirect_uri", "https://locktest.example.com/cb")
	form.Set("scope", "openid")
	form.Set("state", "s1")
	form.Set("username", lockedUser)
	form.Set("password", "anything")
	form.Set("csrf_token", csrfToken)

	req := httptest.NewRequest(http.MethodPost, "/oauth2/authorize",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: csrfToken})

	w := httptest.NewRecorder()
	session := &router.Session{ID: 10}

	status := AuthorizePostHandler(session, w, req)

	// A locked account must not succeed (302) — it must get the re-rendered
	// form (200) with a lockout message.
	if status != http.StatusOK {
		t.Errorf("expected 200 re-render for locked account, got %d", status)
	}

	body := w.Body.String()

	// The lockout error message must appear somewhere in the re-rendered form.
	if !strings.Contains(strings.ToLower(body), "locked") &&
		!strings.Contains(strings.ToLower(body), "temporarily") {
		t.Errorf("lockout message not found in re-rendered body (first 300 chars): %s",
			body[:min(300, len(body))])
	}
}

// TestAuthorizePostHandler_LockedAccount_FreshCSRF verifies that the re-render
// triggered by a locked-account rejection still contains a fresh CSRF token so
// that the user can retry once the lockout expires (OAUTH-H4 + OAUTH-H1 combo).
func TestAuthorizePostHandler_LockedAccount_FreshCSRF(t *testing.T) {
	const lockedUser2 = "oauth_h1_h4_combo_test_user"

	clients = []OAuthClient{{
		ClientID:     "combotestapp",
		RedirectURIs: []string{"https://combotest.example.com/cb"},
		GrantTypes:   []string{"authorization_code"},
		Scopes:       []string{"openid"},
	}}

	t.Cleanup(func() { clients = nil })

	for i := 0; i < 10; i++ {
		router.RecordFailure(0, lockedUser2)
	}

	t.Cleanup(func() { router.RecordSuccess(lockedUser2) })

	originalCSRF, _ := generateCSRFToken()

	form := url.Values{}
	form.Set("client_id", "combotestapp")
	form.Set("redirect_uri", "https://combotest.example.com/cb")
	form.Set("scope", "openid")
	form.Set("username", lockedUser2)
	form.Set("password", "any")
	form.Set("csrf_token", originalCSRF)

	req := httptest.NewRequest(http.MethodPost, "/oauth2/authorize",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: originalCSRF})

	w := httptest.NewRecorder()
	_ = AuthorizePostHandler(&router.Session{ID: 11}, w, req)

	// A fresh CSRF cookie must be set in the response.
	var newCookie *http.Cookie

	for _, c := range w.Result().Cookies() {
		if c.Name == csrfCookieName {
			newCookie = c

			break
		}
	}

	if newCookie == nil || newCookie.Value == "" {
		t.Error("expected a fresh non-empty CSRF cookie in the lockout re-render")
	}
}

// min returns the smaller of a and b.  Used only for safe body truncation in
// error messages; intentionally simple rather than importing math or slices.
func min(a, b int) int {
	if a < b {
		return a
	}

	return b
}
