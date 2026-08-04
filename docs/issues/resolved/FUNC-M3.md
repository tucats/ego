# FUNC-M3 — Dynamic mode silently accepts wrong-type arguments

**Affected files:**

- `compiler/function.go` — argument type checking
- `bytecode/coerce.go` — runtime coercion

**Description:**  
In Go, passing a value of the wrong type to a function parameter is always a
compile-time error. In Ego's dynamic mode (the default), type mismatches at
function call boundaries are not rejected. The argument retains its actual type
inside the function body, which can produce silently wrong results.

The most common case is passing a string where an integer is expected:

```go
func double(n int) int { return n * 2 }
double("5")   // No error in dynamic mode
// Inside double: n is still a string "5"
// "5" * 2 = "55" (string repetition — see FUNC-L1)
```

The result is neither the expected integer `10` nor an error — it is the string
`"55"` silently coerced into the `int` result variable.

In strict mode (`ego test --types=strict`), the mismatch is caught as a runtime
type error.

**Test file:** `tests/functions/arg_types.ego` — test
`"functions: dynamic mode string to int no error"` documents this behavior.

**Recommendation:**  
In dynamic mode, add an optional warning (controllable by a setting) when a
value of a clearly incompatible type is silently accepted for a statically-typed
parameter. Alternatively, promote this check from strict-only to a standard
runtime check, and only bypass it in a more permissive "relaxed" mode.

**Resolution (May 2026):**  
Two changes:

- **`data/types.go`**: Added `IsCoercible(*Type) bool`, which returns `true` for
  scalar types (bool, all integer widths, both float widths, and string). Complex
  types (struct, array, map, channel, function, pointer) return `false` and are
  never silently coerced.

- **`bytecode/arg.go` — `argByteCode`**: After all existing type-guard checks,
  if the expected parameter type is coercible and the argument's type does not
  already match, `data.Coerce` is called to convert the value before it is stored
  in the function's local symbol table. On failure (e.g., `"abc"` where `int` is
  expected), a descriptive error including argument position and original value is
  returned — and because it goes through the normal runtime error path, `try/catch`
  can catch it.

  As a result, `double("5")` now returns `10` instead of `"55"`. Passing a
  non-coercible value such as `double("abc")` raises a catchable runtime error
  rather than producing a silently wrong result.

The previously-used test `"functions: dynamic mode string to int no error"` was
replaced by `"functions: type mismatch is always catchable"` which covers both
the success case (coercible string) and the error case.

