package authserver

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/tucats/ego/router"
)

const egoAdminScope = "ego:admin"

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
			body[:minInt(300, len(body))])
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

// minInt returns the smaller of a and b.  Used only for safe body truncation in
// error messages; intentionally simple rather than importing math or slices.
func minInt(a, b int) int {
	if a < b {
		return a
	}

	return b
}

// ─────────────────────────────────────────────────────────────────────────────
// H3: constant-time CSRF token comparison
// ─────────────────────────────────────────────────────────────────────────────
//
// AuthorizePostHandler compares the CSRF cookie value against the hidden-form
// csrf_token field using subtle.ConstantTimeCompare (H3 fix).  The tests below
// verify that the gate fires correctly for every mismatch case and that a
// matching pair lets the request proceed past the CSRF check.
//
// The CSRF check runs before client validation, so these tests require no
// registered client and no user database — the handler is exercised without
// any auth infrastructure.

// postAuthorizeFormWithCSRF is a helper that drives AuthorizePostHandler with
// the given form values and optionally attaches a CSRF cookie.  It returns
// the HTTP status code.
//
// cookieValue is the value to set in the ego_oauth_csrf cookie.  Pass an empty
// string to simulate a missing cookie (the helper omits the cookie entirely
// when the value is empty, matching the browser behavior of not sending a
// cookie that was never set).
func postAuthorizeFormWithCSRF(t *testing.T, formCSRF, cookieValue string) int {
	t.Helper()

	// Build a minimal form that only populates the csrf_token field.  All other
	// fields (client_id, redirect_uri, etc.) are intentionally left blank so
	// that any rejection happens at the CSRF gate, not at a later check.
	form := url.Values{}
	form.Set("csrf_token", formCSRF)

	req := httptest.NewRequest(http.MethodPost, "/oauth2/authorize",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Only attach the cookie when a value was provided.  An empty cookieValue
	// simulates the case where the browser sent no CSRF cookie at all —
	// for example, if the user navigated directly to the POST endpoint without
	// first loading the GET form.
	if cookieValue != "" {
		req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: cookieValue})
	}

	w := httptest.NewRecorder()

	return AuthorizePostHandler(&router.Session{ID: 50}, w, req)
}

// TestCSRFValidation_MismatchedToken_Rejected verifies that a POST whose CSRF
// form token does not match the CSRF cookie is rejected with 403 Forbidden (H3).
//
// This is the primary CSRF defense: the cookie is set by the server when it
// renders the login form (HttpOnly, so JavaScript cannot read it), and the
// same value is embedded as a hidden input.  Both must be present and identical
// on the POST, which is only possible if the POST originated from the server's
// own form.  A cross-site POST cannot supply the correct cookie value because
// browsers do not allow cross-origin JavaScript to read other origins' cookies.
func TestCSRFValidation_MismatchedToken_Rejected(t *testing.T) {
	// Cookie holds "correct-nonce"; the form carries "wrong-nonce".
	// The two values are deliberately different to exercise the mismatch path.
	status := postAuthorizeFormWithCSRF(t, "wrong-nonce", "correct-nonce")

	if status != http.StatusForbidden {
		t.Errorf(
			"H3: mismatched CSRF token should return 403 Forbidden, got %d\n"+
				"(if this is 400 or 401 the CSRF gate is not firing before client validation)",
			status,
		)
	}
}

// TestCSRFValidation_EmptyFormToken_Rejected verifies that a POST with an empty
// csrf_token form field is rejected with 403 Forbidden even when the cookie is
// present and non-empty (H3).
//
// An empty form token can happen when:
//   - A bug in the login form template caused the hidden input to be blank.
//   - An attacker stripped the hidden input from a cross-origin form.
//
// The empty check is in the guard condition:
//
//	csrfCookie.Value == "" || subtle.ConstantTimeCompare(...) != 1
//
// An empty form token still fails ConstantTimeCompare against a non-empty
// cookie value, so this test also validates that path indirectly.  The
// explicit cookie-empty guard is tested separately in
// TestCSRFValidation_EmptyCookie_Rejected.
func TestCSRFValidation_EmptyFormToken_Rejected(t *testing.T) {
	// Cookie has a value; form field is empty string.
	status := postAuthorizeFormWithCSRF(t, "" /*form token*/, "some-valid-nonce" /*cookie*/)

	if status != http.StatusForbidden {
		t.Errorf("H3: empty form csrf_token should return 403 Forbidden, got %d", status)
	}
}

