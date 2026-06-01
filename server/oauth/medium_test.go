// Package oauth — medium_test.go covers the OAUTH-M2, OAUTH-M4, and OAUTH-M5
// security fixes.  It lives in the same package (package oauth, not package
// oauth_test) so it can reach unexported package-level variables such as
// idpClient, stateStore, and the size-limit constants.
package oauth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// ─────────────────────────────────────────────────────────────────────────────
// OAUTH-M2: bounded HTTP client
// ─────────────────────────────────────────────────────────────────────────────

// TestIdpClientHasTimeout verifies that idpClient, the package-level HTTP
// client used for all outbound identity-provider calls, carries an explicit
// request timeout (OAUTH-M2).
//
// http.DefaultClient has a Timeout of zero, meaning requests never time out.
// A slow or dead IdP would then hold the calling goroutine open indefinitely,
// eventually exhausting the server's goroutine pool (Slowloris-style DoS on
// outbound connections).
func TestIdpClientHasTimeout(t *testing.T) {
	if idpClient.Timeout == 0 {
		t.Fatal("idpClient.Timeout is zero — a hung IdP would block goroutines indefinitely (OAUTH-M2)")
	}

	// Sanity-check the upper bound: anything over 60 seconds provides almost
	// no protection against a slow server.
	const maxReasonable = 60 * time.Second

	if idpClient.Timeout > maxReasonable {
		t.Errorf("idpClient.Timeout = %v; want <= %v (OAUTH-M2)", idpClient.Timeout, maxReasonable)
	}
}

// TestIdpClientIsUsedForDiscovery verifies that discoverEndpoints routes its
// request through idpClient rather than http.DefaultClient (OAUTH-M2).
//
// Technique — "spy transport":
//
//	We swap idpClient's Transport for a spyRoundTripper that sets a boolean
//	flag whenever RoundTrip is called, then delegates to the real transport so
//	the test server receives the request normally.  After discoverEndpoints
//	returns we check the flag.
//
//	If the production code were changed back to http.DefaultClient.Get(...)
//	or http.Get(...), this test would fail because our spy would never be
//	invoked.
func TestIdpClientIsUsedForDiscovery(t *testing.T) {
	resetDiscoveryCache()

	// Build a minimal test HTTP server serving a valid discovery document.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/.well-known/openid-configuration") {
			http.NotFound(w, r)

			return
		}

		// validDiscoveryDoc is defined in discovery_test.go (same package).
		body, _ := json.Marshal(validDiscoveryDoc("http://" + r.Host))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	// Install a spy transport that records whether it was called.
	called := false
	saved := idpClient

	idpClient = &http.Client{
		Timeout: 10 * time.Second,
		Transport: spyRoundTripper{
			// Delegate to the real transport so the test server receives the
			// request and we get a valid response.
			delegate: http.DefaultTransport,
			onCall:   func() { called = true },
		},
	}

	// Restore the original client when the test exits, even on failure.
	defer func() { idpClient = saved }()

	if _, err := discoverEndpoints(srv.URL); err != nil {
		t.Fatalf("discoverEndpoints() error: %v", err)
	}

	if !called {
		t.Error("discoverEndpoints() did not route through idpClient — OAUTH-M2 is broken")
	}

	resetDiscoveryCache()
}

// spyRoundTripper wraps an http.RoundTripper and calls onCall before each
// request.  It implements the http.RoundTripper interface, which requires only:
//
//	RoundTrip(*http.Request) (*http.Response, error)
//
// By plugging this into http.Client.Transport, every request the client makes
// goes through RoundTrip, allowing tests to verify which client was used.
type spyRoundTripper struct {
	// delegate performs the actual network request.
	delegate http.RoundTripper

	// onCall is invoked once before each network request.
	onCall func()
}

// RoundTrip satisfies the http.RoundTripper interface.
func (s spyRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	s.onCall()

	return s.delegate.RoundTrip(req)
}

// ─────────────────────────────────────────────────────────────────────────────
// OAUTH-M4: size-limited response reads
// ─────────────────────────────────────────────────────────────────────────────

// maxDiscoveryBytes and maxJWKSBytes mirror the constants defined inside
// discoverEndpoints and refreshJWKS.  They are duplicated here (rather than
// exported from production code) so the production code does not need to
// export values that are purely an implementation detail.  If the production
// limits change, update these constants to match.
const (
	testMaxDiscoveryBytes = 1 << 20 // 1 MiB — must match discovery.go
	testMaxJWKSBytes      = 1 << 20 // 1 MiB — must match jwks.go
)

