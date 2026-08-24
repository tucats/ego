package tasks

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/tucats/ego/internal/cli/settings"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/router"
	"github.com/tucats/ego/internal/util"
)

// useTestRouter installs a fresh, empty router as router.ServerRouter for
// the duration of the test and restores whatever was there before
// afterward. Returns the new router so the test can register routes on it.
func useTestRouter(t *testing.T) *router.Router {
	t.Helper()

	original := router.ServerRouter
	r := router.NewRouter("tasks-dispatch-test")
	router.ServerRouter = r

	t.Cleanup(func() {
		router.ServerRouter = original
	})

	return r
}

func TestResolveTimeoutUsesDefaultWhenUnset(t *testing.T) {
	task := &Task{ID: "t1"}

	if got := resolveTimeout(task); got != defaultTaskTimeout {
		t.Errorf("resolveTimeout() = %v, want default %v", got, defaultTaskTimeout)
	}
}

func TestResolveTimeoutUsesTaskValue(t *testing.T) {
	task := &Task{ID: "t1", Timeout: "5m"}

	if got := resolveTimeout(task); got != 5*time.Minute {
		t.Errorf("resolveTimeout() = %v, want 5m", got)
	}
}

func TestResolveTimeoutFallsBackOnUnparseableTaskValue(t *testing.T) {
	task := &Task{ID: "t1", Timeout: "garbage"}

	if got := resolveTimeout(task); got != defaultTaskTimeout {
		t.Errorf("resolveTimeout() = %v, want default %v for an unparseable value", got, defaultTaskTimeout)
	}
}

func TestResolveTimeoutClampsToConfiguredMax(t *testing.T) {
	settings.SetDefault(defs.TasksMaxTimeoutSetting, "1m")
	t.Cleanup(func() { settings.DeleteDefault(defs.TasksMaxTimeoutSetting) })

	task := &Task{ID: "t1", Timeout: "10m"}

	if got := resolveTimeout(task); got != time.Minute {
		t.Errorf("resolveTimeout() = %v, want clamped to 1m", got)
	}
}

// testEchoHandler is registered on the test router at POST /test/echo. It
// reports back the authenticated identity, the raw query string, and the
// parsed request body, all nested under "echoed" so a task's "save" block
// (dot-notation path) can pull a value back out.
func testEchoHandler(session *router.Session, w http.ResponseWriter, r *http.Request) int {
	body, _ := io.ReadAll(r.Body)

	var parsedBody any

	_ = json.Unmarshal(body, &parsedBody)

	response := map[string]any{
		"echoed": map[string]any{
			"user":  session.User,
			"query": r.URL.RawQuery,
			"body":  parsedBody,
		},
	}

	b, _ := json.Marshal(response)

	w.Header().Set(defs.ContentTypeHeader, defs.JSONMediaType)
	w.WriteHeader(http.StatusCreated)
	_, _ = w.Write(b)

	return http.StatusCreated
}

func TestDispatchEndToEndSuccessAppliesSave(t *testing.T) {
	resetSaved(t)
	setSaved("SRC", "unit-test")
	setSaved("NAME", "world")

	r := useTestRouter(t)
	r.New("/test/echo", testEchoHandler, "POST").Parameter("src", util.StringParameterType)

	task := &Task{
		ID:         "t1",
		User:       defs.DefaultAdminUsername,
		Method:     "POST",
		Endpoint:   "/test/echo",
		Parameters: map[string]string{"src": "{{SRC}}"},
		Body:       json.RawMessage(`{"greeting": "hello {{NAME}}"}`),
		Status:     http.StatusCreated,
		Save:       map[string]string{"GREETING": "echoed.body.greeting", "CALLER": "echoed.user"},
	}

	status, success, _ := dispatch(task)

	if status != http.StatusCreated {
		t.Errorf("status = %d, want %d", status, http.StatusCreated)
	}

	if !success {
		t.Error("expected success to be true when status matches task.Status")
	}

	if got := substitute("{{GREETING}}"); got != "hello world" {
		t.Errorf("saved GREETING = %q, want %q (proves body substitution and save extraction both worked)", got, "hello world")
	}

	if got := substitute("{{CALLER}}"); got != defs.DefaultAdminUsername {
		t.Errorf("saved CALLER = %q, want %q (proves the minted token resolved to the right identity)", got, defs.DefaultAdminUsername)
	}
}

func TestDispatchPassingTestsKeepSuccess(t *testing.T) {
	resetSaved(t)

	r := useTestRouter(t)
	r.New("/test/echo", testEchoHandler, "POST")

	task := &Task{
		ID:       "t1a",
		User:     defs.DefaultAdminUsername,
		Method:   "POST",
		Endpoint: "/test/echo",
		Status:   http.StatusCreated,
		Tests: []Check{
			{Name: "caller is admin", Query: "echoed.user", Value: defs.DefaultAdminUsername},
		},
	}

	status, success, failedTest := dispatch(task)

	if status != http.StatusCreated || !success || failedTest != "" {
		t.Errorf("dispatch() = (%d, %v, %q), want (%d, true, \"\")", status, success, failedTest, http.StatusCreated)
	}
}

