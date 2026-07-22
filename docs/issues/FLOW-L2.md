# FLOW-L2 — `switch init; expr` semicolon-separated form not supported

**Affected files:**

- `compiler/switch.go` — `compileSwitchAssignedValue`

**Description:**  
In Go, a switch statement can separate the init statement from the switch expression
with a semicolon:

```go
switch x := compute(); x > 0 {   // init-statement ; condition
case true: ...
case false: ...
}
// or for a value switch:
switch x := getCode(); x {
case 1: ...
case 2: ...
}
```

In Ego, the `switchInit` production accepts either an assignment OR an expression,
but not both separated by a semicolon. The Go form `switch x := f(); x { ... }`
produces a compile error `"missing '{}'"`. The Ego-supported form is
`switch x := f() { ... }` where the init variable becomes the switch value
automatically.

**Workaround:**  
Use a preliminary assignment before the switch:

```go
x := compute()
switch x {
case 1: ...
}
// or the Ego init-only form (init is also the switch value):
switch x := compute() {
case 1: ...
}
```

**Test file:** `tests/flow/switch_advanced.ego` — comment at the top of the file
references this issue; `"flow: switch with init assignment"` demonstrates the
supported Ego init-only form.

**Recommendation:**  
In `compileSwitchAssignedValue`, after parsing the first clause, check for a
semicolon token and, if present, parse a second expression as the switch value.
This would make `switch x := f(); x { ... }` a valid Ego form. Low priority since
the Ego init-only form and the pre-assignment workaround cover most use cases.

**Resolution (May 2026):**  
One change to `compiler/switch.go` — `compileSwitchAssignedValue`:

After `emitExpression()` returns for the first (init) clause, the function now
checks for a `SemicolonToken`. If found, it takes one of two paths:

- **Named init** (`switch x := f(); expr`): stores the first expression result
  under the declared variable name via `DefineSymbol` + `CreateAndStore`, making
  `x` available in the second expression and in case bodies. Then generates a new
  synthetic name, emits the second expression, and stores it as the actual switch
  test value.

- **Anonymous init** (`switch f(); expr`): discards the first expression result
  with `Drop` (side effects still execute), then emits the second expression and
  stores it under the originally-generated synthetic name.

When no semicolon is present, the existing single-expression paths are unchanged.
The `hasScope` flag continues to control whether `PopScope` or `SymbolDelete` is
emitted at the end of the switch — both forms still clean up correctly because in
the named case both variables live inside the pushed scope.

Three new tests added to `tests/flow/switch_advanced.ego`:

- `"flow: switch semicolon form with same variable as switch value"` — verifies
  `switch code := getCode(); code { case 3: ... }` selects the correct branch.
- `"flow: switch semicolon form with boolean condition"` — verifies
  `switch n := getValue(); n > 0 { case true: ... }` with a derived boolean.
- `"flow: switch semicolon init var visible in case body"` — verifies that the
  init variable `code` is readable inside the matching case body.

