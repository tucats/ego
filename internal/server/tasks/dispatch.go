package tasks

import (
	"bytes"
	"io"
	"net/http/httptest"
	"net/url"
	"time"

	"github.com/tucats/ego/internal/cli/parser"
	"github.com/tucats/ego/internal/cli/settings"
	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/language/tokens"
	"github.com/tucats/ego/internal/router"
	"github.com/tucats/ego/internal/util"
)

// wire the real dispatcher into the scheduler. Kept as an init() rather
// than a direct assignment to dispatchFunc's declaration so scheduler.go
// has no compile-time dependency on this file (or its router/tokens
// imports) -- see scheduler.go's comment on dispatchFunc.
func init() {
	dispatchFunc = dispatch
}

// defaultTaskTimeout and defaultTaskMaxTimeout back-stop
// resolveTimeout when the corresponding config settings are missing or
// unparseable, which should only happen on a profile written before this
// feature existed.
const (
	defaultTaskTimeout    = 30 * time.Second
	defaultTaskMaxTimeout = time.Hour

	// tokenTTL is how long the minted bearer token remains valid. It only
	// has to outlive the moment between minting it and Authenticate()
	// checking it at the top of ServeHTTP -- essentially instantaneous,
	// since dispatch is in-process -- so this is independent of (and much
	// shorter than) how long the task's own timeout allows the call itself
	// to run.
	tokenTTL = "5m"
)

// dispatch performs one task's endpoint call in-process (no real network
// round trip) and reports the resulting HTTP status, whether it's
// considered successful, and -- only when a failing Check in Task.Tests is
// the reason -- that Check's Name. This is the function scheduler.go's
// dispatchFunc variable is wired to via this file's init().
func dispatch(task *Task) (status int, success bool, failedTest string) {
	timeout := resolveTimeout(task)

	type result struct {
		status     int
		success    bool
		failedTest string
	}

	done := make(chan result, 1)

	go func() {
		st, ok, ft := doDispatch(task)
		done <- result{st, ok, ft}
	}()

	select {
	case r := <-done:
		status, success, failedTest = r.status, r.success, r.failedTest
	case <-time.After(timeout):
		// The in-process call has no way to be preempted -- Go has no
		// cooperative cancellation hook into an arbitrary running
		// goroutine -- so a timed-out call is simply abandoned: its
		// result is discarded when it eventually finishes, and this run
		// is recorded as a failure now. See docs/internals/TASKS.md for
		// why this is a deliberate, accepted limitation of dispatching
		// in-process rather than over a real HTTP connection.
		ui.Log(tasksLogger, "tasks.run.timeout", ui.A{"id": task.ID, "timeout": timeout.String()})

		return 0, false, ""
	}

	ui.Log(tasksLogger, "tasks.run.complete", ui.A{
		"id":       task.ID,
		"status":   status,
		"expected": task.Status,
		"success":  success,
	})

	return status, success, failedTest
}

// doDispatch mints a token for the task's user, builds the request (with
// {{name}} substitution applied to the endpoint, parameters, and body),
// and runs it through the server's own router in-process. If the response
// status matches what the task expects, it applies the task's save block
// (regardless of what happens next) and then runs Task.Tests: any failing
// check downgrades success to false and is named in the third return
// value, even though the status itself matched.
func doDispatch(task *Task) (status int, success bool, failedTest string) {
	token, err := tokens.New(task.User, "", tokenTTL, defs.InstanceID, 0)
	if err != nil {
		ui.Log(tasksLogger, "tasks.run.token.error", ui.A{"id": task.ID, "error": err.Error()})

		return 0, false, ""
	}

	req := httptest.NewRequest(task.Method, buildURL(task), buildBody(task))
	req.Header.Set(defs.ContentTypeHeader, defs.JSONMediaType)
	req.Header.Set("Accept", defs.JSONMediaType)
	req.Header.Set("Authorization", defs.AuthScheme+token)
	// Identifies this call as task-originated (not an external client) in
	// the REST logger's header dump (rest.header.values, logging.go) when
	// REST logging is enabled.
	req.Header.Set("User-Agent", "Ego task "+task.ID)

	recorder := httptest.NewRecorder()

	router.ServerRouter.ServeHTTP(recorder, req)

	status = recorder.Code
	success = status == task.Status

	if success {
		body := recorder.Body.Bytes()

		applySave(task, body)

		if ok, name := runTests(task, body); !ok {
			success = false
			failedTest = name

			ui.Log(tasksLogger, "tasks.test.failed", ui.A{"id": task.ID, "test": name})
		}
	}

	return status, success, failedTest
}

// buildURL applies {{name}} substitution to the task's endpoint and
// appends its (also substituted) parameters as a query string.
func buildURL(task *Task) string {
	endpoint := substitute(task.Endpoint)

	if len(task.Parameters) == 0 {
		return endpoint
	}

	values := url.Values{}
	for name, value := range task.Parameters {
		values.Set(name, substitute(value))
	}

	return endpoint + "?" + values.Encode()
}

// buildBody applies {{name}} substitution to the task's raw JSON body. A
// task with no body returns nil, matching http.NewRequest's convention for
// a bodyless request.
func buildBody(task *Task) io.Reader {
	if len(task.Body) == 0 {
		return nil
	}

	return bytes.NewReader([]byte(substitute(string(task.Body))))
}

// applySave runs every dot-notation path in the task's save block against
// the response body and stores the extracted values in the global,
// cross-task substitution dictionary. A path that doesn't resolve is
// logged and skipped -- it doesn't fail the run, since the run already
// succeeded by the time save is considered.
func applySave(task *Task, body []byte) {
	if len(task.Save) == 0 {
		return
	}

	text := string(body)

	for name, path := range task.Save {
		value, err := parser.GetItem(text, path)
		if err != nil {
			ui.Log(tasksLogger, "tasks.save.error", ui.A{"id": task.ID, "name": name, "path": path, "error": err.Error()})

			continue
		}

		setSaved(name, value)
	}
}

// resolveTimeout returns the timeout to use for a task's endpoint call:
// the task's own value if present, clamped to the configured maximum,
// otherwise the configured default.
func resolveTimeout(task *Task) time.Duration {
	def := defaultTaskTimeout

	if parsed, err := util.ParseDuration(settings.Get(defs.TasksDefaultTimeoutSetting)); err == nil && parsed > 0 {
		def = parsed
	}

	max := defaultTaskMaxTimeout

	if parsed, err := util.ParseDuration(settings.Get(defs.TasksMaxTimeoutSetting)); err == nil && parsed > 0 {
		max = parsed
	}

	timeout := def

	if task.Timeout != "" {
		if parsed, err := util.ParseDuration(task.Timeout); err == nil {
			timeout = parsed
		}
	}

	if timeout > max {
		ui.Log(tasksLogger, "tasks.timeout.clamped", ui.A{"id": task.ID, "requested": task.Timeout, "max": max.String()})

		timeout = max
	}

	return timeout
}
