package router

import (
	"os"
	"time"

	"github.com/tucats/ego/internal/cli/ui"
)

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
func RequestShutdown() {
	ServerShutdownLock.Lock()

	go func() {
		time.Sleep(1 * time.Second)
		ui.Log(ui.ServerLogger, "server.shutdown", nil)
		os.Exit(0)
	}()
}
