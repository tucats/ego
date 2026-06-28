// Package authserver — low_test.go covers the OAUTH-L1 and OAUTH-L2 security
// fixes.  It lives in the same package (package authserver, not package
// authserver_test) so it can reach unexported helpers such as generateCSRFToken,
// csrfCookieName, and the package-level client registry.
package authserver

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/tucats/ego/internal/router"
)

// ─────────────────────────────────────────────────────────────────────────────
// OAUTH-L1: Secure flag on the CSRF cookie
// ─────────────────────────────────────────────────────────────────────────────

// setupL1Client registers a minimal OAuth2 client used by the AuthorizeGetHandler
// tests.  The handler validates the client before it ever sets the cookie, so
// the registry must have a matching entry.
func setupL1Client(t *testing.T) {
	t.Helper()

	clients = []OAuthClient{{
		ClientID:     "l1testapp",
		RedirectURIs: []string{"https://l1.example.com/cb"},
		GrantTypes:   []string{"authorization_code"},
		Scopes:       []string{"openid"},
	}}

	t.Cleanup(func() { clients = nil })
}

// csrfCookieFromResponse finds and returns the CSRF cookie from the recorder's
// response, or nil when the cookie is absent.
func csrfCookieFromResponse(w *httptest.ResponseRecorder) *http.Cookie {
	for _, c := range w.Result().Cookies() {
		if c.Name == csrfCookieName {
			return c
		}
	}

	return nil
}

// authorizeGetRequest builds a minimal valid GET /oauth2/authorize request.
// Pass https = true to simulate a TLS connection (sets r.TLS), which causes
// IsSecureRequest to return true and the cookie's Secure attribute to be set.
func authorizeGetRequest(https bool) *http.Request {
	// M2: l1testapp is a public client (no ClientSecretHash), so the M2 gate
	// now requires code_challenge to be present.  We supply a valid S256
	// challenge so these OAUTH-L1 cookie tests are not blocked by the PKCE
	// gate — they are testing the Secure attribute, not PKCE enforcement.
	req := httptest.NewRequest(
		http.MethodGet,
		"/oauth2/authorize?client_id=l1testapp"+
			"&redirect_uri=https://l1.example.com/cb"+
			"&response_type=code"+
			"&scope=openid"+
			"&code_challenge=E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"+
			"&code_challenge_method=S256",
		nil,
	)

	if https {
		// Setting r.TLS to a non-nil *tls.ConnectionState is how httptest
		// simulates an HTTPS connection.  The specific fields inside the struct
		// don't matter for IsSecureRequest — it only checks for nil vs non-nil.
		req.TLS = &tls.ConnectionState{}
	}

	return req
}

// TestAuthorizeGetHandler_SecureCookie_HTTPS verifies that AuthorizeGetHandler
// sets the CSRF cookie's Secure attribute to true when the request arrives
// over TLS (OAUTH-L1).
//
// Why the Secure flag matters:
//
//	Without Secure, a browser will transmit the cookie over plain HTTP as well
//	as HTTPS.  On a network where an attacker can observe plain-HTTP traffic —
//	for example, a shared Wi-Fi network — the cookie's nonce value can be read
//	and used to forge a CSRF-valid POST to the login form.
//
//	With Secure set, browsers silently discard the cookie on plain-HTTP requests,
//	so the nonce never travels unencrypted.
func TestAuthorizeGetHandler_SecureCookie_HTTPS(t *testing.T) {
	setupL1Client(t)

	w := httptest.NewRecorder()
	req := authorizeGetRequest(true) // simulate HTTPS

	status := AuthorizeGetHandler(&router.Session{ID: 1}, w, req)
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d", status)
	}

	c := csrfCookieFromResponse(w)
	if c == nil {
		t.Fatal("CSRF cookie not set in response")
	}

	// The cookie must carry Secure: true on an HTTPS connection (OAUTH-L1).
	if !c.Secure {
		t.Error("CSRF cookie Secure attribute should be true for an HTTPS request (OAUTH-L1)")
	}
}