// TestDiscoverEndpoints_OversizedBody verifies that discoverEndpoints returns
// an error when the IdP responds with a body larger than the 1 MiB cap
// (OAUTH-M4).
//
// How io.LimitedReader works (for novice Go developers):
//
//	io.LimitedReader{R: reader, N: limit} wraps reader and counts bytes.
//	After N bytes have been read, further reads return (0, io.EOF) — so
//	io.ReadAll stops as though the stream ended.  The trick is to check
//	whether lr.N reached zero after reading; if it did, the original stream
//	was longer than N bytes and the result is truncated.
//
//	We test this by serving exactly limit+1 bytes and expecting an error.
func TestDiscoverEndpoints_OversizedBody(t *testing.T) {
	resetDiscoveryCache()

	// One byte over the limit — the smallest body that must be rejected.
	oversized := bytes.Repeat([]byte("x"), testMaxDiscoveryBytes+1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(oversized)
	}))
	defer srv.Close()

	_, err := discoverEndpoints(srv.URL)
	if err == nil {
		t.Fatal("discoverEndpoints() should return an error when the response body exceeds the size cap (OAUTH-M4)")
	}

	// The error message should say "exceeds" so operators know why the
	// discovery document was rejected.
	if !strings.Contains(err.Error(), "exceeds") {
		t.Errorf("error message should contain 'exceeds', got: %v", err)
	}

	resetDiscoveryCache()
}

// TestDiscoverEndpoints_ExactlyAtLimit verifies that a response body of exactly
// testMaxDiscoveryBytes is accepted by the size check (boundary condition).
//
// The body will fail JSON parsing (it is not valid JSON), so we still expect
// an error — but it must be a parse error, not a size-limit error.  A
// size-limit error here would mean an off-by-one bug in the limit check.
func TestDiscoverEndpoints_ExactlyAtLimit(t *testing.T) {
	resetDiscoveryCache()

	// Exactly the limit — should pass the size check, then fail at JSON parse.
	atLimit := bytes.Repeat([]byte("z"), testMaxDiscoveryBytes)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(atLimit)
	}))
	defer srv.Close()

	_, err := discoverEndpoints(srv.URL)
	if err == nil {
		t.Error("expected a JSON parse error for a non-JSON body at the size limit")
	}

	// Must NOT be a size-limit error — the body was within the allowed range.
	if strings.Contains(err.Error(), "exceeds") {
		t.Errorf("got a size-limit error at exactly the limit — possible off-by-one bug: %v", err)
	}

	resetDiscoveryCache()
}

// TestRefreshJWKS_OversizedBody verifies that refreshJWKS returns an error
// when the JWKS endpoint returns more than testMaxJWKSBytes (OAUTH-M4).
//
// Real-world JWKS documents contain a handful of public keys and are well
// under 10 KiB.  1 MiB is hundreds of times larger — a body that size is
// almost certainly a misconfiguration or an attack.
func TestRefreshJWKS_OversizedBody(t *testing.T) {
	resetJWKSCache()

	oversized := bytes.Repeat([]byte("k"), testMaxJWKSBytes+1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(oversized)
	}))
	defer srv.Close()

	err := refreshJWKS(srv.URL)
	if err == nil {
		t.Fatal("refreshJWKS() should return an error when the response body exceeds the size cap (OAUTH-M4)")
	}

	if !strings.Contains(err.Error(), "exceeds") {
		t.Errorf("error message should contain 'exceeds', got: %v", err)
	}

	resetJWKSCache()
}

// ─────────────────────────────────────────────────────────────────────────────
// OAUTH-M5: state store cap
// ─────────────────────────────────────────────────────────────────────────────

