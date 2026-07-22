# CODE-M2 — Global trace logger state mutated per-request without synchronization

**Affected file:** `server/admin/run.go:186` — `RunCodeHandler()`

```go
savedTrace := ui.IsActive(ui.TraceLogger)
ui.Active(ui.TraceLogger, req.Trace)
output, runErr := executeAdminEgo(session.ID, req.Code, req.Console, req.Trace, req.Session)
ui.Active(ui.TraceLogger, savedTrace)
```

**Description:**  
`ui.IsActive` and `ui.Active` operate on a single global logger-state map
shared across all goroutines. The read-modify-execute-restore sequence above is
not protected by any mutex. When two concurrent requests arrive with different
`Trace` values, one request can overwrite the other's saved state. This creates
two observable problems:

1. **Unintended trace exposure** — a request that did not ask for tracing may
   run with the trace logger enabled because a concurrent request enabled it
   after the first request saved `savedTrace = false`.
2. **Data race** — Go's race detector will flag concurrent reads and writes to
   the shared logger state as a data race (no synchronization).

Since the execution context already accepts a per-request trace flag via
`ctx.SetTrace(trace)`, the global mutation is unnecessary for controlling
per-execution tracing. The `ui.Active` calls are only needed if some code path
outside the context checks the global flag directly.

**Recommendation:**  
Remove the global `ui.Active` mutations from `RunCodeHandler`. Pass the trace
flag exclusively through the `bytecode.Context` (`ctx.SetTrace(req.Trace)`)
so each request controls its own tracing without touching shared state. If
global trace output is still required for some paths, protect the
save/restore pair with a dedicated mutex.

**Resolution (April 2026):**  
The save/restore of `ui.Active(ui.TraceLogger)` has been replaced with a
package-level `traceRunMu sync.Mutex`. When `req.Trace` is true, the handler
acquires the mutex, sets the logger active, and defers both the restore and the
unlock. Non-trace requests proceed without serialization. This eliminates the
data race while preserving global trace output for the bytecode run-loop log
messages that check `ui.IsActive(ui.TraceLogger)` directly.

