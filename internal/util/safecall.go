package util

import (
	"fmt"
	"runtime/debug"

	"github.com/tucats/ego/internal/cli/settings"
	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/defs"
)

// PanicRecoveryEnabled reports whether the server's last-resort panic handlers
// should absorb a panic (true) or let it propagate (false). It is the single
// place that interprets the ego.server.panic.recovery setting, so the HTTP
// request handler and the background-task wrapper cannot disagree.
//
// The setting defaults to ENABLED, and the check is deliberately written as
// "enabled unless explicitly set to false" rather than a plain GetBool.
//
// The reason matters. settings.GetBool returns false for a key that was never
// configured, because an unset key reads as an empty string. If this function
// used GetBool directly, then any configuration file written before this setting
// existed -- which is every existing deployment -- would report false and run
// with panic recovery switched OFF, silently doing the opposite of the
// documented default. Checking Exists first separates "explicitly set to false"
// from "not mentioned at all", and only the former disables recovery.
func PanicRecoveryEnabled() bool {
	if !settings.Exists(defs.ServerPanicRecoverySetting) {
		return true
	}

	return settings.GetBool(defs.ServerPanicRecoverySetting)
}

// SafeCall runs fn and converts any panic it produces into a log entry,
// returning true when fn completed normally and false when it panicked.
//
// NILPTR-6: why this exists, for anyone new to Go.
//
// Go's net/http package installs a recover() around each request, so a panic
// inside an HTTP handler does not kill the server process. That protection
// covers ONLY the goroutine http created to serve the request. A panic in any
// OTHER goroutine -- one the server started itself, such as a periodic cleanup
// task -- has nothing above it to catch it, and Go's rule for an unrecovered
// panic is to terminate the entire program.
//
// That makes a background goroutine a much more dangerous place to panic than a
// request handler. A nil map entry or a stale pointer in a once-a-minute cleanup
// task will take the whole server down, including every healthy connection, with
// no 500 response and no opportunity to retry.
//
// Wrapping the body of such a loop in SafeCall means one bad iteration is logged
// and skipped, and the loop lives to run again on the next tick.
//
// Use it for the body of a background loop, not for the loop itself:
//
//	for {
//	    time.Sleep(interval)
//	    util.SafeCall("prune login attempts", pruneLoginAttempts)
//	}
//
// Recovery honors ego.server.panic.recovery, the same setting that controls
// request-handler recovery. When that setting is false the panic is re-raised so
// it terminates the process with the original stack trace, which is what you
// usually want while debugging.
func SafeCall(name string, fn func()) (completed bool) {
	// The deferred function below runs whether fn returns normally or panics.
	// Naming the return value ("completed") lets that deferred function change
	// what SafeCall returns, which is how the false result is produced on a panic.
	defer func() {
		// recover() returns nil during a normal return, so this costs almost
		// nothing on the healthy path.
		panicValue := recover()
		if panicValue == nil {
			return
		}

		completed = false

		// Re-panicking inside a deferred function lets the original panic
		// continue up the stack untouched, so the process dies with the real
		// stack trace rather than a summarized log line.
		if !PanicRecoveryEnabled() {
			panic(panicValue)
		}

		// debug.Stack must be called here, while the panicking frames are still
		// on the stack; they are gone once this function returns.
		ui.Log(ui.ServerLogger, "server.panic.task", ui.A{
			"task":  name,
			"error": fmt.Sprintf("%v", panicValue),
		})

		ui.Log(ui.InternalLogger, "server.panic.stack", ui.A{
			"session": 0,
			"stack":   string(debug.Stack()),
		})
	}()

	fn()

	return true
}
