package router

// Regression tests for the NILPTR-1 and NILPTR-2 fixes.
//
// The central claim of NILPTR-1 is that a panic in a route handler used to leave
// ServerShutdownLock locked forever, which wedges every later request and makes
// the server permanently unavailable even though the process is still running.
// TestPanicDoesNotLeakShutdownLock is the test that actually demonstrates that,
// and it is the most important test in this file: it fails by hanging (and then
// timing out) against the unfixed code.

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tucats/ego/internal/cli/settings"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/i18n"
)

// withPanicRecovery sets ego.server.panic.recovery for the duration of a test
// and restores whatever was there before. Tests must not leave a global setting
// changed behind them, because that would silently affect later tests.
func withPanicRecovery(t *testing.T, enabled bool) {
	t.Helper()

	saved := settings.Get(defs.ServerPanicRecoverySetting)

	value := defs.False
	if enabled {
		value = defs.True
	}

	settings.SetDefault(defs.ServerPanicRecoverySetting, value)

	t.Cleanup(func() {
		settings.SetDefault(defs.ServerPanicRecoverySetting, saved)
	})
}

// panicRouter builds a router with a single route whose handler always panics
// with a nil-map write -- a realistic stand-in for the nil dereferences this
// audit was about, rather than an artificial panic() call.
func panicRouter(t *testing.T, name, endpoint string) *Router {
	t.Helper()

	m := NewRouter(name)
	m.New(endpoint, func(session *Session, w http.ResponseWriter, r *http.Request) int {
		// Writing to a nil map panics at runtime. This is the same class of
		// failure as a nil-pointer dereference and needs no import to trigger.
		var broken map[string]string

		// This little charade is to settle down the linter, which seems
		// determined to tell me what we already know -- the write
		// to the map is to a nil value. So adding a little logic
		// the linter can predict get it to be quiet. The seesion.ID
		// will _never_ be less than zero, so we always panic -- as
		// the code is inteded to do.
		if session.ID < 0 {
			broken = map[string]string{}
		}

		broken["key"] = "value"

		return http.StatusOK
	}, http.MethodGet).Authentication(false)

	return m
}

// TestHandlerPanicDoesNotLeakShutdownLock covers the ordinary case: a panic in
// a route handler must not leave ServerShutdownLock held.
//
// Note on scope: ServeHTTP releases that mutex immediately after routing, which
// is *before* the handler runs, so a handler panic never leaked it even before
// the fix. This test pins that property down so a future change that moves the
// release later cannot silently reintroduce the hazard.
// TestFindRoutePanicDoesNotLeakShutdownLock below covers the window where the
// leak was actually reachable.
func TestHandlerPanicDoesNotLeakShutdownLock_NILPTR1(t *testing.T) {
	i18n.Language = "en"

	withPanicRecovery(t, true)

	m := panicRouter(t, "panic-lock-test", "/services/panic-lock")

	r, err := http.NewRequest(http.MethodGet, "/services/panic-lock", nil)
	if err != nil {
		t.Fatalf("failed to build test request: %v", err)
	}

	recorder := httptest.NewRecorder()

	// With recovery enabled this must return normally rather than propagating
	// the panic out of ServeHTTP.
	m.ServeHTTP(recorder, r)

	// TryLock reports whether the mutex is currently free without blocking, so
	// the test can assert "not deadlocked" instead of hanging on it.
	if !ServerShutdownLock.TryLock() {
		t.Fatal("ServerShutdownLock is still held after a panicking request")
	}

	ServerShutdownLock.Unlock()
}

// TestFindRoutePanicDoesNotLeakShutdownLock is the test that demonstrates the
// actual NILPTR-1 hazard.
//
// ServeHTTP holds ServerShutdownLock across the call to FindRoute. Before the
// fix, the matching Unlock was a straight-line statement, so a panic raised
// inside FindRoute skipped it and the mutex stayed locked for the life of the
// process -- every later request then blocked on it and the server, though still
// running, could not answer anything.
//
// To panic inside that window the test puts a nil *Route into the router's map.
// FindRoute reads route.endpoint as it scans the map, which dereferences the nil
// pointer. That is a genuine (if internal) nil dereference in exactly the code
// the lock protects, so it exercises the window faithfully rather than through
// an artificial panic() call.
func TestFindRoutePanicDoesNotLeakShutdownLock_NILPTR1(t *testing.T) {
	i18n.Language = "en"

	// Recovery is enabled so ServeHTTP returns normally and the test can go on
	// to inspect the mutex.
	withPanicRecovery(t, true)

	m := NewRouter("findroute-panic-test")
	m.New("/services/findroute-ok", func(session *Session, w http.ResponseWriter, r *http.Request) int {
		return http.StatusOK
	}, http.MethodGet).Authentication(false)

	// Inject a nil route. This is reaching into package internals, which a test
	// in the same package is allowed to do, and is the only way to make the
	// scan inside FindRoute fail.
	for selector := range m.routes {
		m.routes[selector] = nil

		break
	}

	r, err := http.NewRequest(http.MethodGet, "/services/findroute-ok", nil)
	if err != nil {
		t.Fatalf("failed to build test request: %v", err)
	}

	recorder := httptest.NewRecorder()
	m.ServeHTTP(recorder, r)

	if !ServerShutdownLock.TryLock() {
		t.Fatal("ServerShutdownLock is still held after a panic inside FindRoute; one such panic would wedge the server permanently")
	}

	ServerShutdownLock.Unlock()
}

