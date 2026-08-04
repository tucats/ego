# BUG-30 — Closures created in a loop all capture the loop variable's final value, not a per-iteration value

**Severity:** HIGH  
**Status:** Fixed

**Description:**  
Building a slice of closures inside a `for` or `range` loop, where each closure captures
the loop's index/value variable, results in every closure observing the *same final* value
of that variable rather than the value it had on the iteration that created the closure.
This was standard Go behavior before Go 1.22, but since Go 1.22 the language spec changed
so that `for`-loop and `range`-loop variables get a fresh instance per iteration — this is
now one of the best-known recent Go semantic changes, and Ego does not match it. This
divergence is not listed in `docs/LANGUAGE.md`'s "Key differences from Go" table.

**Reproducer:**

```go
func main() {
    var funcs []any
    for i := 0; i < 3; i++ {
        funcs = append(funcs, func() int { return i })
    }
    for _, f := range funcs {
        fmt.Println(f())
    }
}
```

**Actual output:**

```text
3
3
3
```

**Expected output** (verified against real Go 1.24, `go run`):

```text
0
1
2
```

**Notes:**  
The same defect reproduces for `range` loops (both the range value and the range index
variable). Root cause: `internal/language/compiler/for.go` emits `PushScope` once, before
the loop begins, for the loop's index/value variable(s); the same scope — and therefore the
same underlying variable — is reused across all iterations instead of a fresh scope per
iteration. The existing regression test for the related FUNC-H2 fix
(`Test_pushByteCode_LoopIterationsCaptureDifferentScopes`) only verifies that closures
created in two *separate contexts* (simulating separate goroutines) capture distinct
scopes; it does not exercise the single-context, sequential-loop-iteration case shown here.

This also means Ego's own existing regression test
(`tests/functions/scope_advanced.ego`, `"functions: stored closure survives after loop"`)
encodes stale, pre-Go-1.22 semantics: it asserts that a single closure variable reassigned
each iteration observes the *post-loop* value of `i` (`3`), commented "Matches Go" — but
real modern Go actually returns `2` for that exact scenario (verified via `go run`), because
the last iteration's closure captures its own per-iteration copy of `i`, which is never
mutated further once the loop exits. That test's assertion should be revisited alongside
any fix for this issue.

**Fix:**  
`compileFor`'s single, once-per-loop `PushScope` (for the loop's index/value variable) is
unchanged — the increment/condition clauses of a classic `for i := 0; i < n; i++` loop, and
the `RangeNext` bytecode for a `for k, v := range x` loop, still read and write that one
persistent variable exactly as before. What changed is that the loop **body** — which,
unlike the loop's own outer scope, is already recreated fresh every iteration (its
`PushScope` instruction is ordinary loop-body bytecode, re-executed every time the loop
branches back to the top) — now always begins with a small "prologue" that copies the
persistent variable's current value into a same-named variable in that fresh, per-iteration
scope. A closure created anywhere in the body captures *that* scope (this already worked
correctly per-iteration for ordinary body-declared variables; see the FUNC-H2 fix), and
therefore this iteration's own copy, not the loop's single, ever-changing variable.

For a classic `for` loop specifically, an "epilogue" was also added: it copies the
per-iteration copy's value back out to the persistent variable before the per-iteration
scope is destroyed, preserving the (rare, but legal) Go pattern of a loop body reassigning
its own counter to influence later iterations (e.g. `for i := 0; i < 10; i++ { if skip { i
+= 5 } }`). Range loops do not get an epilogue: reassigning a range loop's index or value
variable inside the body has no effect on which element is visited next in Go, so there is
nothing to copy back.

A loop whose counter is a *qualified* lvalue rather than a plain name (e.g. `for a[0] = 0;
a[0] < n; a[0]++`) is deliberately left unchanged — there is no single variable name to give
a fresh per-iteration copy to, and this form is rare enough that it isn't worth the added
complexity of shadowing an arbitrary lvalue expression.

**Known limitation:** the epilogue (mutation copy-back) only runs when a loop iteration
finishes normally, by falling off the end of the body. A `break` or `continue` — including
one nested inside an `if`/`switch`/`try` block — branches directly out of the body and skips
the epilogue entirely. This is a deliberate, accepted trade-off, distinct from [BUG-61](#BUG-61)
(now resolved): `break`/`continue` correctly closes the per-iteration body scope itself via an
explicit `PopScope, N` before branching, so the scope-leak BUG-61 describes does not apply
here — what's still skipped is only the epilogue's *value copy-back* step (reading the
per-iteration shadow copy and writing it into the loop's outer, persistent variable), which
was never part of what BUG-61's fix set out to address. In practice this means a
same-iteration reassignment of the loop counter is not carried forward to the next iteration
if that same iteration also used `break` or `continue`. This does **not** affect the primary
fix: closures still correctly capture their own per-iteration copy of the variable no matter
how the iteration ends, because the copy-in prologue always runs first, before any user code
(including any `break`/`continue`) executes — this is verified directly by a test in both the
Go and Ego test suites.

`tests/functions/scope_advanced.ego`'s `"functions: stored closure survives after loop"` and
`tests/flow/defer_lifo.ego`'s `"flow: defer in a loop registers multiple defers"` were updated
to match: the former now asserts the modern-Go value (`2`, not `3`); the latter had its
now-unnecessary — and, after this fix, actively conflicting (duplicate declaration in the
same scope) — manual `s := s` per-iteration-copy workaround removed.

**Files changed:**

- `internal/language/compiler/for.go` — added `loopVariablePrologue()`, `loopVariableEpilogue()`,
  and `compileForBody()`; wired the prologue into `rangeFor()` and the prologue+epilogue into
  `iterationFor()` (only when the loop counter is a simple name)
- `tests/functions/scope_advanced.ego` — updated `"functions: stored closure survives after
  loop"` to assert the correct Go 1.22+ result
- `tests/flow/defer_lifo.ego` — removed the now-obsolete `s := s` workaround in `"flow: defer
  in a loop registers multiple defers"`
- `internal/language/compiler/for_loopvar_test.go` — new Go unit tests: the BUG-30 reproducer
  for classic and range loops, index/value/both range variants, the updated
  stored-closure-after-loop scenario, counter mutation propagating through normal completion,
  break/continue/labeled-continue regression guards, nested loops, an unused-loop-variable
  compile guard, and the qualified-lvalue-counter fallback path
- `tests/flow/for_loopvar.ego` — new Ego-level regression tests covering the same scenarios
  end-to-end, plus a goroutine-in-a-loop guard

