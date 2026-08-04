# FUNC-H2 — Closures stored during a loop are invalid after the loop ends

**Affected files:**

- `bytecode/bytecode.go` — `ByteCode` struct and accessor methods
- `bytecode/stack.go` — `pushByteCode` opcode handler
- `bytecode/callBytecodeFunction.go` — closure dispatch

**Description:**  
In Go, closures capture variables by reference and those references remain
valid for the lifetime of the closure (the garbage collector keeps the
captured variable alive as long as the closure lives). A common Go pattern is
to build a slice of closures inside a loop and call them later:

```go
var funcs []func() int
for i := 0; i < 3; i++ {
    i := i                          // capture-copy in Go
    funcs = append(funcs, func() int { return i })
}
// All closures are valid and callable after the loop ends
fmt.Println(funcs[0](), funcs[1](), funcs[2]()) // → 0 1 2
```

In Ego, the loop variable `i` (and any variable declared inside the loop
body, including an explicit copy `i_copy := i`) goes out of scope when the
loop exits. Any closure that references such a variable produces a runtime
error `"unknown identifier: i"` when called after the loop:

```go
for i := 0; i < 3; i = i + 1 {
    i_copy := i
    funcs = append(funcs, func() int { return i_copy })
}
funcs[0]()  // ERROR: unknown identifier: i_copy
```

Even the standard Go workaround of taking a copy of the loop variable does
not help in Ego, because the copy variable is itself scoped to the loop
body and goes out of scope at the same time as `i`.

Closures called *immediately* within the loop (before the variable leaves
scope) work correctly.

**Test file:** `tests/functions/scope_advanced.ego` — test
`"functions: stored closure is invalid after loop"` documents this behavior.

**Recommendation:**  
Extend closure capture semantics so that variables referenced by a closure are
kept alive (allocated on the heap or in a persistent parent symbol table) for
the full lifetime of the closure, even after the enclosing scope is popped.
This is the standard "escape analysis" approach used by Go and other languages
with first-class functions.

**Resolution (May 2026):**  
The root cause was two separate gaps:

1. **`ByteCode` had no field for the captured scope.** A compiled function
   literal is a single `*ByteCode` object. Every iteration of a loop reuses
   the same pointer — so simply setting a scope on the object would cause all
   iterations to overwrite each other.

2. **`callBytecodeFunction` always parented the closure's symbol table to
   `c.symbols` (the runtime scope at call time)**, not the scope that was
   active when the closure was defined.

Three changes fixed this:

- **`bytecode/bytecode.go`**: Added a `capturedScope *symbols.SymbolTable`
  field to `ByteCode` plus `Clone()`, `CaptureScope()`, and
  `GetCapturedScope()` accessors. `Clone()` returns a shallow copy of the
  struct; the instructions slice is shared (read-only after `Seal`) so the
  clone is cheap.

- **`bytecode/stack.go` — `pushByteCode`**: When the operand is a literal
  `*ByteCode`, the handler clones it and stamps `c.symbols` onto the clone's
  `capturedScope` field before pushing. Each push in a loop iteration
  produces a fresh clone with the scope at that moment. Non-literal bytecodes
  are pushed unchanged.

- **`bytecode/callBytecodeFunction.go`**: When dispatching a literal closure
  whose `capturedScope` is non-nil, the function's symbol table is created as
  a child of the captured scope (via `NewChildSymbolTable` +
  `callFramePushWithTable`) rather than of `c.symbols`. This ensures that
  variable lookup walks up through the captured scope chain, where the loop
  variable is still alive as a Go heap object even after `PopScope` removed
  it from the active parent chain.

The scope object is never freed by `PopScope` — Go's garbage collector keeps
it alive as long as any reference to it exists. Once a closure's
`capturedScope` holds that reference, the entire ancestor chain remains valid
and reachable for the lifetime of the closure, which matches Go's escape
analysis semantics.

Tests updated in `tests/functions/scope_advanced.ego`: the test previously
named `"functions: stored closure is invalid after loop"` is renamed to
`"functions: stored closure survives after loop"` and now asserts that
`captured()` returns `3` (the post-loop value of `i`) instead of asserting
that a runtime error is thrown.

