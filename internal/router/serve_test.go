package router

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/i18n"
	"github.com/tucats/ego/internal/server/auth"
	egostrings "github.com/tucats/ego/internal/util/strings"
)

// TestNegotiateLanguage checks that negotiateLanguage (the helper that
// populates Session.Language from the incoming request) correctly reads
// the standard "Accept-Language" HTTP header and falls back to the
// server's own default language when the header is missing or names only
// languages Ego has no translations for.
//
// Most of the actual header-parsing logic lives in, and is more
// thoroughly tested by, i18n.NegotiateLanguage -- this test exists to
// confirm that negotiateLanguage wires that function up to the request
// correctly, and that the "use the server default" fallback works.
func TestNegotiateLanguage(t *testing.T) {
	// Pin the server's own default language so the "fallback" test cases
	// below have a known, fixed value to compare against, regardless of
	// what EGO_LANG/LANG happen to be set to in whatever environment runs
	// this test.
	i18n.Language = "en"

	tests := []struct {
		name           string
		acceptLanguage string
		want           string
	}{
		{
			name:           "client prefers french over english",
			acceptLanguage: "fr-CA,fr;q=0.9,en;q=0.8",
			want:           "fr",
		},
		{
			name:           "client asks for spanish only",
			acceptLanguage: "es",
			want:           "es",
		},
		{
			name:           "no Accept-Language header at all falls back to server default",
			acceptLanguage: "",
			want:           "en",
		},
		{
			name:           "Accept-Language names only unsupported languages, falls back to server default",
			acceptLanguage: "de,it,xx",
			want:           "en",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := http.NewRequest(http.MethodGet, "/services/test", nil)
			if err != nil {
				t.Fatalf("failed to build test request: %v", err)
			}

			if tt.acceptLanguage != "" {
				r.Header.Set("Accept-Language", tt.acceptLanguage)
			}

			if got := negotiateLanguage(r); got != tt.want {
				t.Errorf("negotiateLanguage() with Accept-Language=%q = %q, want %q", tt.acceptLanguage, got, tt.want)
			}
		})
	}
}

// TestServeHTTPSetsSessionLanguage is a smoke test confirming that a real
// request, routed all the way through Router.ServeHTTP, ends up with its
// Session.Language field populated from the Accept-Language header --
// not just that the negotiateLanguage helper function works in isolation.
//
// The handler captures the *Session it was called with, so the test can
// inspect session.Language after the request completes.
func TestServeHTTPSetsSessionLanguage(t *testing.T) {
	i18n.Language = "en"

	var capturedSession *Session

	m := NewRouter("language-test")
	m.New("/services/language-test", func(session *Session, w http.ResponseWriter, r *http.Request) int {
		capturedSession = session

		return http.StatusOK
	}, http.MethodGet).Authentication(false, false)

	r, err := http.NewRequest(http.MethodGet, "/services/language-test", nil)
	if err != nil {
		t.Fatalf("failed to build test request: %v", err)
	}

	r.Header.Set("Accept-Language", "fr;q=0.9,en;q=0.8")

	// httptest.NewRecorder gives us a real http.ResponseWriter that
	// records what's written to it in memory, without needing to start an
	// actual network listener.
	recorder := httptest.NewRecorder()

	m.ServeHTTP(recorder, r)

	if capturedSession == nil {
		t.Fatal("handler was never called, so no Session was captured")
	}

	if capturedSession.Language != "fr" {
		t.Errorf("Session.Language = %q, want %q", capturedSession.Language, "fr")
	}
}

// TestServeHTTPStopsAtFirstMissingPermission guards against a route that
// requires more than one permission producing a corrupted response.
//
// The permission-check loop used to call util.ErrorResponse once for every
// missing permission rather than stopping at the first one. Each call
// writes its own status line and its own JSON body; since a response can
// only be written to once, every call after the first appends another JSON
// document onto the one already sent instead of replacing it. A caller
// missing two or more of a route's required permissions would receive a
// response body that is not valid JSON at all -- two (or more) documents
// concatenated together.
//
// This requires a genuinely authenticated, non-admin caller: an
// unauthenticated request is rejected earlier, by the plain
// "!session.Authenticated && route.mustAuthenticate" check, and never
// reaches the permission loop at all. So the test registers a real user
// with valid credentials and no permissions, and authenticates as that user
// via HTTP Basic Auth.
func TestServeHTTPStopsAtFirstMissingPermission(t *testing.T) {
	const (
		username = "permission-test-user"
		password = "permission-test-password"
	)

	if err := auth.AuthService.WriteUser(0, defs.User{
		Name:     username,
		Password: egostrings.HashString(password),
		// ValidatePassword requires "logon" (or "root") before it will accept
		// any password as valid at all -- without it, this user could never
		// authenticate, and the test would never reach the permission-check
		// loop this is meant to exercise. Deliberately not "perm-a"/"perm-b",
		// so the user is authenticated but still missing both permissions the
		// test route requires.
		Permissions: []string{defs.LogonPermission},
	}); err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	handlerCalled := false

	m := NewRouter("permission-test")
	m.New("/services/permission-test", func(session *Session, w http.ResponseWriter, r *http.Request) int {
		handlerCalled = true

		return http.StatusOK
	}, http.MethodGet).Permissions("perm-a", "perm-b")

	r, err := http.NewRequest(http.MethodGet, "/services/permission-test", nil)
	if err != nil {
		t.Fatalf("failed to build test request: %v", err)
	}

	r.SetBasicAuth(username, password)

	recorder := httptest.NewRecorder()

	m.ServeHTTP(recorder, r)

	if handlerCalled {
		t.Fatal("route handler was called despite the caller having neither required permission")
	}

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusForbidden)
	}

	var body defs.RestStatusResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("response body did not parse as a single JSON document (want one error for the missing-permission set, not one per missing permission): %v\nbody: %s", err, recorder.Body.String())
	}

	if body.Status != http.StatusForbidden {
		t.Errorf("body.Status = %d, want %d", body.Status, http.StatusForbidden)
	}
}
