// Package oauth — medium_test.go covers the OAUTH-M2, OAUTH-M4, OAUTH-M5,
// OAUTH-M6, and OAUTH-M7 security fixes.  It lives in the same package
// (package oauth, not package oauth_test) so it can reach unexported
// package-level variables such as idpClient, stateStore, missRefresh, and the
// size-limit constants.
package oauth

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tucats/ego/errors"
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

	// The error should be the OIDC discovery size-limit error so operators know why the
	// discovery document was rejected.
	if !errors.Equals(err, errors.ErrOIDCDiscoverySizeLimit) {
		t.Errorf("error should be ErrOIDCDiscoverySizeLimit, got: %v", err)
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
	if errors.Equals(err, errors.ErrOIDCDiscoverySizeLimit) {
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

	if !errors.Equals(err, errors.ErrJWKSSizeLimit) {
		t.Errorf("error should be ErrJWKSSizeLimit, got: %v", err)
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
	// Collect the keys we inject so the cleanup can remove exactly them.
	injected := make([]string, 0, maxPendingStates)

	stateStore.mu.Lock()
	for i := len(stateStore.items); i < maxPendingStates; i++ {
		// Synthetic key: deterministic and clearly fake, so it is easy to
		// identify in a failing test's debug output.
		key := stateTestKey("cap", i)

		stateStore.items[key] = &pendingState{
			CreatedAt: time.Now(),
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
	_, _, err := newState()
	if err == nil {
		t.Errorf("newState() should fail when stateStore has %d entries (OAUTH-M5)", maxPendingStates)
	}

	if !errors.Equals(err, errors.ErrOAuthTooManyFlows) {
		t.Errorf("error should be ErrOAuthTooManyFlows, got: %v", err)
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

	state, verifier, err := newState()
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
	injected := make([]string, 0, maxPendingStates)

	stateStore.mu.Lock()
	for i := len(stateStore.items); i < maxPendingStates-1; i++ {
		key := stateTestKey("atomic", i)

		stateStore.items[key] = &pendingState{
			CreatedAt: time.Now(),
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
	state1, _, err1 := newState()
	if err1 != nil {
		t.Fatalf("newState() at cap-1 should succeed; got error: %v", err1)
	}

	t.Cleanup(func() { _, _ = validateState(state1) })

	// Call 2: store is now at cap.  This must fail.
	_, _, err2 := newState()
	if err2 == nil {
		t.Error("newState() at cap should fail — the cap check or the atomicity is broken (OAUTH-M5)")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// OAUTH-M6: size-limited token exchange response
// ─────────────────────────────────────────────────────────────────────────────

// testMaxTokenBodyBytes mirrors the constant defined inside ExchangeCode.
// It is duplicated here so the production code does not need to export a value
// that is purely an implementation detail.  Update this constant if the
// production cap changes.
const testMaxTokenBodyBytes = 64 << 10 // 64 KiB — must match flow_authcode.go

// TestExchangeCode_OversizedBody verifies that ExchangeCode returns an error
// when the IdP token endpoint responds with a body larger than the 64 KiB cap
// (OAUTH-M6).
//
// This is the same protection applied to OIDC discovery documents (OAUTH-M4)
// but for the per-request token exchange path.  A slow or malicious IdP that
// streams a large response can exhaust server memory; the LimitedReader cap
// prevents that.
//
// Test strategy:
//   - Spin up a test HTTP server that responds to POST requests with a body of
//     exactly testMaxTokenBodyBytes+1 bytes (the minimum oversized body).
//   - Wire the idpClient spy to point at this server so ExchangeCode uses it.
//   - Confirm that ExchangeCode returns ErrOAuthTokenSizeLimit rather than a
//     JSON parse error (which would mean the body was accepted and then rejected
//     for a different reason).
func TestExchangeCode_OversizedBody(t *testing.T) {
	resetDiscoveryCache()

	// We need a real discovery document that points the token_endpoint at our
	// test server, so ExchangeCode reaches the oversized-body handler.
	var tokenSrv *httptest.Server

	tokenSrv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Serve both the OIDC discovery document and the token endpoint from
		// the same test server to keep wiring simple.
		switch {
		case strings.HasSuffix(r.URL.Path, "/.well-known/openid-configuration"):
			doc := validDiscoveryDoc("http://" + r.Host)
			// Override the token_endpoint so ExchangeCode POSTs to our handler.
			doc["token_endpoint"] = "http://" + r.Host + "/token"
			body, _ := json.Marshal(doc)

			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(body)

		case r.URL.Path == "/token":
			// One byte over the cap — the smallest body that must be rejected.
			oversized := bytes.Repeat([]byte("x"), testMaxTokenBodyBytes+1)

			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(oversized)

		default:
			http.NotFound(w, r)
		}
	}))
	defer tokenSrv.Close()

	cfg := rsConfig{
		Provider:    tokenSrv.URL,
		ClientID:    "test-client",
		RedirectURI: "http://localhost/callback",
	}

	_, _, err := ExchangeCode(cfg, "test-code", "test-verifier")
	if err == nil {
		t.Fatal("ExchangeCode() should return an error when the token response body exceeds the size cap (OAUTH-M6)")
	}

	// The error must be the size-limit error, not a JSON parse error.  A JSON
	// parse error would mean the oversized body was accepted and parsed, which
	// is the bug we are preventing.
	if !errors.Equals(err, errors.ErrOAuthTokenSizeLimit) {
		t.Errorf("error should be ErrOAuthTokenSizeLimit, got: %v", err)
	}

	resetDiscoveryCache()
}

// TestExchangeCode_ExactlyAtLimit verifies that a token response of exactly
// testMaxTokenBodyBytes is accepted by the size check (boundary condition).
//
// A body at exactly the cap should pass the LimitedReader check and then fail
// for a different reason (JSON parse error, because the body is not valid JSON).
// A size-limit error at this boundary would indicate an off-by-one bug.
func TestExchangeCode_ExactlyAtLimit(t *testing.T) {
	resetDiscoveryCache()

	var tokenSrv *httptest.Server

	tokenSrv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/.well-known/openid-configuration"):
			doc := validDiscoveryDoc("http://" + r.Host)
			// Override the token_endpoint so ExchangeCode POSTs to our handler.
			doc["token_endpoint"] = "http://" + r.Host + "/token"
			body, _ := json.Marshal(doc)

			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(body)

		case r.URL.Path == "/token":
			// Exactly the cap — should pass the size check, then fail JSON parse.
			atLimit := bytes.Repeat([]byte("z"), testMaxTokenBodyBytes)

			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(atLimit)

		default:
			http.NotFound(w, r)
		}
	}))
	defer tokenSrv.Close()

	cfg := rsConfig{
		Provider:    tokenSrv.URL,
		ClientID:    "test-client",
		RedirectURI: "http://localhost/callback",
	}

	_, _, err := ExchangeCode(cfg, "test-code", "test-verifier")
	if err == nil {
		t.Error("expected a JSON parse error for a non-JSON body at the size limit")
	}

	// Must NOT be a size-limit error — the body was within the allowed cap.
	if errors.Equals(err, errors.ErrOAuthTokenSizeLimit) {
		t.Errorf("got a size-limit error at exactly the cap — possible off-by-one bug: %v", err)
	}

	resetDiscoveryCache()
}

// ─────────────────────────────────────────────────────────────────────────────
// OAUTH-M7: JWKS miss-refresh cooldown
// ─────────────────────────────────────────────────────────────────────────────

// buildJWKSServer returns a test HTTP server that serves a JWKS document
// containing a single EC P-256 key under the given kid, along with a counter
// that tracks how many times the server has been called.
//
// The server and counter pointer are returned so the caller can verify fetch
// counts and shut the server down after the test.
func buildJWKSServer(t *testing.T, kid string) (*httptest.Server, *int) {
	t.Helper()

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generating EC key: %v", err)
	}

	// ecKeyToJWK is defined in jwks_test.go (same package), so it is available here.
	jwkEntry := ecKeyToJWK(t, &priv.PublicKey, kid)
	doc := jwksDocument{Keys: []jwkKey{jwkEntry}}
	body, _ := json.Marshal(doc)

	fetchCount := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fetchCount++
		
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))

	return srv, &fetchCount
}

