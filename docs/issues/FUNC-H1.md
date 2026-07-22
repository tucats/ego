# FUNC-H1 — Calling a variadic function with zero variadic arguments fails

**Affected files:**

- `compiler/function.go` — argument count validation
- `bytecode/callframe.go` — argument unpacking at call time

**Description:**  
In Go, a function declared as `func f(args ...int)` may be called as `f()` —
passing zero variadic arguments is valid, and `args` inside the function body
is `nil` (or an empty slice). The same is true for functions with a fixed
parameter plus varargs: `func g(base int, args ...int)` may be called as
`g(10)` — the varargs receive an empty slice.

In Ego, both forms fail at runtime with `"incorrect function argument count"`:

```go
func sum(args... int) int { ... }
sum()       // ERROR: incorrect function argument count

func sumFrom(base int, args... int) int { ... }
sumFrom(10) // ERROR: incorrect function argument count
```

This breaks common Go patterns such as logging helpers, optional-parameter
functions, and aggregation functions that are sometimes called with no
additional values.

The spread operator (`sum(nums...)`) works correctly.

**Test file:** `tests/functions/variadics.ego` — tests
`"functions: zero variadic args is an error"` and
`"functions: zero varargs after fixed is error"` document the current behavior.

**Recommendation:**  
In the argument-count validation pass, treat a variadic parameter as
contributing zero or more arguments (not one or more). When the call provides
exactly the number of fixed parameters and no varargs, pass an empty (nil)
slice for the variadic parameter rather than rejecting the call.

**Resolution (May 2026):**  
Three changes to `compiler/function.go`:

1. **`generateFunctionBytecode` (lines 150–161):** The `ArgCheck` bytecode was
   emitted as `(len(parameters), -1)` for variadic functions, using the total
   parameter count as both the minimum and (sentinel) maximum. The minimum is
   now emitted as `len(parameters) - 1`, so only the fixed parameters are
   required and the variadic parameter contributes zero or more.

2. **`ParseFunctionDeclaration` (line 487):** The `hasVarArgs` return value from
   `parseParameterDeclaration` was silently discarded with `_`. It is now
   captured and used to set `funcDef.Variadic = hasVarArgs`, so the
   `Declaration.Variadic` flag is correctly propagated when a function is
   called via a `data.Function` wrapper (e.g. as a type method).

3. **`storeOrInvokeFunction` (line 323):** When building a `data.Declaration`
   for a function literal whose declaration was not already known, `Variadic`
   is now derived from the `parms` slice by checking whether the last
   parameter's kind is `VarArgsKind`.

The `ArgCheck` opcode handler (`bytecode/argcheck.go`) and the variable-argument
extraction instruction (`bytecode/stack.go:getVarArgsByteCode`) already handled
the empty-slice case correctly — no changes were needed there.

Tests updated in `tests/functions/variadics.ego`: the two tests that previously
expected errors on zero-argument calls now assert the correct return values
instead, and their names and comments reflect Go-compatible behavior.