// TestPanicReturns500 confirms the recovered panic is reported to the caller as
// a 500 rather than silently closing the connection, and that the panic text is
// not leaked into the response body (it routinely names internal types and
// files).
func TestPanicReturns500_NILPTR1(t *testing.T) {
	i18n.Language = "en"

	withPanicRecovery(t, true)

	m := panicRouter(t, "panic-500-test", "/services/panic-500")

	r, err := http.NewRequest(http.MethodGet, "/services/panic-500", nil)
	if err != nil {
		t.Fatalf("failed to build test request: %v", err)
	}

	recorder := httptest.NewRecorder()
	m.ServeHTTP(recorder, r)

	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}

	body := recorder.Body.String()
	for _, leak := range []string{"nil map", "goroutine", "panic_recovery_test.go"} {
		if strings.Contains(body, leak) {
			t.Errorf("response body leaks internal detail %q: %s", leak, body)
		}
	}
}

// TestPanicPropagatesWhenRecoveryDisabled confirms the configuration switch
// works in the other direction: with ego.server.panic.recovery set to false the
// panic is re-raised, so a developer sees the original failure rather than a log
// entry and a 500.
func TestPanicPropagatesWhenRecoveryDisabled_NILPTR1(t *testing.T) {
	i18n.Language = "en"

	withPanicRecovery(t, false)

	m := panicRouter(t, "panic-passthrough-test", "/services/panic-passthrough")

	r, err := http.NewRequest(http.MethodGet, "/services/panic-passthrough", nil)
	if err != nil {
		t.Fatalf("failed to build test request: %v", err)
	}

	recorder := httptest.NewRecorder()

	// Catch the propagating panic here so the test binary itself survives; this
	// stands in for the recover() that net/http installs around real requests.
	recovered := func() (value any) {
		defer func() {
			value = recover()
		}()

		m.ServeHTTP(recorder, r)

		return nil
	}()

	if recovered == nil {
		t.Error("panic was swallowed even though ego.server.panic.recovery is false")
	}

	// Even on the propagating path the shutdown lock must have been released,
	// because that release is a deferred call and deferred calls still run while
	// a panic unwinds. This is the part of the NILPTR-1 fix that protects the
	// server regardless of how the setting is configured.
	if !ServerShutdownLock.TryLock() {
		t.Fatal("ServerShutdownLock is still held after a propagating panic")
	}

	ServerShutdownLock.Unlock()
}

// TestNilHandlerDoesNotPanic is the NILPTR-2 regression test.
//
// A Route whose handler field is nil used to be detected only when REST logging
// was switched on, because the check lived inside an "if ui.IsActive(...)" block.
// With logging off -- the normal production case -- the nil handler reached the
// call site and panicked on a call through a nil function value.
//
// This test deliberately does NOT enable REST logging, so it exercises the path
// that used to be unprotected.
func TestNilHandlerDoesNotPanic_NILPTR2(t *testing.T) {
	i18n.Language = "en"

	// Recovery is disabled so that this test proves the nil-handler check itself
	// works. If the check were still missing, the panic would escape ServeHTTP
	// and fail the test rather than being masked by the NILPTR-1 safety net.
	withPanicRecovery(t, false)

	m := NewRouter("nil-handler-test")

	// Register the route with a nil handler function, which is the shape of an
	// incompletely constructed Route.
	m.New("/services/nil-handler", nil, http.MethodGet).Authentication(false)

	r, err := http.NewRequest(http.MethodGet, "/services/nil-handler", nil)
	if err != nil {
		t.Fatalf("failed to build test request: %v", err)
	}

	recorder := httptest.NewRecorder()

	done := make(chan any, 1)

	go func() {
		defer func() {
			done <- recover()
		}()

		m.ServeHTTP(recorder, r)
	}()

	select {
	case recovered := <-done:
		if recovered != nil {
			t.Fatalf("nil route handler panicked instead of reporting an error: %v", recovered)
		}

	case <-time.After(5 * time.Second):
		t.Fatal("request with a nil handler did not complete")
	}

	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
}