// TestCSRFValidation_EmptyCookie_Rejected verifies that a POST is rejected when
// the CSRF cookie is absent entirely (H3).
//
// The guard condition checks cookieErr != nil first, which handles both
// "cookie not found" and "cookie value empty" cases.  Passing an empty
// cookieValue to the helper causes it to omit the cookie, so r.Cookie() returns
// http.ErrNoCookie and cookieErr is non-nil.
func TestCSRFValidation_EmptyCookie_Rejected(t *testing.T) {
	// "" cookieValue → helper omits the cookie from the request entirely.
	status := postAuthorizeFormWithCSRF(t, "some-nonce" /*form token*/, "" /*no cookie*/)

	if status != http.StatusForbidden {
		t.Errorf("H3: missing CSRF cookie should return 403 Forbidden, got %d", status)
	}
}

// TestCSRFValidation_MatchingTokens_PassesCSRFGate verifies that when the CSRF
// cookie and form token are identical the CSRF gate accepts the request and
// execution moves on to the next check (client validation in this case) (H3).
//
// Because no client is registered in this test, the request fails at the
// client_id check (returns 400 or 401) rather than at the CSRF gate (403).
// The test asserts only that the response is NOT 403, proving the gate was
// passed.  This is the correct level of assertion: we are testing the CSRF
// gate in isolation, not the full login flow.
func TestCSRFValidation_MatchingTokens_PassesCSRFGate(t *testing.T) {
	// Use a realistic-looking nonce — the exact value doesn't matter, only
	// that cookie and form carry the same string.
	const sharedNonce = "ab12cd34ef56gh78ij90kl12mn34op56"

	status := postAuthorizeFormWithCSRF(t, sharedNonce, sharedNonce)

	if status == http.StatusForbidden {
		t.Errorf(
			"H3: matching CSRF token should pass the CSRF gate (not 403), got %d\n"+
				"(the constant-time comparison is incorrectly rejecting equal values)",
			status,
		)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// H2: client scope allowlist enforcement
// ─────────────────────────────────────────────────────────────────────────────
//
// The H2 fix adds a clientAllowsScope gate in AuthorizePostHandler that runs
// before intersectScopes.  Without it, a client registered for ["openid","ego:read"]
// could request "ego:admin" and receive it when a privileged user logs in.
//
// Testing this at the full-handler level requires a live authenticated user
// (validatePassword must succeed), which demands a running auth database.  The
// tests below instead verify the security property at the unit level:
//
//  1. clientAllowsScope alone correctly gates elevated scopes (H2 gate logic).
//  2. intersectScopes alone does NOT protect against client elevation (proving
//     why the extra gate was necessary).
//  3. Both gates together enforce the combined policy.
//
// These unit tests are complementary to the existing clientAllowsScope tests in
// clients_test.go; they focus on the security invariant rather than the API.

// TestH2_ClientScopeGate_BlocksElevatedScope verifies that clientAllowsScope
// returns false when the client requests a scope it was not registered for.
//
// The key security scenario: a client is registered for ["openid","ego:read"]
// but requests "ego:admin".  The gate must block the request even if the
// authenticated user is an admin, because the administrator scoped this
// client's privilege at registration time.
func TestH2_ClientScopeGate_BlocksElevatedScope(t *testing.T) {
	// A client registered with a restricted set of scopes — typical for a
	// read-only dashboard application that should never obtain write access.
	restrictedClient := &OAuthClient{
		ClientID: "read-only-dashboard",
		Scopes:   []string{"openid", "ego:read"},
	}

	// The client requests ego:admin — a scope it was never registered for.
	// This simulates either a misconfigured client or a tampered hidden form field.
	requestedScopes := []string{"openid", egoAdminScope}

	if clientAllowsScope(restrictedClient, requestedScopes) {
		t.Errorf(
			"H2: clientAllowsScope should return false when the client requests "+
				"ego:admin which is not in its registered Scopes [openid ego:read]\n"+
				"(if this passes, the H2 gate would allow scope elevation)",
		)
	}
}

// TestH2_ClientScopeGate_AllowsRegisteredScopes verifies that clientAllowsScope
// returns true when every requested scope is in the client's registered list.
//
// This is the positive path: a correctly configured client requesting only what
// it was registered for must not be blocked by the H2 gate.
func TestH2_ClientScopeGate_AllowsRegisteredScopes(t *testing.T) {
	client := &OAuthClient{
		ClientID: "standard-app",
		Scopes:   []string{"openid", "profile", "ego:read"},
	}

	// Request a subset of the registered scopes — normal operation.
	requestedScopes := []string{"openid", "ego:read"}

	if !clientAllowsScope(client, requestedScopes) {
		t.Errorf(
			"H2: clientAllowsScope should return true when every requested scope "+
				"is in the client's registered list [openid profile ego:read]\n"+
				"(the H2 gate is incorrectly blocking a legitimate request)",
		)
	}
}

// TestH2_IntersectScopesAlone_DoesNotBlockElevation demonstrates WHY the H2
// gate was needed: intersectScopes alone (the pre-fix code) would grant
// "ego:admin" to a restricted client if the user is an admin.
//
// This test documents the vulnerability by running intersectScopes against a
// privileged user's permissions WITHOUT the prior clientAllowsScope gate.  The
// result shows that the user's root permission would cause "ego:admin" to be
// granted, bypassing the client-scope restriction.
//
// After the H2 fix, AuthorizePostHandler calls clientAllowsScope BEFORE
// intersectScopes, so this code path is never reached for disallowed scopes.
// The test is kept to make the vulnerability visible in the test suite as an
// explanation, not as a regression test of broken behavior.
func TestH2_IntersectScopesAlone_DoesNotBlockElevation(t *testing.T) {
	// Simulate a privileged user: the root permission maps to the ego:admin scope
	// inside intersectScopes (see the permission-to-scope mapping in authorize.go).
	adminUserPermissions := []string{"ego.root", "ego.logon"}

	// The client requested ego:admin — a scope it is not registered for.
	requestedScopes := []string{"openid", egoAdminScope}

	// Call intersectScopes directly, simulating what happened BEFORE the H2 fix.
	granted := intersectScopes(requestedScopes, adminUserPermissions)

	// Without the H2 gate, intersectScopes would include ego:admin in the
	// result because the user has the root permission.  Document this finding.
	egoAdminGranted := false

	for _, s := range granted {
		if s == egoAdminScope {
			egoAdminGranted = true

			break
		}
	}

	if !egoAdminGranted {
		// If intersectScopes no longer grants ego:admin to root users, the test
		// assumption has changed.  Update the test to reflect the new behavior.
		t.Log("NOTE: intersectScopes did not grant ego:admin to a root user — " +
			"the test assumption about the vulnerability may have changed.")
	} else {
		// This is the expected (pre-fix) result: intersectScopes alone would
		// have granted ego:admin.  The H2 fix prevents reaching this code by
		// checking clientAllowsScope first in the handler.
		t.Log("Confirmed: intersectScopes alone grants ego:admin to a root user " +
			"regardless of the client's registered Scopes.  The H2 clientAllowsScope " +
			"gate in AuthorizePostHandler prevents this scope elevation at the handler level.")
	}
}

// TestH2_BothGatesTogether_EnforcePolicy verifies that the combined policy
// (clientAllowsScope AND intersectScopes) correctly restricts the granted
// scopes to the intersection of what the client is registered for AND what
// the user is entitled to.
//
// This test runs both gates in sequence, mirroring the actual handler logic
// after the H2 fix, and confirms that the result never exceeds either boundary.
func TestH2_BothGatesTogether_EnforcePolicy(t *testing.T) {
	// A client registered for openid and ego:read only.
	client := &OAuthClient{
		ClientID: "gated-client",
		Scopes:   []string{"openid", "ego:read"},
	}

	// The user is an admin with all permissions.
	adminPermissions := []string{"ego.root", "ego.logon"}

	// The client requests openid and ego:admin — ego:admin is beyond its allowlist.
	requestedScopes := []string{"openid", egoAdminScope}

	// Gate 1 (H2): does the client allow all requested scopes?
	if clientAllowsScope(client, requestedScopes) {
		// In a real handler, execution would stop here with 400.  For the test,
		// we record a failure and continue so the reader sees the full picture.
		t.Error("H2 gate 1 FAILED: clientAllowsScope should have blocked ego:admin " +
			"for a client registered only for [openid ego:read]")
	}

	// Gate 2 (user-permission intersection): even if gate 1 were bypassed,
	// what would intersectScopes produce?  Show that it is NOT a sufficient
	// defense on its own.
	grantedByGate2 := intersectScopes(requestedScopes, adminPermissions)

	for _, s := range grantedByGate2 {
		if s == egoAdminScope {
			t.Log("Gate 2 alone would have granted ego:admin (confirms H2 was needed).")

			break
		}
	}

	// The correct combined result: only scopes that pass BOTH gates.
	// For this client/user pair, only "openid" passes (ego:admin fails gate 1;
	// ego:read was not requested so it is not in the result).
	//
	// In the handler the code never reaches gate 2 when gate 1 fails, but we
	// run gate 2 on the allowlist-validated scope subset to illustrate the full
	// two-gate pipeline.
	allowedByClient := []string{"openid"} // ego:admin stripped by gate 1
	finalGranted := intersectScopes(allowedByClient, adminPermissions)

	for _, s := range finalGranted {
		if s == "ego:admin" {
			t.Errorf("H2 combined policy: ego:admin should never appear in the final "+
				"granted set for a client registered for [openid ego:read]; got %v", finalGranted)
		}
	}
}
