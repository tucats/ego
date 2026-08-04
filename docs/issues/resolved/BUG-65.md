# BUG-65 — The built-in `error` type is not treated as nil-compatible in three separate coercion/conformance-check paths — **Resolved**

**Severity:** HIGH

**Description:**  
Found while writing comprehensive package-boundary tests for `tests/packages/employee.ego`
(covering constants, package vars, receivers, callbacks, a second imported package, and
error/panic handling across a package boundary — see `tests/packages/employee_errors.ego`
and `tests/errors/strict_nil_error.ego`). A function, method, variable, or parameter declared
with the built-in `error` type could not reliably hold or receive a `nil` value — the single
most common case for any function that returns `error` (the "no error occurred" case). The
root cause recurs in three independent places, all stemming from the same gap: `error`'s
`data.Kind` is `data.ErrorKind`, not `data.InterfaceKind`, so code that special-cases
"interface types accept nil" via `t.IsInterface()` silently excludes `error`.

1. **`--types strict` return values.** A function or method declared to return `error` that
   executes `return nil` on its success path aborts with a runtime error instead of returning
   nil, in `internal/language/bytecode/coerce.go`'s `requireMatch` (called from
   `coerceByteCode` when `c.typeStrictness == StrictTypeEnforcement`). `requireMatch` already
   special-cased "pointer type + nil value", but had no equivalent case for `error`.

2. **Any mode, explicit-type variable declarations.** `var e error = nil` compiled and ran
   without error in *both* strict and dynamic mode, but silently produced a **non-nil** error
   value whose message was the literal text `"<nil>"`, so `e == nil` evaluated to `false`. The
   same corruption occurred for a `nil` argument passed positionally to a plain Ego function
   whose parameter was typed `error` in dynamic mode (this specific call shape goes through
   `internal/language/data/coerce.go`'s `Coerce`, not the strict-mode bytecode path).

3. **`--types strict` function parameters.** Independent of (1) and (2): calling any function
   or method with an `error`-typed parameter under strict mode — passing either `nil` **or a
   genuine, valid error value** — silently replaced the argument with
   `errors.ErrPanic.Context(v)` inside `internal/language/bytecode/types.go`'s
   `strictConformanceCheck`. A `nil` argument became a non-nil error reading `"panic: <nil>"`;
   a real error such as one caught from `5 / 0` became `"panic: division by zero"` and lost
   its identity (`e.Is(errors.New("div.zero"))` returned `false`, and `e.Code()` returned
   `"panic"` instead of `"div.zero"`).

**Reproducer:**

```go
package main

func ok(n int) error {
    if n < 0 {
        return errors.New("negative")
    }
    return nil               // Case 1
}

func accepts(e error) bool {
    return e == nil          // Case 3 (strict), also exercises Case 2's parameter path
}

func main() {
    var e error = nil        // Case 2
    fmt.Println("var nil == nil:", e == nil)

    fmt.Println("return nil ok:", ok(5))
    fmt.Println("param nil accepted:", accepts(nil))
}
```

**Actual output (`ego --types strict run repro.go`):**

```text
Error: at ok(line 6), type mismatch: nil, error
Error: terminated with errors
```

Commenting out the call to `ok` and testing case 2/3 alone instead:

```text
var nil == nil: false
param nil accepted: false
```

**Expected output:**

```text
var nil == nil: true
return nil ok: <nil>
param nil accepted: true
```

**Notes:**  
All three sites independently need to recognize that a `nil` value is a valid instance of
`error` (Go's zero value for any interface type, including the built-in `error`), and that a
non-nil `error` argument must pass through coercion/conformance checks unchanged. Case 3's
`errors.ErrPanic.Context(v)` line pre-dates the current test suite and appears to have been an
attempt to give *some* concrete error value to a caller expecting `error`, but it fired
unconditionally — for `nil` and for legitimate error values alike — rather than only when `v`
did not already satisfy the `error` "interface". No existing Go or Ego test exercised an
`error`-typed return, variable, or parameter with an actual `nil`/real-error value under
`--types strict`, which is why this had gone unnoticed.

**Resolution (July 2026):**

- `internal/language/data/coerce.go` — the `Coerce()` function's `case *errors.Error:` branch
  now returns `(nil, nil)` immediately when the incoming value is `nil`, instead of running it
  through `fmt.Sprintf("%v", value)` (which produced the literal string `"<nil>"`).
- `internal/language/bytecode/coerce.go` — `requireMatch` now treats `v == nil` as an
  automatic match whenever the target type is a pointer **or** `data.ErrorKind`, alongside the
  pre-existing pointer-only nil check.
- `internal/language/bytecode/types.go` — `strictConformanceCheck` no longer unconditionally
  rewrites a value destined for an `error`-typed slot into `errors.ErrPanic.Context(v)`. A
  `nil` value now short-circuits to a direct pass-through; any other value (including a real
  error) falls through to the normal `actualType.IsType(t)` conformance check below, which
  already handles `error` correctly once it is not preempted by the panic-wrapping line.

Regression tests were added at two levels:

- `internal/language/data/coerce_test.go` — `TestCoerce/test_with_nil_error_model_stays_nil`.
- `internal/language/bytecode/coerce_test.go` — `Test_requireMatch_ErrorTypeWithNilValue`.
- `internal/language/bytecode/types_test.go` — `Test_requiredTypeByteCode_NilForError_Strict`
  and `Test_requiredTypeByteCode_RealErrorForError_Strict` (the latter asserts on `Code()` to
  confirm the error's original identity, not just its `Is()` behavior, survives the call).
- `tests/errors/strict_nil_error.ego` — five `ego test` cases covering all three call shapes
  (function return, pointer-receiver method return, `var ... error = nil`, a `nil` literal
  parameter, and a real caught error passed as a parameter), run under both the default and
  `--types strict` compilers.
- `tests/packages/employee_errors.ego` — exercises the same `error`-return idiom through an
  imported package's methods (`Employee.Validate()`), which is what surfaced this bug during
  package-boundary testing in the first place.

All three fixes were verified against `go test ./...`, `go test -race ./...`, and both
`ego test tests/` and `ego test --types strict tests/` (1229 `@test` blocks each) with no
regressions.