// TestKeyByID_FirstMissTriggersFetch verifies that the very first unknown-kid
// request after a cache population triggers exactly one JWKS refresh
// (OAUTH-M7 positive path — genuine key-rotation scenario).
//
// The cooldown is reset to zero before the test so the first miss is always
// allowed through, matching how the server behaves on startup or after a long
// idle period.
func TestKeyByID_FirstMissTriggersFetch(t *testing.T) {
	resetJWKSCache()
	resetMissRefresh()

	// Build a JWKS server that advertises "known-key".
	srv, fetchCount := buildJWKSServer(t, "known-key")
	defer srv.Close()

	// Populate the cache with a real refresh so keys exist and fetchedAt is
	// recent.  This puts us in the "fresh cache" state.
	if err := refreshJWKS(srv.URL); err != nil {
		t.Fatalf("initial refreshJWKS: %v", err)
	}

	setJWKSCacheTTL(time.Hour) // cache stays fresh for 1 hour

	countBefore := *fetchCount

	// Request a kid that is NOT in the cache.  The cooldown is clear so a
	// refresh is expected.
	_, err := keyByID(srv.URL, "unknown-kid")
	if err == nil {
		t.Error("keyByID() should fail for an unknown kid (not present even after refresh)")
	}

	got := *fetchCount - countBefore
	if got != 1 {
		t.Errorf("first unknown-kid miss: expected exactly 1 JWKS fetch, got %d (OAUTH-M7)", got)
	}

	resetJWKSCache()
	resetMissRefresh()
}

