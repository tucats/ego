# GORTNS-1 — `RunFromAddress` leaked one goroutine per bytecode execution

**Affected function:** `(*Context).RunFromAddress`
**File:** `language/bytecode/run.go`
**Risk:** High — unbounded goroutine and memory growth, reachable from ordinary
Ego code as well as from every HTTP service request
**Status: RESOLVED**

## GORTNS-1: Description

Every bytecode execution installed a SIGINT watcher so that Ctrl-C could stop a
running program:

```go
intChan := make(chan os.Signal, 1)
signal.Notify(intChan, os.Interrupt)

go func(c *Context) {
    sig := <-intChan
    ...
}(c)

// And, when done with this context, remove the SIGINT trap thread.
defer func() {
    signal.Stop(intChan)
}()
```

The comment on the cleanup says it removes the trap thread. It does not.
`signal.Stop` does exactly what its documentation promises — it stops the signal
package from *delivering* further signals to that channel — but it **does not
close the channel**. A receive on an open channel that nobody will ever send to
blocks forever, so the watcher goroutine never returned.

Because the closure captures `c`, each abandoned goroutine also kept an entire
`*Context` reachable — its symbol table, evaluation stack, and bytecode — so this
was a memory leak of unbounded size, not merely a goroutine count problem.

The signal registration itself was cleaned up correctly, so the leak is purely
the goroutine and what it retained.

### Why the impact was severe

`RunFromAddress` is not a once-per-program call. It runs once per *execution*, and
several callers execute repeatedly:

| Path | Leak rate |
|---|---|
| `services/service.go`, `services/child.go`, `admin/run.go` | one per HTTP service request, forever |
| `sort.Slice` / `sort.Stable` / `sort.Search` with an Ego comparator (`sort/slice.go`) | **one per comparison** |
| `fmt` formatting a value with an Ego `String()` method (`fmt/print.go`) | one per formatted value |
| `tables.Find` (`runtime/tables/find.go`) | one per call |
| Ego `go` statement | one extra, on top of the goroutine being launched |

The sort row is the worst: sorting a 300-element array with an Ego comparator does
roughly 2500 comparisons, so a single Ego statement leaked about 2500 goroutines.
Measured before the fix:

```
200 Run calls, fresh context each   -> 201 goroutines leaked
2500 Run calls, one reused context  -> 2501 goroutines leaked
```

## GORTNS-1: Fix

The standard Go "done channel" shutdown pattern. A second channel is created
purely as a broadcast signal, and the watcher waits on both channels with a
`select`, so whichever event happens first wins:

```go
done := make(chan struct{})

go func(c *Context) {
    select {
    case sig := <-intChan:
        // ... handle the interrupt as before ...

    case <-done:
        // RunFromAddress has finished; this watcher is no longer needed.
    }
}(c)

defer func() {
    signal.Stop(intChan)
    close(done)
}()
```

Nothing is ever sent on `done`; closing it is the entire message. That is the
idiomatic Go broadcast, because a receive from a closed channel returns
immediately, and does so for every receiver, without the sender needing to know
whether anyone is currently listening.

The ordering in the deferred function is deliberate: `signal.Stop` first, so no
further signal can be delivered to a channel that is about to have no reader,
then `close(done)` to release the watcher.

## GORTNS-1: Tests

`language/bytecode/goroutine_leak_test.go` counts live goroutines before and
after, which is the only way to assert "the goroutine went away":

- `TestRunDoesNotLeakGoroutinePerContext_GORTNS1` — 200 executions with a fresh
  context each time, the server's per-request pattern. Fails with
  `leaked 201 goroutines across 200 Run calls` against the unfixed code.
- `TestRunDoesNotLeakGoroutinePerCall_GORTNS1` — 2500 executions of one reused
  context, the `sort.Slice` and `fmt` pattern. Fails with
  `leaked 2500 goroutines across 2500 RunFromAddress calls` against the unfixed
  code.

Both allow a small tolerance for unrelated runtime bookkeeping goroutines; the
pre-fix deltas are orders of magnitude outside it.