// TestAuthorizeGetHandler_SecureCookie_PlainHTTP verifies that the CSRF cookie
// does NOT carry Secure when the request is plain HTTP (OAUTH-L1).
//
// Why omit Secure on plain HTTP?
//
//	Setting Secure on a plain-HTTP server would make the browser silently discard
//	the cookie, breaking the login form entirely in development environments that
//	run without TLS.  The correct behavior is to omit Secure on plain HTTP and
//	rely on SameSite=Strict as the remaining CSRF protection.
func TestAuthorizeGetHandler_SecureCookie_PlainHTTP(t *testing.T) {
	setupL1Client(t)

	w := httptest.NewRecorder()
	req := authorizeGetRequest(false) // plain HTTP — r.TLS is nil

	status := AuthorizeGetHandler(&router.Session{ID: 2}, w, req)
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d", status)
	}

	c := csrfCookieFromResponse(w)
	if c == nil {
		t.Fatal("CSRF cookie not set in response")
	}

	// Secure must be false on plain HTTP so the cookie is not silently dropped
	// by the browser on a development server.
	if c.Secure {
		t.Error("CSRF cookie Secure attribute should be false for a plain-HTTP request")
	}
}

// TestAuthorizeGetHandler_SecureCookie_ForwardedProto verifies that the CSRF
// cookie carries Secure when the request is forwarded from an HTTPS proxy
// (X-Forwarded-Proto: https), even though r.TLS is nil (OAUTH-L1).
//
// This covers the common deployment topology where Ego sits behind a reverse
// proxy (nginx, AWS ALB, etc.) that terminates TLS and forwards requests over
// plain HTTP to Ego.  In that case r.TLS is nil but the original connection
// was HTTPS, so the cookie should be marked Secure.
func TestAuthorizeGetHandler_SecureCookie_ForwardedProto(t *testing.T) {
	setupL1Client(t)

	w := httptest.NewRecorder()
	req := authorizeGetRequest(false) // r.TLS is nil (proxy scenario)
	req.Header.Set("X-Forwarded-Proto", "https")

	status := AuthorizeGetHandler(&router.Session{ID: 3}, w, req)
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d", status)
	}

	c := csrfCookieFromResponse(w)
	if c == nil {
		t.Fatal("CSRF cookie not set in response")
	}

	// The X-Forwarded-Proto header indicates the original connection was HTTPS,
	// so Secure should be true even though r.TLS is nil.
	if !c.Secure {
		t.Error("CSRF cookie Secure should be true when X-Forwarded-Proto: https is set (OAUTH-L1)")
	}
}

// TestReRenderWithError_SecureCookie_HTTPS verifies that the re-rendered form
// cookie (issued after a failed login attempt) also carries Secure: true on
// HTTPS connections (OAUTH-L1).
//
// OAUTH-H4 fixed an earlier bug where the re-render path did not issue a fresh
// cookie at all.  OAUTH-L1 now ensures that when the fresh cookie IS issued it
// carries the correct Secure attribute — the fix must apply to both code paths.
func TestReRenderWithError_SecureCookie_HTTPS(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/oauth2/authorize", nil)
	req.TLS = &tls.ConnectionState{} // simulate HTTPS

	_ = reRenderWithError(&router.Session{ID: 4}, w, req,
		"app", "https://app.example.com/cb",
		"openid", "state1", "", "", "Bad credentials")

	c := csrfCookieFromResponse(w)
	if c == nil {
		t.Fatal("CSRF cookie not set by reRenderWithError")
	}

	if !c.Secure {
		t.Error("re-render CSRF cookie Secure should be true for HTTPS (OAUTH-L1)")
	}
}

