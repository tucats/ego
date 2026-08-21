package router

import (
	"os"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/dsns"
	"github.com/tucats/ego/internal/language/bytecode"
)

// This stores the start time when the server is started up, and is
// used to support the STATS log dump done at shutdown time.
var ServerStartTime *time.Time

// This is a counter of the number of in-fligth REST request at any
// one time. It is used to allow the shutdown operation to know when
// the last in-flight reqeust has been processed.
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

		if ui.IsActive(ui.StatsLogger) && ServerStartTime != nil {
			dumpStats(*ServerStartTime)
		}

		// Release the DSN service's own resources (open database connections,
		// for the SQL-backed provider) before the process exits. DSNService is
		// only nil if the server started without a DSN provider configured at
		// all, which InitializeFromURL/Initialize guarantee doesn't happen once
		// the server is actually running -- but the nil check costs nothing and
		// avoids a nil-interface panic if that assumption is ever wrong.
		if dsns.DSNService != nil {
			if err := dsns.DSNService.Close(); err != nil {
				ui.Log(ui.ServerLogger, "server.shutdown.dsn.error", ui.A{
					"error": err.Error(),
				})
			}
		}

		ui.Log(ui.ServerLogger, "server.shutdown", nil)
		os.Exit(0)
	}()
}

// dumpStats function is used to log various statistics about the application's runtime.
// It includes the execution elapsed time, bytecode instructions executed, maximum runtime stack size,
// memory currently on heap, objects currently on heap, total heap memory allocated, total system memory allocated,
// garbage collection cycles, and garbage collection percentage of CPU.
//
// This function takes a time.Time as a parameter, representing the start time of the application.
// It uses the ui package to log the statistics to the console if the StatsLogger is active.
//
// The function uses the runtime package to get memory statistics and the bytecode package to get
// bytecode execution statistics.
func dumpStats(start time.Time) {
	if ui.IsActive(ui.StatsLogger) {
		ui.Log(ui.StatsLogger, "stats.time", ui.A{"duration": time.Since(start).String()})

		if count := bytecode.InstructionsExecuted; count > 0 {
			ui.Log(ui.StatsLogger, "stats.instructions", ui.A{"count": count})
			ui.Log(ui.StatsLogger, "stats.max.stack", ui.A{"size": bytecode.MaxStackSize})
		}

		if bytecode.TotalDuration > 0.0 {
			ms := bytecode.TotalDuration * 1000
			ui.Log(ui.StatsLogger, "stats.time.test", ui.A{"duration": ms})
		}

		m := &runtime.MemStats{}
		runtime.ReadMemStats(m)

		ui.Log(ui.StatsLogger, "stats.memory.heap", ui.A{"size": m.Alloc})
		ui.Log(ui.StatsLogger, "stats.objects.heap", ui.A{"size": m.Mallocs - m.Frees})
		ui.Log(ui.StatsLogger, "stats.total.heap", ui.A{"size": m.TotalAlloc})
		ui.Log(ui.StatsLogger, "stats.system.heap", ui.A{"size": m.Sys})
		ui.Log(ui.StatsLogger, "stats.go.routines", ui.A{"count": runtime.NumGoroutine()})
		ui.Log(ui.StatsLogger, "stats.gc.count", ui.A{"count": m.NumGC})
		ui.Log(ui.StatsLogger, "stats.gc.cpu", ui.A{"cpu": m.GCCPUFraction})
	}
}
