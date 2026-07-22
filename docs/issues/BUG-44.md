# BUG-44 — In `switch init; expr`, a `case` body cannot shadow the switch's init variable

**Severity:** MEDIUM

**Description:**  
FLOW-L2 added support for `switch init; expr { ... }`, documenting that "the init variable
is in scope in case bodies." However, the fix appears to have made case bodies share the
*same* scope as the init clause rather than a nested child scope, so a `case` body cannot
declare a new local variable with the same name as the switch's init variable — something
that compiles and runs fine in real Go, since a `case` clause body is its own implicit
block nested inside the switch statement's block.

**Reproducer:**

```go
func main() {
    switch v := 1; v {
    case 1:
        v := 99
        fmt.Println(v)
    }
}
```

**Actual output:**

```text
Error: at main(line 4), symbol already exists: v
```

**Expected output:**

```text
99
```

**Notes:**  
Confirmed workaround: wrapping the `case` body in an extra explicit `{}` block fixes it
(prints `99`), which proves the case body itself is not being treated as its own scope when
the switch has an init clause — ordinary `switch` (no init clause) does not have this
problem; variable shadowing across separate `case`s works correctly there.

**Resolution (July 2026):**  
The "ordinary switch has no problem" claim in the original report turned out to be
incomplete — investigating the root cause showed case bodies were never scoped as their own
block at all, with or without an init clause; the "no problem" case above was only masked
because two different `case` labels in the same switch never both execute, so the runtime
collision never had a chance to surface. Fixed in `internal/language/compiler/switch.go`:

- **`compileSwitchCase`** now wraps its case-body statement-compiling loop in a
  `PushSymbolScope()`/`bytecode.PushScope` pair before the loop and the matching
  `bytecode.PopScope`/`PopSymbolScope()` pair after it — exactly the same pattern
  `compileBlock` already uses for ordinary brace-delimited blocks (`if`, `for`, function
  bodies, etc.). This nests each case body's scope one level inside the switch's own scope
  (the one holding the init variable, when present), so a `:=` inside a case body creates a
  genuinely new, independently-scoped variable instead of writing into the switch's shared
  scope.
- **`compileSwitchDefaultBlock`** got the identical treatment, since it is a separate
  function from `compileSwitchCase` and would otherwise still exhibit the bug for `default:`
  clauses.
- The push/pop pair is placed so that a pending `fallthrough` branch (patched to land at the
  very start of the *next* clause's body) enters that clause's own fresh scope, and the
  normal exit branch out of a case runs only after that case's scope has already been
  popped — so a variable declared in a case that falls through is correctly **not** visible
  in the case it falls into, matching real Go (each case, including one entered via
  `fallthrough`, is its own block).

This fixes three symptoms that turned out to share one root cause: (1) the reported
init-variable-shadowing case, (2) the same collision for the semicolon-less named-init form
(`switch v := expr { ... }`) and for the `default:` clause, and (3) — beyond what the report
described — a case-body variable in a plain `switch expr { ... }` (no init clause at all)
leaking into whatever scope enclosed the switch statement and remaining there after the
switch ended, which could collide with an unrelated later declaration of the same name.

**Tests added:**

- `internal/language/compiler/switch_test.go`: `TestBUG44CaseBodyCanShadowSemicolonInitVariable`
  (the exact reproducer), `TestBUG44CaseBodyCanShadowNamedInitVariable` (the semicolon-less
  named-init spelling), `TestBUG44CaseBodyVariableDoesNotLeakToEnclosingScope` (the broader
  no-init-clause leak), `TestBUG44DefaultBodyCanShadowSwitchInitVariable`, and
  `TestBUG44FallthroughDoesNotLeakVariableToNextCase`.
- `tests/flow/switch_advanced.ego`: `"flow: case body can shadow switch init variable
  (BUG-44)"`, `"flow: case body can shadow named-init switch variable (BUG-44)"`, `"flow:
  default body can shadow switch init variable (BUG-44)"`, `"flow: case body variable does
  not leak past the switch (BUG-44)"`, and `"flow: fallthrough does not leak variable into
  next case (BUG-44)"`.

