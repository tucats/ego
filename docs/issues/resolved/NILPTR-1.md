# NILPTR-1 — No panic recovery in the HTTP request dispatcher

**Affected function:** `(*Router).ServeHTTP`
**File:** `router/serve.go`
**Risk:** High — a panic anywhere in a request produced no response and no log
entry; a panic inside `FindRoute` stranded a mutex and made the server
permanently unresponsive
**Status: RESOLVED**

## NILPTR-1: Description

`ServeHTTP` had no `recover()` anywhere in it, and neither did anything it calls.

### What was, and was not, at risk

Go's `net/http` installs its own `recover()` at the connection level, so a
panicking handler never terminated the server process. That part was already
safe, and it is worth stating plainly because it bounds the rest of this issue.
Two real problems remained.

**1. No response and no log.** When a handler panicked, `net/http` closed the
connection and wrote its own `http: panic serving` line to the Go standard
logger. The client saw a dropped connection rather than an HTTP 500 — impossible
to distinguish from a network fault — and nothing appeared in the Ego server log
where an operator would look.

**2. A stranded mutex.** `ServeHTTP` acquires `ServerShutdownLock` on entry and
releases it after routing:

```go
ServerShutdownLock.Lock()

start := time.Now()
route, status := m.FindRoute(r.Method, r.URL.Path, true)
defer route.Unlock()

ServerShutdownLock.Unlock()
```

That `Unlock` was a plain statement. Go runs deferred calls while a panic
unwinds, but it does not run ordinary statements, so a panic inside `FindRoute`
skipped the release and left the mutex locked for the life of the process. Every
later request blocks on `Lock()` at the top of `ServeHTTP`. The server keeps
running, keeps accepting connections, and answers nothing — the failure mode this
audit was commissioned to find.

**Scope correction.** An earlier draft of this fix claimed any handler panic
stranded the lock. It does not: the release happens *before* the handler is
called, so the exposed window is only `FindRoute`. `FindRoute` is not trivial —
it iterates the route map and does string surgery on a caller-supplied URL path —
but the window is much narrower than "any handler".

## NILPTR-1: Fix

Two independent changes.

**The lock release is now deferred**, which protects the window regardless of how
panic recovery is configured:

```go
shutdownLockHeld := true

ServerShutdownLock.Lock()

defer func() {
    if shutdownLockHeld {
        ServerShutdownLock.Unlock()
    }
}()
...
shutdownLockHeld = false
ServerShutdownLock.Unlock()
```

The flag keeps the deferred release idempotent so the normal path can still drop
the lock early rather than holding it for the whole request.

**A last-resort panic handler** converts a panic into a logged HTTP 500:

```go
defer func() {
    if panicValue := recover(); panicValue != nil {
        reportRequestPanic(w, r, sessionID, panicValue)
    }
}()
```

It is registered first so that, deferred calls being last-in-first-out, it runs
*after* the unlock defers have already released their locks.

The panic value and a `debug.Stack()` trace are logged; the response body carries
only a generic message, since panic text routinely names internal types and files
and the caller may have provoked it deliberately.

### A Go subtlety this fix tripped over

The first attempt put the `recover()` inside `reportRequestPanic` and deferred a
closure that called it. That silently does nothing: Go only honors `recover()`
when it is called **directly** by the deferred function. One extra call frame and
it returns nil and the panic continues. The regression test caught this — the
panic still escaped `ServeHTTP` — which is why the `recover()` now lives in the
deferred closure itself and the value is passed to the reporting function.

## NILPTR-1: Configuration

Controlled by `ego.server.panic.recovery`, default **true**.

Set it to `false` during development to let panics propagate unmodified. The
deferred unlock protection stays in force either way.

The setting is read through `util.PanicRecoveryEnabled()`, which treats an absent
key as enabled. This matters: `settings.GetBool` returns false for a key that was
never configured, so reading the setting directly would have silently disabled
recovery for every configuration file written before the setting existed.

## NILPTR-1: Tests

- `TestFindRoutePanicDoesNotLeakShutdownLock_NILPTR1` — the one that demonstrates
  the hazard. It injects a nil `*Route` into the router map so `FindRoute` panics
  inside the lock window, then asserts the mutex is free. Against the pre-fix code
  it reports `ServerShutdownLock is still held after a panic inside FindRoute`.
- `TestHandlerPanicDoesNotLeakShutdownLock_NILPTR1` — pins down that a handler
  panic also leaves the lock free, so a future change that moves the release later
  cannot quietly reintroduce the hazard.
- `TestPanicReturns500_NILPTR1` — asserts the 500 status and that the panic text
  does not leak into the response body.
- `TestPanicPropagatesWhenRecoveryDisabled_NILPTR1` — asserts the configuration
  switch works in the other direction, and that the lock is released even on the
  propagating path.

Note that `FindRoute` was deliberately *not* given a nil-route guard. The state is
unreachable through the public API (`Router.New` never stores a nil route), and
leaving it unguarded is what allows the test above to exercise the lock window
with a genuine nil dereference rather than a synthetic `panic()` call.
