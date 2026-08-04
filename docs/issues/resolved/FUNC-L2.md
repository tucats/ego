# FUNC-L2 — Named return with explicit return value is a compile error

**Affected files:**

- `compiler/return.go` — return statement compilation

**Description:**  
In Go, a function with named return variables can mix bare returns (`return`)
and explicit-value returns (`return someValue`):

```go
func clamp(x, lo, hi int) (result int) {
    if x < lo { return lo }   // explicit value with named return — valid in Go
    if x > hi { return hi }
    result = x
    return                    // bare return — valid in Go
}
```

In Ego, using an explicit return value expression inside a function declared
with named returns is a compile error:

```go
func clamp(x, lo, hi int) (result int) {
    if x < lo { return lo }   // Ego compile error
    ...
}
```

Users must assign to the named return variable and then use a bare `return`:

```go
if x < lo { result = lo; return }
```

Bare returns and defErred modifications to named returns both work correctly.

**Test file:** `tests/functions/named_returns.ego` covers the working cases
(bare return, zero values, early return paths). The explicit-value form is
excluded from tests because it does not compile.

**Recommendation:**  
In the return statement compiler (`compiler/return.go`), when the current
function has named return variables and an explicit return value expression
is provided, assign the expression to the named return variable (if there is
exactly one) or to all named return variables in order (if there are multiple)
and then proceed as a bare return. This is consistent with Go's semantics.

**Resolution (May 2026):**  
`compiler/return.go` — `compileReturn` restructured:

1. **Explicit-value path added**: when the function has named return variables
   and the return statement is not at a statement end, each expression is
   parsed, coerced via `c.coercions[i]`, and stored into the corresponding
   named return variable with `Store`. Too many or too few values produce
   `ErrReturnValueCount` / `ErrMissingReturnValues` respectively.

2. **`RunDefers` moved inside the named-return block**, placed *after* any
   explicit assignments. This matches Go semantics: the named variable receives
   the explicit value first; defErred functions then run and may read or modify
   it; the final value is loaded and returned. The previous bare-return
   behavior is unchanged — when no explicit values are present the assignment
   loop is skipped and `RunDefers` still fires before the loads.

3. **`ErrInvalidReturnValues` check removed** — the error that previously
   rejected any token after the `Return` instruction in the named-return branch
   is deleted.

Three new tests added to `tests/functions/named_returns.ego`:

- `"functions: single named return with explicit value"` — the `clamp` pattern
  from the issue description, exercising all three return paths in one function
- `"functions: two named returns with explicit values"` — multi-value explicit
  return via `split`
- `"functions: named return explicit value interacts with defer"` — verifies
  that a defErred closure observes the post-assignment value of the named
  variable (`f(5)` returns `12`, not `6`)