func TestDispatchFailingTestOverridesSuccessButSaveStillRuns(t *testing.T) {
	resetSaved(t)

	r := useTestRouter(t)
	r.New("/test/echo", testEchoHandler, "POST")

	task := &Task{
		ID:       "t1b",
		User:     defs.DefaultAdminUsername,
		Method:   "POST",
		Endpoint: "/test/echo",
		Status:   http.StatusCreated,
		Save:     map[string]string{"CALLER": "echoed.user"},
		Tests: []Check{
			{Name: "caller is somebody else", Query: "echoed.user", Value: "not-the-real-user"},
		},
	}

	status, success, failedTest := dispatch(task)

	if status != http.StatusCreated {
		t.Errorf("status = %d, want %d (the HTTP status itself still matched)", status, http.StatusCreated)
	}

	if success {
		t.Error("expected success to be false: the status matched, but the test should have failed")
	}

	if failedTest != "caller is somebody else" {
		t.Errorf("failedTest = %q, want %q", failedTest, "caller is somebody else")
	}

	if got := substitute("{{CALLER}}"); got != defs.DefaultAdminUsername {
		t.Errorf("save = %q, want %q (save should still run even though a test later failed)", got, defs.DefaultAdminUsername)
	}
}

func TestDispatchStatusMismatchSkipsTests(t *testing.T) {
	resetSaved(t)

	r := useTestRouter(t)
	r.New("/test/echo", testEchoHandler, "POST")

	task := &Task{
		ID:       "t1c",
		User:     defs.DefaultAdminUsername,
		Method:   "POST",
		Endpoint: "/test/echo",
		Status:   http.StatusTeapot, // handler always returns 201, never this
		Tests: []Check{
			{Name: "would fail if ever evaluated", Query: "no.such.field", Operator: "exists"},
		},
	}

	status, success, failedTest := dispatch(task)

	if status != http.StatusCreated {
		t.Errorf("status = %d, want the handler's actual %d", status, http.StatusCreated)
	}

	if success {
		t.Error("expected success to be false due to the status mismatch")
	}

	if failedTest != "" {
		t.Errorf("failedTest = %q, want \"\" (a status mismatch is the failure reason, not a test)", failedTest)
	}
}

func TestDispatchStatusMismatchSkipsSave(t *testing.T) {
	resetSaved(t)

	r := useTestRouter(t)
	r.New("/test/echo", testEchoHandler, "POST")

	task := &Task{
		ID:       "t2",
		User:     defs.DefaultAdminUsername,
		Method:   "POST",
		Endpoint: "/test/echo",
		Status:   http.StatusTeapot, // handler always returns 201, never this
		Save:     map[string]string{"SHOULD_NOT_EXIST": "echoed.user"},
	}

	status, success, _ := dispatch(task)

	if status != http.StatusCreated {
		t.Errorf("status = %d, want the handler's actual %d", status, http.StatusCreated)
	}

	if success {
		t.Error("expected success to be false when the actual status doesn't match task.Status")
	}

	if got := substitute("{{SHOULD_NOT_EXIST}}"); got != "{{SHOULD_NOT_EXIST}}" {
		t.Errorf("save block ran despite a status mismatch; got %q", got)
	}
}

func TestDispatchQueryParametersAreSubstitutedAndSent(t *testing.T) {
	resetSaved(t)
	setSaved("SRC", "the-value")

	r := useTestRouter(t)
	r.New("/test/echo", testEchoHandler, "POST").Parameter("src", util.StringParameterType)

	task := &Task{
		ID:         "t3",
		User:       defs.DefaultAdminUsername,
		Method:     "POST",
		Endpoint:   "/test/echo",
		Parameters: map[string]string{"src": "{{SRC}}"},
		Status:     http.StatusCreated,
		Save:       map[string]string{"GOT_QUERY": "echoed.query"},
	}

	if _, success, _ := dispatch(task); !success {
		t.Fatal("expected dispatch to succeed")
	}

	if got := substitute("{{GOT_QUERY}}"); got != "src=the-value" {
		t.Errorf("query received by handler = %q, want %q", got, "src=the-value")
	}
}

func slowHandler(delay time.Duration) router.HandlerFunc {
	return func(session *router.Session, w http.ResponseWriter, r *http.Request) int {
		time.Sleep(delay)
		w.WriteHeader(http.StatusOK)

		return http.StatusOK
	}
}

func TestDispatchTimeoutIsReportedAsFailure(t *testing.T) {
	resetSaved(t)

	r := useTestRouter(t)
	r.New("/test/slow", slowHandler(300*time.Millisecond), "GET")

	task := &Task{
		ID:       "t4",
		User:     defs.DefaultAdminUsername,
		Method:   "GET",
		Endpoint: "/test/slow",
		Status:   http.StatusOK,
		Timeout:  "20ms",
	}

	start := time.Now()
	status, success, _ := dispatch(task)
	elapsed := time.Since(start)

	if status != 0 || success {
		t.Errorf("dispatch() = (%d, %v), want (0, false) for a timed-out call", status, success)
	}

	if elapsed > 250*time.Millisecond {
		t.Errorf("dispatch() took %v, expected it to return promptly at the ~20ms timeout rather than waiting for the 300ms handler", elapsed)
	}

	// Give the abandoned handler goroutine time to finish in the background
	// so it doesn't outlive the test (harmless, but keeps -race output clean
	// across the whole run).
	time.Sleep(350 * time.Millisecond)
}

func TestDispatchUnknownUserFailsGracefully(t *testing.T) {
	resetSaved(t)

	r := useTestRouter(t)
	r.New("/test/echo", testEchoHandler, "POST")

	task := &Task{
		ID:       "t5",
		User:     fmt.Sprintf("no-such-user-%d", time.Now().UnixNano()),
		Method:   "POST",
		Endpoint: "/test/echo",
		Status:   http.StatusCreated,
	}

	// A nonexistent user still gets a token (minting doesn't check the user
	// database) and the request still runs -- there's simply no local user
	// record, so permissions resolve to empty. Whether that yields the
	// handler's real response or a 403 depends on the target route's own
	// permission requirements; this route requires none, so the call still
	// goes through and this should not panic or hang.
	status, _, _ := dispatch(task)

	if status == 0 {
		t.Error("expected a real HTTP status even for an unknown user, not a token-mint failure")
	}
}