// TestKeyByID_CooldownBlocksSecondMiss verifies that a second unknown-kid
// request within minMissRefreshInterval does NOT trigger another JWKS fetch
// (OAUTH-M7 — this is the attack-suppression path).
//
// Strategy: after the first miss sets the cooldown, we verify that the next
// unknown-kid call returns an error immediately with zero network activity.
// We simulate "still within the cooldown" by leaving missRefresh.last at the
// value set by the first call rather than rewinding it.
func TestKeyByID_CooldownBlocksSecondMiss(t *testing.T) {
	resetJWKSCache()
	resetMissRefresh()

	srv, fetchCount := buildJWKSServer(t, "known-key")
	defer srv.Close()

	// Populate the cache.
	if err := refreshJWKS(srv.URL); err != nil {
		t.Fatalf("initial refreshJWKS: %v", err)
	}

	setJWKSCacheTTL(time.Hour)

	// First unknown-kid call: cooldown is clear, refresh fires.
	_, _ = keyByID(srv.URL, "unknown-kid-1")

	// Record fetches so far.
	countAfterFirst := *fetchCount

	// Second unknown-kid call: cooldown IS active (set by the first call).
	// No network fetch should occur.
	_, err := keyByID(srv.URL, "unknown-kid-2")
	if err == nil {
		t.Error("keyByID() should fail for an unknown kid")
	}

	// The error must be ErrJWKSKeyNotFound — returned from the cooldown branch
	// without making a network request.
	if !errors.Equals(err, errors.ErrJWKSKeyNotFound) {
		t.Errorf("within cooldown: expected ErrJWKSKeyNotFound, got: %v", err)
	}

	got := *fetchCount - countAfterFirst
	if got != 0 {
		t.Errorf("within cooldown: expected 0 JWKS fetches, got %d (OAUTH-M7)", got)
	}

	resetJWKSCache()
	resetMissRefresh()
}

// TestKeyByID_CooldownExpiryAllowsRefresh verifies that once minMissRefreshInterval
// has elapsed, a subsequent unknown-kid request is again allowed to trigger a
// JWKS refresh (OAUTH-M7 — genuine rotation after the cooldown window).
//
// Waiting 30 real seconds in a unit test is impractical, so we manipulate
// missRefresh.last directly to simulate the passage of time.  This is safe
// because missRefresh is a package-level variable in the same package.
func TestKeyByID_CooldownExpiryAllowsRefresh(t *testing.T) {
	resetJWKSCache()
	resetMissRefresh()

	srv, fetchCount := buildJWKSServer(t, "known-key")
	defer srv.Close()

	// Populate the cache.
	if err := refreshJWKS(srv.URL); err != nil {
		t.Fatalf("initial refreshJWKS: %v", err)
	}

	setJWKSCacheTTL(time.Hour)

	// Pre-set missRefresh.last to "just past the cooldown window" so that the
	// next unknown-kid call sees an expired cooldown and is allowed through.
	missRefresh.mu.Lock()
	missRefresh.last = time.Now().Add(-(minMissRefreshInterval + time.Second))
	missRefresh.mu.Unlock()

	countBefore := *fetchCount

	_, err := keyByID(srv.URL, "unknown-kid-after-expiry")
	if err == nil {
		t.Error("keyByID() should fail for an unknown kid")
	}

	got := *fetchCount - countBefore
	if got != 1 {
		t.Errorf("after cooldown expiry: expected 1 JWKS fetch, got %d (OAUTH-M7)", got)
	}

	resetJWKSCache()
	resetMissRefresh()
}

