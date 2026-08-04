# BUG-63 — A standalone `x++`/`x--` statement leaks a "let" stack marker, corrupting later function calls — **Resolved**

**Severity:** HIGH

**Description:**  
Found while investigating an unexpected result while working on PERFORMANCE.md Finding 4
(this bug is unrelated to Finding 4 and reproduces identically on an unmodified compiler —
verified by stashing all Finding 4 changes and re-running the reproducer). A standalone
`x++` or `x--` statement, where `x` is a plain variable (not an array element or struct
field), never cleans up a "let" stack marker that was pushed onto the runtime stack while
compiling it. The marker stays on the stack indefinitely, so a later function call in the
same scope (or an enclosing one) can receive extra, unexpected values on its argument/return
stack — most visibly as a "did not return the expected number of values" error, but any
other stack-shape-sensitive operation downstream of the leaked marker is equally at risk.

**Reproducer:**

```go
package main

import "fmt"

func run() int {
    n := 0
    n++

    return n
}

func main() {
    fmt.Println(run())
}
```

**Actual output:**

```text
Error: at main(line 13), function did not return the expected number of values
Error: terminated with errors
```

**Expected output:**

```text
1
```

**Notes:**  
Root cause: `internal/language/compiler/assignment.go`, `compileAssignment`'s auto-increment
handling. `c.assignmentTarget()` (`internal/language/compiler/lvalue.go`) always emits
`Push, NewStackMarker("let")` directly into `c.b` before returning a separate `storeLValue`
bytecode fragment; for a simple variable, that fragment is exactly `[Store x, DropToMarker]`
— the `DropToMarker` is what balances the marker `assignmentTarget` already emitted. In the
normal (non-auto) assignment path, `storeLValue` is appended in full, so the marker is always
drained. But in the "isSimpleLValue" branch of the auto-increment case (`assignment.go`,
around the comment "Simple variable: x++ or x--"), the code builds its own inline
`Load, Push 1, Add/Sub, Dup, Store` sequence and returns immediately — `storeLValue`,
containing the only `DropToMarker` for the marker `assignmentTarget` pushed, is discarded
entirely, only its first instruction's operand (the variable name) is ever read. Every plain
`x++`/`x--` statement compiled this way leaks one "let" marker onto the runtime stack.
This is why the increment clause of a classic `for i := 0; i < n; i++ { ... }` loop is
unaffected: `internal/language/compiler/for.go`'s `iterationFor` does not call
`compileAssignment` for the increment clause — it calls `c.assignmentTarget()` directly and
appends the resulting `incrementStore` (Store + DropToMarker) in full itself, so that call
site was never exposed to this bug. Only a bare `x++`/`x--` used as its own statement (not as
a for-loop's increment clause) triggers it. The qualified-lvalue branch (`a[i]++`,
`s.field++`) is unaffected for the same reason: it explicitly appends the full `storeLValue`
(see the comment "DropToMarker at its end cleans up the 'let' stack marker").

**Resolution (July 2026):**  
`internal/language/compiler/assignment.go` — `compileAssignment`'s "isSimpleLValue"
branch (the `x++`/`x--` case) now emits an explicit `DropToMarker` for the `"let"`
marker immediately after the `Store` instruction, instead of returning right after
`Store`. This mirrors what the qualified-lvalue branch already does by appending the
full `storeLValue` fragment, and restores the "every statement leaves the stack the
way it found it" invariant that the rest of `compileAssignment` relies on. The
previously-emitted `Dup` instruction (which left an extra, never-consumed copy of the
new value on the stack) was removed at the same time — `compileAssignment` is only
ever invoked as a full statement (from `statement.go` and as an `if`-statement init
clause in `if.go`), never as a value-producing expression, so nothing read that extra
copy; a separate code path (`compileSymbolValue` in `expr_atom.go`, gated behind
`extensionsEnabled`) already handles `x++` used as an expression atom and is
unaffected by this change.

Regression tests were added at two levels:

- `internal/language/compiler/run_test.go` — four new cases in
  `TestArbitraryCodeFragments` (tagged `BUG-63`) compile and run small Ego snippets
  containing a bare `x++`/`x--` followed by a function call, and assert on the
  resulting value. Note: these Go-level cases pass even without the fix, because the
  `RunString` test harness does not reproduce the failure mode — see the Ego-language
  tests below for the tests that actually catch the regression.
- `tests/flow/simple_increment_stack_leak.ego` — five `ego test` cases that reproduce
  the bug end-to-end through the real CLI compilation/execution path used by
  `ego run`/`ego test` (confirmed to fail with "function did not return the expected
  number of values" against the pre-fix compiler, and to pass after the fix).

