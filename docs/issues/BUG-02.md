# BUG-02 — `go func() {}()` closures cannot read outer-scope variables

**Severity:** HIGH

**Description:**  
An anonymous function literal launched as a goroutine (`go func() { ... }()`)
cannot access variables from the enclosing scope. Any reference to an outer
variable produces a runtime error `"unknown identifier: <name>"`. This breaks
the idiomatic Go pattern of launching goroutines as closures.

The bug occurs when the goroutine runs asynchronously in a separate OS thread.
When the goroutine happens to run on the same thread (because the main goroutine
blocks on an unbuffered channel receive), it can sometimes access the parent
scope — but this is unreliable and race-dependent.

**Reproducer:**

```go
import "fmt"

func main() {
    x := 42
    go func() {
        fmt.Println("x:", x)   // should print 42
    }()
    for i := 0; i < 1000000; i++ {}   // spin to let goroutine run
    fmt.Println("done")
}
```

**Actual output:**

```text
Error: at go func ()(line 6), unknown identifier: x
```

**Expected output:**

```text
x: 42
done
```

**Workaround:**  
Pass all captured variables as explicit arguments to the anonymous function:

```go
go func(val int) {
    fmt.Println("x:", val)
}(x)
```

**Notes:**  
This also affects `sync.WaitGroup` usage where the goroutine captures `wg`
from the outer scope — `wg.Wait()` hangs because the goroutines cannot call
`wg.Done()` through the captured reference.

**Resolution (June 2026):**

**Root cause:** `pushByteCode` in `bytecode/stack.go` unconditionally overwrote
`capturedScope` with `c.symbols` every time a literal `*ByteCode` was pushed.
The sequence for `go func() { ... }()` is:

1. The parent context executes `Push <literal>` — clones the raw compiled
   bytecode and stamps `c.symbols` (the parent's local scope, containing `x`)
   as `capturedScope`. This is correct.
2. `goByteCode` pops the clone as `fx`.  `fx.capturedScope` now points at the
   parent's local scope.
3. `GoRoutine` constructs a tiny call script `Push fx; Call N` that runs in a
   minimal goroutine context whose `c.symbols` is a child of the global root
   (knows nothing about `x`).
4. When `Push fx` executes in the goroutine, `pushByteCode` cloned `fx` (the
   clone inherits `capturedScope` from `fx`) then **overwrote**
   `capturedScope = c.symbols` — the goroutine's root-child scope.
5. The closure's `callBytecodeFunction` then created a function scope as a
   child of the goroutine's root-child scope instead of the parent's local
   scope, so `x` was never reachable.

The user hint was confirmed: anonymous functions are (and must be) compiled with
`PushScope` (no boundary flag) while named functions use
`PushScope BoundaryScope`. This was already correct in the compiler
(`compiler/function.go` lines 146–149). The bug was purely in `pushByteCode`.

**Fixes applied:**

`bytecode/stack.go` — `pushByteCode`: added a guard `if clone.capturedScope == nil` so
the current scope is only stamped onto a clone when none has been captured yet.
An already-captured scope (the goroutine re-push case) is left intact, preserving
the parent's local variables in the chain. The loop-closure case (FUNC-H2) is
unaffected: the original compiled literal always starts with `capturedScope == nil`,
so each loop iteration still captures its per-iteration scope.

`bytecode/goroutine.go` — `GoRoutine`: when `fx` is a literal closure with a non-nil
`capturedScope`, calls `capturedScope.Shared(true)` before the goroutine runs.
`Shared(true)` propagates up the entire ancestor chain, so every symbol table
the closure will traverse is protected by read/write locks. This prevents data
races when the parent thread and the goroutine access the same tables concurrently.

**Tests added:**

- `bytecode/stack_test.go` — two new Go unit tests:
  - `Test_pushByteCode_PreservesCapturedScope` — direct regression test simulating
    the parent → goroutine double-push sequence; verifies the second push does
    not overwrite the scope captured by the first.
  - `Test_pushByteCode_LoopIterationsCaptureDifferentScopes` — verifies the
    loop-closure case continues to produce a distinct per-iteration scope.

- `tests/flow/go_func_literal.ego` — four new Ego language tests:
  - `"flow: goroutine closure reads outer-scope variable"` — direct BUG-02
    regression test; a goroutine closure must be able to read `x` from the
    outer scope.
  - `"flow: goroutine closure captures multiple outer-scope variables"` — same
    scenario with three captured variables.
  - `"flow: goroutine closure with explicit arguments still works"` — verifies
    the original argument-passing form is unaffected.
  - `"flow: named-function goroutine unaffected by BUG-02 fix"` — verifies named
    function goroutines are not broken by the change.
