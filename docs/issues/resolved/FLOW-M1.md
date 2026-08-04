# FLOW-M1 — `for` init clause only accepts `:=`; `=` for existing variables fails — **Resolved**

**Affected files:**

- `compiler/for.go` — for-statement parsing

**Description:**  
In Go, the init clause of a C-style `for` loop accepts either `:=` (declare a new
variable) or `=` (assign to an existing variable):

```go
var i int
for i = 0; i < 10; i++ { ... }  // Go: valid; i retains its value after the loop
```

The LANGUAGE.md guide documents this pattern explicitly and provides a code example.
In Ego, the init clause only accepted `:=`; using `=` for a pre-declared variable
produced a compile error:

```go
var i int
for i = 0; i < 10; i++ {}   // Ego: ERROR "missing ':='"
```

**Resolution:**  
Fixed in `compiler/for.go` — `IsNext(DefineToken)` changed to
`AnyNext(DefineToken, AssignToken)` at line 93. The `assignmentTarget()` function
already handled both forms correctly; only the guard that rejected `=` needed removal.
Test added: `"flow: for-loop init clause with assignment to existing variable"` in
`tests/flow/while_loop.ego`.

