# BUG-27 — Misusing `sync.WaitGroup` crashes the entire `ego` process with a raw Go panic

**Severity:** CRITICAL

**Description:**  
Calling `Done()` more times than `Add()` on a `sync.WaitGroup` produces an unrecoverable Go
runtime panic that crashes the whole `ego` process (not just the running program), printing
a full Go stack trace that discloses internal file paths. It is not catchable with
`try`/`catch`.

**Reproducer:**

```go
import "sync"

func main() {
    var wg sync.WaitGroup
    wg.Add(1)
    wg.Done()
    wg.Done()
    fmt.Println("after double done")
}
```

**Actual output:**

```text
panic: sync: negative WaitGroup counter

goroutine 1 [running]:
sync.(*WaitGroup).Add(...)
    .../sync/waitgroup.go:118 +0x264
...
github.com/tucats/ego/internal/language/bytecode.CallWithReceiver(...)
    .../internal/language/bytecode/callNative.go:583 +0x484
...
main.main()
    /Users/tom/go/src/github.com/tucats/ego/main.go:90 +0x2f8
```

(exit code 2 — the entire process terminates; "after double done" is never printed)

**Expected output:**

A catchable Ego runtime error (or at minimum a clean fatal message), not a raw Go-runtime
stack dump that crashes the whole binary and bypasses `try`/`catch` entirely.

**Notes:**  
`sync.WaitGroup` is a native pass-through (per CLAUDE.md's documented "Native pass-through"
pattern), dispatched via `reflect.Value.Call` in
`internal/language/bytecode/callNative.go:CallWithReceiver`. There is no `recover()`
anywhere on this call path (`callNative.go`, `callRuntimeFunction.go`, `run.go`, `main.go`),
so any Go-level panic from the underlying stdlib type propagates all the way out of the
process. Wrapping the calls in `try {} catch (e) {}` does not help — confirmed the crash
still occurs identically.

**Resolution (July 2026):**  
Added a general-purpose panic-recovery safety net at the native-call boundary, rather than
a `sync.WaitGroup`-specific fix, so that *any* native Go method or function called from Ego
that panics is turned into a normal, catchable Ego error instead of crashing the process.

- **`internal/language/bytecode/callNative.go` — new `safeReflectCall` helper.** Both
  `CallWithReceiver` (method calls, e.g. `wg.Done()`) and `CallDirect` (package-level
  function calls, e.g. `math.Sqrt(x)`) now route their `reflect.Value.Call(...)` through this
  shared helper instead of calling it directly. `safeReflectCall` wraps the call in a
  `defer recover()`; if the native code panics, the recovered value is captured in a named
  return and converted into a new error (see below) instead of being allowed to keep
  unwinding the goroutine's stack.
- **New error `ErrNativeCallPanic`** (key `native.call.panic`), added to
  `internal/errors/messages.go` and localized in all three `messages_*.txt` files, per
  CLAUDE.md's "Adding Error Messages" convention. The error's `.Context(...)` includes both
  a short description of what was being called (e.g. `*sync.WaitGroup.Done`, built from
  `reflect.TypeOf(receiver)` and the method name, or the resolved function name via
  `runtime.FuncForPC` for `CallDirect`) and the original panic value, so a developer — or
  Ego code inspecting the caught error — has as much context as possible about what failed
  and why.
- Because the returned value is a plain Go `error` (not a `data.List`), it flows through the
  ordinary bytecode `handleCatch` mechanism and is automatically try/catch-eligible, with no
  further wrapping required.

**What this does *not* fix:** a small number of Go runtime conditions use `runtime.fatal()`
instead of an ordinary `panic()`, and a fatal error is deliberately unrecoverable — no
`recover()`, however placed, can catch it. `sync.Mutex.Unlock()` on an unlocked mutex is
exactly this case; see BUG-28's own resolution for how that specific situation is instead
prevented from ever happening, rather than recovered from after the fact.

**Tests added:**

- `internal/language/bytecode/callNative_test.go` (Section 10): `Test_safeReflectCall_NoPanic`
  (ordinary calls are unaffected) and `Test_safeReflectCall_RecoversPanic` (the direct
  regression test — verifies no panic escapes, and that the resulting error is
  `ErrNativeCallPanic` and mentions both the call description and the original panic text).
- `tests/packages/sync.ego` — `"sync: WaitGroup extra Done is catchable, not a crash"` is the
  end-to-end Ego-level regression test for the original repro above; `"sync: WaitGroup
  normal Add, Done, and Wait"` guards against a regression in ordinary usage.