// TestReRenderWithError_SecureCookie_PlainHTTP verifies the negative case:
// the re-render cookie should NOT carry Secure on plain HTTP (OAUTH-L1).
func TestReRenderWithError_SecureCookie_PlainHTTP(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/oauth2/authorize", nil)
	// r.TLS is nil by default in httptest.NewRequest — plain HTTP.

	_ = reRenderWithError(&router.Session{ID: 5}, w, req,
		"app", "https://app.example.com/cb",
		"openid", "state2", "", "", "Bad credentials")

	c := csrfCookieFromResponse(w)
	if c == nil {
		t.Fatal("CSRF cookie not set by reRenderWithError")
	}

	if c.Secure {
		t.Error("re-render CSRF cookie Secure should be false for plain HTTP (OAUTH-L1)")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// OAUTH-L2: token endpoint accepts form-encoded requests
// ─────────────────────────────────────────────────────────────────────────────

// TestTokenHandler_AcceptsFormEncodedWithoutAcceptHeader verifies that
// TokenHandler processes a form-encoded POST even when the request has no
// Accept: application/json header (OAUTH-L2).
//
// Before the fix, the router registration included AcceptMedia(defs.JSONMediaType),
// which caused the router to reject any token request whose Accept header did not
// include "application/json".  RFC 6749 §4.1.3 says nothing about the Accept
// header on token requests — clients are expected to send form-encoded bodies
// and receive JSON responses, regardless of what they declare in Accept.
//
// This test calls TokenHandler directly (bypassing the router), which means it
// cannot test the router-level rejection that the fix removes.  What it DOES
// test is that TokenHandler itself never inspects or rejects based on the
// Accept header — the handler processes the request and returns a grant-type or
// credential error, not a media-type error.
func TestTokenHandler_AcceptsFormEncodedWithoutAcceptHeader(t *testing.T) {
	// A completely minimal form POST with no Accept header and an unknown
	// client — the handler should fail with 401 (invalid client), not 400
	// (media type rejected).
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", "nonexistent-client")

	req := httptest.NewRequest(http.MethodPost, "/oauth2/token",
		strings.NewReader(form.Encode()))

	// Set the correct content type for form-encoded bodies.
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	// Deliberately omit the Accept header — this is the OAUTH-L2 scenario.

	w := httptest.NewRecorder()
	status := TokenHandler(&router.Session{ID: 99}, w, req)

	// 401 means the handler reached the client-validation step — it was not
	// short-circuited by an Accept header check.  400 could mean either a
	// parse error or a media-type rejection; if the body mentions "media" we
	// flag it explicitly.
	if status == http.StatusBadRequest {
		body := w.Body.String()
		if strings.Contains(strings.ToLower(body), "media") ||
			strings.Contains(strings.ToLower(body), "accept") {
			t.Error("TokenHandler rejected the request due to an Accept header check — OAUTH-L2 fix is not working")
		}
	}

	// The expected outcome is 401 (unknown client).  Any other 4xx or 5xx that
	// is NOT caused by a media-type check is also acceptable.
	if status == http.StatusUnsupportedMediaType {
		t.Error("TokenHandler returned 415 Unsupported Media Type — Accept header is still being checked (OAUTH-L2)")
	}
}

// TestTokenHandler_UnknownGrantType_NotMediaTypeError verifies that the token
// endpoint processes a form-encoded POST body even without an Accept header,
// and that errors come from grant-type logic rather than media-type validation
// (OAUTH-L2).
//
// We send an unsupported grant_type so the handler returns 400 "invalid grant"
// from its own logic.  The body must mention "grant" (not "media" or "accept"),
// proving the handler parsed the form and dispatched on the grant type rather
// than being blocked at the router's Accept-header check.
func TestTokenHandler_UnknownGrantType_NotMediaTypeError(t *testing.T) {
	clients = nil // empty registry — handler will reject on grant type, not client

	form := url.Values{}
	form.Set("grant_type", "urn:totally:unsupported:grant")
	form.Set("client_id", "any")

	req := httptest.NewRequest(http.MethodPost, "/oauth2/token",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	// Deliberately no Accept header.

	w := httptest.NewRecorder()
	_ = TokenHandler(&router.Session{ID: 101}, w, req)

	body := strings.ToLower(w.Body.String())

	// A media-type rejection would say "media" or "accept"; a grant-type
	// rejection says "grant".  We must see the latter (OAUTH-L2).
	if strings.Contains(body, "media") || strings.Contains(body, "unsupported media") {
		t.Error("response mentions 'media' — Accept header check may still be active (OAUTH-L2)")
	}

	// The handler dispatches on grant_type and falls through to the default
	// "unsupported grant type" branch, so the body should mention "grant".
	if !strings.Contains(body, "grant") {
		t.Errorf("expected grant-type error in response body, got: %s", body)
	}
}
