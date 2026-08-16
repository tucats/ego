package router

import (
	"os"
	"sync/atomic"
	"time"

	"github.com/tucats/ego/internal/cli/ui"
)

var RequestsActive atomic.Int64

// RequestShutdown is the single, sanctioned way for a handler to trigger an
// orderly shutdown of this server process.
//
// Shutdown must be requested by a handler calling this function directly --
// never inferred from an HTTP response status code (e.g. 503 Service
// Unavailable) or any other property of the response. A status code is
// visible to, and can be produced by, any handler for any reason; treating one
// as an implicit shutdown signal makes it too easy for an unrelated handler to
// accidentally (or maliciously) bring the whole server down. The only handler
// that should call RequestShutdown is DownHandler (or an Ego service
// overriding the /services/admin/down endpoint), after the router has already
// authenticated and authorized the caller for that specific operation.
//
// The shutdown itself is asynchronous. ServerShutdownLock is taken immediately
// -- and deliberately never released -- so that ServeHTTP's lock at the start
// of every request blocks forever, refusing any further requests. After a
// short delay, to allow the in-flight response to flush back to the caller,
// the process exits.
//
// The grace period lets the shutdown request specify a timeout to wait for
// in-flight requests to complete. The first polling interval is always
// 500ms, which allows the requesting client to receive the message that
// the shutdown request is in progress. Subsequent polling intervals are
// 200ms. The shutdown occurs when either the trace period is exhausted,
// or the in-fligth requests falls to zero.
func RequestShutdown(grace time.Duration) {
	ServerShutdownLock.Lock()

	startWaiting := time.Now()

	go func() {
		waiting := true
		pauseMs := 500 * time.Millisecond

		ui.Log(ui.ServerLogger, "server.shutdown.waiting", ui.A{
			"count": RequestsActive.Load(),
			"grace": grace.String(),
		})

		for waiting {
			time.Sleep(pauseMs)

			if RequestsActive.Load() <= 0 {
				waiting = false
			}

			if time.Since(startWaiting) > grace {
				waiting = false
			}

			pauseMs = 200 * time.Millisecond
		}

		ui.Log(ui.ServerLogger, "server.shutdown", nil)
		os.Exit(0)
	}()
}