// TestNewState_EnforcesMaxPendingStates verifies that newState returns an error
// when stateStore.items already contains maxPendingStates entries (OAUTH-M5).
//
// How the cap works:
//
//	newState acquires stateStore.mu and checks len(stateStore.items) before
//	inserting.  If the count is already at maxPendingStates, it returns an error
//	without modifying the map.  The check and the insert share the same lock,
//	so there is no window between them where another goroutine could sneak in
//	(no TOCTOU race).
//
// Test strategy:
//
//	We inject synthetic entries directly into stateStore.items to fill it to
//	the cap without generating real random tokens.  After the test, t.Cleanup
//	removes every injected key.
func TestNewState_EnforcesMaxPendingStates(t *testing.T) {
	const redirect = "https://cap.example.com/cb"

	// Collect the keys we inject so the cleanup can remove exactly them.
	injected := make([]string, 0, maxPendingStates)

	stateStore.mu.Lock()
	for i := len(stateStore.items); i < maxPendingStates; i++ {
		// Synthetic key: deterministic and clearly fake, so it is easy to
		// identify in a failing test's debug output.
		key := stateTestKey("cap", i)

		stateStore.items[key] = &pendingState{
			RedirectURI: redirect,
			CreatedAt:   time.Now(),
		}

		injected = append(injected, key)
	}
	stateStore.mu.Unlock()

	// Always clean up the injected entries so other tests start with a known
	// state.  t.Cleanup runs even when the test fails.
	t.Cleanup(func() {
		stateStore.mu.Lock()
		for _, k := range injected {
			delete(stateStore.items, k)
		}
		stateStore.mu.Unlock()
	})

	// With the store full, newState must return an error.
	_, _, err := newState(redirect)
	if err == nil {
		t.Errorf("newState() should fail when stateStore has %d entries (OAUTH-M5)", maxPendingStates)
	}

	if !strings.Contains(err.Error(), "too many") {
		t.Errorf("error should mention 'too many', got: %v", err)
	}
}

// TestNewState_BelowCapSucceeds verifies that newState works normally when the
// store is below maxPendingStates (OAUTH-M5 positive path).
//
// A too-strict cap implementation could incorrectly reject all insertions even
// when the store is nearly empty.  This test is the regression guard.
func TestNewState_BelowCapSucceeds(t *testing.T) {
	stateStore.mu.Lock()
	currentCount := len(stateStore.items)
	stateStore.mu.Unlock()

	if currentCount >= maxPendingStates {
		t.Skipf("stateStore is already at or above cap (%d/%d) — cannot test below-cap path",
			currentCount, maxPendingStates)
	}

	state, verifier, err := newState("https://below-cap.example.com/cb")
	if err != nil {
		t.Fatalf("newState() below cap returned unexpected error: %v", err)
	}

	if state == "" {
		t.Error("newState() returned an empty state string")
	}

	if verifier == "" {
		t.Error("newState() returned an empty code_verifier")
	}

	// Remove the entry we just created so it does not affect other tests.
	t.Cleanup(func() { _, _ = validateState(state) })
}

// TestNewState_CapIsAtomicCheckAndInsert verifies that the cap check and the
// map insert are performed inside the same mutex lock (no TOCTOU race).
//
// We fill the store to cap-1, then call newState twice sequentially:
//
//   - Call 1: store has cap-1 entries → insert succeeds → store has cap entries.
//   - Call 2: store has cap entries  → insert refused → error returned.
//
// If the check and insert were not atomic (e.g., check under read lock, insert
// under write lock), two goroutines could both see "count < cap" and both
// insert, silently exceeding the cap.  This sequential test detects the
// simpler case; the race detector covers the concurrent case.
func TestNewState_CapIsAtomicCheckAndInsert(t *testing.T) {
	const redirect = "https://atomic.example.com/cb"

	injected := make([]string, 0, maxPendingStates)

	stateStore.mu.Lock()
	for i := len(stateStore.items); i < maxPendingStates-1; i++ {
		key := stateTestKey("atomic", i)

		stateStore.items[key] = &pendingState{
			RedirectURI: redirect,
			CreatedAt:   time.Now(),
		}

		injected = append(injected, key)
	}
	stateStore.mu.Unlock()

	t.Cleanup(func() {
		stateStore.mu.Lock()
		for _, k := range injected {
			delete(stateStore.items, k)
		}
		stateStore.mu.Unlock()
	})

	// Call 1: store is at cap-1.  This should succeed.
	state1, _, err1 := newState(redirect)
	if err1 != nil {
		t.Fatalf("newState() at cap-1 should succeed; got error: %v", err1)
	}

	t.Cleanup(func() { _, _ = validateState(state1) })

	// Call 2: store is now at cap.  This must fail.
	_, _, err2 := newState(redirect)
	if err2 == nil {
		t.Error("newState() at cap should fail — the cap check or the atomicity is broken (OAUTH-M5)")
	}
}

// stateTestKey builds a deterministic, human-readable key for test injection.
// The prefix argument labels the test that created the entry; the index makes
// each key unique within that test.
//
// Using a zero-padded decimal index guarantees uniqueness for any number of
// entries — the previous single-character approach collided at index 26
// ('A' + 26%26 == 'A' again), silently capping the injected count at 26 and
// causing the cap test to fail because the store was never actually full.
func stateTestKey(prefix string, index int) string {
	// Format: "TEST-<prefix>-<zero-padded-index>-injected"
	// Example: "TEST-cap-0042-injected"
	return fmt.Sprintf("TEST-%s-%04d-injected", prefix, index)
}