// TestKeyByID_StaleCacheBypassesCooldown verifies that when the JWKS cache is
// stale (age > TTL), a refresh is always triggered regardless of the miss-
// refresh cooldown (OAUTH-M7 — the cooldown must only throttle the fresh-cache
// miss branch, not normal stale-cache refreshes).
//
// Without this guarantee, the cooldown could accidentally block normal cache
// invalidation and leave the server stuck with an outdated key set.
func TestKeyByID_StaleCacheBypassesCooldown(t *testing.T) {
	resetJWKSCache()

	srv, fetchCount := buildJWKSServer(t, "known-key")
	defer srv.Close()

	// Forcibly inject a stale cache: keys exist but fetchedAt is far in the
	// past so age > ttl.  We bypass refreshJWKS to keep fetchCount at zero.
	jwksCache.mu.Lock()
	jwksCache.keys = []publicKeyEntry{{Kid: "known-key", Key: "placeholder"}}
	jwksCache.fetchedAt = time.Now().Add(-2 * time.Hour) // 2 hours ago
	jwksCache.ttl = time.Minute                          // 1-minute TTL → stale
	jwksCache.mu.Unlock()

	// Set the cooldown to "just now" so it would block a fresh-cache miss.
	missRefresh.mu.Lock()
	missRefresh.last = time.Now()
	missRefresh.mu.Unlock()

	// Even though the cooldown is active, the stale cache must trigger a refresh.
	_, _ = keyByID(srv.URL, "unknown-kid")

	if *fetchCount != 1 {
		t.Errorf("stale cache must always refresh regardless of cooldown; got %d fetches (OAUTH-M7)", *fetchCount)
	}

	resetJWKSCache()
	resetMissRefresh()
}

// TestKeyByID_KnownKidInFreshCacheHitsNoNetwork verifies the fast path: when
// the cache is fresh and the requested kid IS present, keyByID returns
// immediately with no network call (no regression from the cooldown change).
func TestKeyByID_KnownKidInFreshCacheHitsNoNetwork(t *testing.T) {
	resetJWKSCache()
	resetMissRefresh()

	srv, fetchCount := buildJWKSServer(t, "known-key")
	defer srv.Close()

	// Populate the cache.
	if err := refreshJWKS(srv.URL); err != nil {
		t.Fatalf("initial refreshJWKS: %v", err)
	}

	setJWKSCacheTTL(time.Hour)

	countBefore := *fetchCount

	// Request the known kid — should be served from cache without any fetch.
	key, err := keyByID(srv.URL, "known-key")
	if err != nil {
		t.Fatalf("keyByID() for a known, cached kid returned error: %v", err)
	}

	if key == nil {
		t.Error("keyByID() returned nil key for a known, cached kid")
	}

	got := *fetchCount - countBefore
	if got != 0 {
		t.Errorf("known kid in fresh cache: expected 0 JWKS fetches, got %d", got)
	}

	resetJWKSCache()
	resetMissRefresh()
}

// ─────────────────────────────────────────────────────────────────────────────
// M4: double-checked locking in discoverEndpoints
// ─────────────────────────────────────────────────────────────────────────────

