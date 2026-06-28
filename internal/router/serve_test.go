package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tucats/ego/internal/i18n"
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
			acceptLanguage: "de,it,ja",
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
