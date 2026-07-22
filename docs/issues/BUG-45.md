# BUG-45 — An unrecovered `panic()` inside a goroutine does not stop the main program

**Severity:** MEDIUM

**Description:**  
In real Go, an unrecovered panic in *any* goroutine terminates the whole process
immediately — this is one of Go's best-known concurrency behaviors, and is more severe than
an ordinary returned/propagated error, not less. In Ego, an unrecovered `panic()` inside a
goroutine prints a panic message but does **not** stop the main program, which continues
running to completion (or times out). By contrast, an ordinary runtime error (e.g. division
by zero) inside a goroutine *does* correctly abort the main program almost immediately. This
divergence is not listed in `docs/LANGUAGE.md`'s "Key differences from Go" table or the
Threads section.

**Reproducer A** (panic — main is not stopped):

```go
func worker(done chan) {
    done <- true
    panic("goroutine panic")
}

func main() {
    done := make(chan, 1)
    go worker(done)
    _ = <-done
    count := 0
    for i := 0; i < 50000000; i++ {
        count = count + 1
    }
    fmt.Println("main finished loop, count =", count)
}
```

**Actual output:** prints `panic: goroutine panic` plus call frames, then runs the entire
50,000,000-iteration loop to completion (or times out under `--timeout`) — the loop is
never aborted regardless of how much wall-clock time is given.

**Reproducer B** (ordinary runtime error, for comparison — identical structure, but
`worker` does `x := 0; y := 5 / x` instead of `panic(...)`):

**Actual output:** `Error: at worker(line 6), division by zero` /
`Error: terminated with errors`, exit 1 — main's loop is correctly aborted almost
immediately.

**Expected output:**

An unrecovered `panic()` in a goroutine should be at least as fatal to the whole program as
an ordinary propagated runtime error — not silently swallowed.

**Notes:**  
Root cause: `internal/language/bytecode/panic.go`'s `unwindPanic()` returns
`errors.ErrStop` for an unrecovered top-level panic — the same sentinel used for a
goroutine's normal, successful completion.
`internal/language/bytecode/goroutine.go:176` explicitly excludes `ErrStop` from
propagating to the parent context
(`if err != nil && !err.Is(errors.ErrStop)`), so the fatal-panic path and the
falls-off-the-end-normally path are indistinguishable to the parent, and
`parentCtx.running.Store(false)` (`goroutine.go:186`) is never reached for the panic case.

**Resolution (July 2026):**  
Fixed at the root: `unwindPanic()`'s final, unrecovered-panic fallback in
`internal/language/bytecode/panic.go` now returns a new, distinguishable error —
`errors.ErrPanicUnhandled` (key `panic.unhandled`, added to `internal/errors/messages.go`
and localized in all three `messages_*.txt` files) — instead of `errors.ErrStop`. The panic
message and call-frame dump this function already printed are unchanged; only the *returned
error value* changed.

No other file needed to change. Every caller that treats an unrecovered panic differently
from a normal stop already does so purely by checking `errors.Equals(err, errors.ErrStop)`
(or `err.Is(errors.ErrStop)`); once the fallback stopped returning that literal sentinel,
each of the following started doing the right thing automatically:

- **`internal/language/bytecode/goroutine.go`'s `GoRoutine`** — `if err != nil &&
  !err.Is(errors.ErrStop)` now correctly recognizes an unrecovered panic as a real error, sets
  `parentCtx.goErr`, and calls `parentCtx.running.Store(false)`, stopping the program that
  launched the goroutine — this was the reported bug.
- **`internal/commands/run.go`'s `runCompiledCode`** — its own `errors.Equals(err,
  errors.ErrStop)` check no longer swallows an unrecovered *top-level* (non-goroutine) panic
  either. This was a closely related defect beyond the literal bug report, found while
  verifying the fix: `ego run` on a program whose `main()` panics without recovering
  previously printed the panic message but still exited **0**; it now exits non-zero, matching
  real Go (`go run` exits 2 on an unrecovered panic).
- **`internal/language/bytecode/defer.go`'s `invokeDeferredStatements` /
  `invokePanicDefers`** — both already special-case `errors.ErrStop` as "the deferred call's
  own mini-context finished normally, don't propagate." A deferred function that itself
  panics without recovering now correctly propagates as a real error too, instead of being
  silently absorbed — the same class of bug, one level further down the call stack.

**Tests added:**

- `internal/language/bytecode/panic_test.go` (new file):
  `Test_unwindPanic_UnrecoveredAtTopLevel_ReturnsPanicUnhandledNotStop` (the core fix, tested
  in isolation) and `Test_unwindPanic_RecoveredAtTopLevel_StillReturnsErrStop` (confirms the
  *other*, unrelated `errors.ErrStop` return in the same function — a panic that a deferred
  `recover()` successfully catches at the outermost frame — was correctly left untouched).
- `internal/language/bytecode/goroutine_test.go` (new file):
  `Test_GoRoutine_UnrecoveredPanic_StopsParentContext`,
  `Test_GoRoutine_OrdinaryError_StopsParentContext` (regression guard for the behavior that
  was already correct), and `Test_GoRoutine_NormalCompletion_DoesNotStopParentContext`
  (regression guard for the ordinary success path).
- `internal/language/compiler/goroutine_test.go` (new file):
  `TestBUG45UnrecoveredGoroutinePanicStopsMainProgram`, an end-to-end test using the exact
  reproducer above, compiled and run for real via `RunString`/`runWithTimeout` (the same
  timeout-guarded helper introduced for BUG-25), confirming the 50,000,000-iteration loop
  is aborted rather than completing; and
  `TestBUG45OrdinaryGoroutineErrorStillStopsMainProgram` as a regression guard.
- `examples/panic.ego` (new file): a runnable, heavily-commented sample program demonstrating
  both a recovered goroutine panic (no effect on `main()`) and an unrecovered one (halts the
  whole program before its own long-running loop can finish), added at the user's request in
  place of an Ego-level `@test` for this scenario — an unrecovered panic's whole *point* is
  that it stops the running program, which makes it unsuitable to exercise from inside the
  shared `ego test tests/` suite (it would stop the test runner's own shared top-level context
  partway through, silently skipping every test that runs after it in file order).