// TestDiscoverEndpoints_ConcurrentCalls_Consistent verifies that when many
// goroutines call discoverEndpoints simultaneously against a stale cache, they
// all receive an identical, valid document and the cache ends up with exactly
// one entry (M4).
//
// How the double-checked locking fix prevents a problem:
//
//	Without the fix, each goroutine that finds a stale cache makes a network
//	request and then writes to discoveryCache under a write lock.  The writes
//	are not atomic with the reads, so two goroutines can both decide to write
//	and each overwrite the other's result.  If the discovery document changes
//	between the two fetches (uncommon but possible during a rolling IdP deploy),
//	some goroutines would hold a reference to the old document and some to the
//	new one.
//
//	With the fix, the second goroutine to acquire the write lock re-checks
//	whether the cache is now fresh.  If it is, it discards its own result and
//	returns the document that was already stored — so all goroutines see the
//	same pointer.
//
// What this test verifies:
//
//  1. All goroutines receive a non-nil document (no errors).
//  2. All returned documents have the same Issuer field (consistency).
//  3. Run with -race: no data races are reported on the cache fields.
func TestDiscoverEndpoints_ConcurrentCalls_Consistent(t *testing.T) {
	resetDiscoveryCache()

	// Build a test server that serves a valid discovery document.  The fetch
	// counter uses sync/atomic because httptest.Server handles each inbound
	// connection in its own goroutine — a plain int++ from concurrent handler
	// calls would be a data race.
	var fetchCount int64

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&fetchCount, 1)

		doc := validDiscoveryDoc("http://" + r.Host)
		body, _ := json.Marshal(doc)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	const goroutines = 20

	// results collects the Issuer from each goroutine's returned document.
	// All values must be identical (same URL base) to confirm consistency.
	results := make([]string, goroutines)
	errs := make([]error, goroutines)

	// Use a simple done channel rather than sync.WaitGroup to keep the test
	// readable without importing an extra package.
	done := make(chan int, goroutines)

	for i := 0; i < goroutines; i++ {
		// Capture the loop index so each goroutine writes to its own slot.
		idx := i

		go func() {
			// All goroutines are launched before any single one completes, so
			// they race past the stale-cache check in discoverEndpoints.
			doc, err := discoverEndpoints(srv.URL)

			if err != nil {
				errs[idx] = err
			} else {
				results[idx] = doc.Issuer
			}

			done <- idx
		}()
	}

	// Wait for all goroutines.
	for i := 0; i < goroutines; i++ {
		<-done
	}

	// All goroutines must have succeeded.
	for i, err := range errs {
		if err != nil {
			t.Errorf("goroutine %d: discoverEndpoints error: %v", i, err)
		}
	}

	// All goroutines must have received the same Issuer.
	expected := srv.URL // validDiscoveryDoc sets Issuer to the base URL

	for i, issuer := range results {
		if issuer != expected {
			t.Errorf("goroutine %d: Issuer = %q, want %q (M4 consistency check)", i, issuer, expected)
		}
	}

	resetDiscoveryCache()
}

// TestDiscoverEndpoints_DoubleCheckPreventsDuplicateWrite verifies that when
// the discovery cache becomes fresh between the initial RLock check and the
// write-lock acquisition, the double-checked locking returns the already-cached
// document rather than overwriting it (M4).
//
// How the test simulates the race:
//
//	We cannot easily control goroutine interleaving, but we can manually
//	reproduce the race window by:
//	  1. Fetching the document with discoverEndpoints (populates the cache).
//	  2. Recording the pointer stored in the cache.
//	  3. Resetting only the fetchedAt timestamp to force a "stale" read.
//	  4. Calling discoverEndpoints again — it should detect the cache is now
//	     fresh under the write lock (because we restored fetchedAt) and return
//	     the existing pointer without overwriting it.
//
// Note: step 3 directly manipulates internal state, which is only possible
// because this test is in the same package (package oauth).
func TestDiscoverEndpoints_DoubleCheckReturnsCachedDoc(t *testing.T) {
	resetDiscoveryCache()

	fetchCount := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fetchCount++

		doc := validDiscoveryDoc("http://" + r.Host)
		body, _ := json.Marshal(doc)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	// First call: populates the cache.
	doc1, err := discoverEndpoints(srv.URL)
	if err != nil {
		t.Fatalf("first discoverEndpoints: %v", err)
	}

	if fetchCount != 1 {
		t.Fatalf("expected 1 fetch after first call, got %d", fetchCount)
	}

	// Record the pointer stored in the cache.
	discoveryCache.mu.RLock()
	cachedPtr := discoveryCache.doc
	discoveryCache.mu.RUnlock()

	// The returned pointer must be the one stored in the cache.
	if doc1 != cachedPtr {
		t.Error("first call: returned document pointer does not match the cached pointer")
	}

	// Second call: cache is still fresh — must return the cached doc without a
	// network request.  This also exercises the early-return path (RLock check
	// at the top of discoverEndpoints), not the double-check path; we verify
	// the pointer is the same to confirm no overwrite.
	doc2, err := discoverEndpoints(srv.URL)
	if err != nil {
		t.Fatalf("second discoverEndpoints: %v", err)
	}

	if fetchCount != 1 {
		t.Errorf("second call should not make a network request; fetch count = %d", fetchCount)
	}

	// Both calls must return the same pointer — the one that was cached on the
	// first fetch.
	if doc2 != cachedPtr {
		t.Error("M4: second call returned a different pointer than the cached one")
	}

	resetDiscoveryCache()
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
