# BUG-67 — `--types strict` rejects a plain int literal at an `int32` (or other narrower-int) return/argument boundary — **Resolved**

**Severity:** MEDIUM

**Description:**  
Found while implementing named scalar type identity (`type buzz int32`) and adding tests for
it under `--types strict`. Two related, narrower-int-typed boundaries both reject a bare `int`
literal/expression where Go itself would happily convert an untyped constant: (1) returning
the result of `int32 * int-literal` arithmetic from a function declared to return `int32`, and
(2) passing a bare integer literal to a parameter declared `int32`. Neither reproducer involves
a user-defined type — both fail identically with plain built-in `int32`, so this is a
pre-existing gap in strict-mode return/argument type-checking, not something introduced by the
named-scalar-types feature. Notably, `==`/`!=`/ordered comparisons already have special-cased
handling for a compile-time-constant operand (`getComparisonTerms` in
`internal/language/bytecode/equal.go`, which coerces a literal up/down to match the other
operand's kind even in strict mode) — the return-value and argument-passing boundaries have no
equivalent leniency.

**Reproducer (return value):**

```go
func doubled(b int32) int32 {
    return b * 2
}

func main() {
    fmt.Println(doubled(int32(7)))
}
```

Run with `ego --types strict run`.

**Actual output:**

```text
Error: at doubled(line 2), type mismatch: int, int32
Error: terminated with errors
```

**Expected output:**

```text
14
```

**Reproducer (argument passing):**

```go
func f(x int32) int32 {
    return x
}

func main() {
    fmt.Println(f(4))
}
```

Run with `ego --types strict run`.

**Actual output:**

```text
Error: at f(line 6), incorrect function argument type: argument 1: int
Error: terminated with errors
```

**Expected output:**

```text
4
```

**Notes:**  
Root-caused to two independent bugs, not one shared cause.

**Resolution (July 2026):**

The two reproducers turned out to have different root causes, both fixed:

- **Return-value case** — `internal/language/data/coerce.go`'s `Normalize` always promoted
  the *narrower-kind* operand up to the *wider-kind* one (`int32`, kind 6, is less than `int`,
  kind 8, so `int32` was promoted to `int`), regardless of which operand was a literal constant
  and which was a real, typed value. Go's actual rule is the reverse: an *untyped constant*
  adapts to the *other*, typed operand. `Normalize` now takes a `v1Const`/`v2Const` flag per
  operand; when exactly one operand is a constant and both are numeric, the constant adapts to
  match the other operand's kind instead of the kind-ordering promotion. `b * 2` (`b` an
  `int32`) now correctly stays `int32`, and the return-boundary check
  (`requireMatch` in `internal/language/bytecode/coerce.go`) — which already did an exact
  `vt.IsType(t)` match — now passes unmodified, since it was never wrong; it was just being
  handed a wrongly-promoted value. The constant-flags are threaded through `getDiadicValues`
  (and `moduloByteCode`'s inline duplicate of the same pop/unwrap logic, which turned out to
  have a latent bug of its own: it called the auto-unwrapping `c.Pop()` instead of
  `c.PopWithoutUnwrapping()`, so its own `data.Immutable` check could never fire and modulo's
  `coerceOk` was always `false`) into every `data.Normalize` call site. The six comparison
  opcodes (`equal.go`, `notEqual.go`, `greaterThan.go`, `greaterThanorEqual.go`, `lessThan.go`,
  `lessThanorEqual.go`) and the compile-time constant-folding optimizer needed only a mechanical
  signature update — their output is always a `bool` or a folded constant, so the promotion
  direction never affected their correctness.

- **Argument-passing case** — `internal/language/bytecode/types.go`'s `strictConformanceCheck`
  required an *exact* kind match for every argument value with no leniency for constants at
  all, unlike `getComparisonTerms` (`equal.go`), which already special-cases "either operand is
  a constant and both are numeric" for `==`. The "was this a constant" information was
  discarded even earlier than `arg.go`: `callByteCode`'s argument-collection loop
  (`internal/language/bytecode/call.go`) unconditionally unwrapped `data.Immutable` via
  `c.Pop()` before arguments were packed into the `__args` array shared by every kind of
  function call (bytecode, native, runtime). A new parallel array,
  `defs.ArgumentConstListVariable` (`"__argsconst"`), is now populated alongside `__args` for
  Ego-bytecode-function calls only (native/runtime call paths are untouched), and consumed by
  `arg.go` to pass a `valueIsConst` flag into a new `requiredTypeByteCodeWithConst`, which
  `strictConformanceCheck` uses to let a numeric constant adapt to a narrower declared
  parameter type — falling through to the coercion switch that already existed just below the
  check. `requiredTypeByteCode` itself (used for variadic parameters and `throw.go`) is
  unchanged: it now delegates to the same shared implementation with `valueIsConst` hardcoded
  to `false`, preserving its exact prior behavior.

Both fixes reuse the same "constant adapts to context" pattern already established by
`getComparisonTerms`, rather than introducing a new concept. A non-constant value of the wrong
kind (e.g. two `int32`/`int` *variables* added together, or a string constant passed where an
`int32` is expected) is still correctly rejected in strict mode — see the regression tests
below.

Regression tests: `tests/types/bug67_constant_narrowing.ego` — both reproducers, plus two
guard tests confirming the leniency does not over-relax strict mode for non-constant or
non-numeric mismatches.

Verified against `go test ./...`, `go test -race ./internal/language/... ./internal/runtime/math/...`,
and both `ego test tests/` and `ego test --types strict tests/` (1292 `@test` blocks each,
including `tests/types/named_scalar_types.ego` from the prior BUG-fix session, which depends
on `Normalize`'s `*data.Scalar`-unwrap behavior surviving the signature change) with no
regressions. Also manually spot-checked that a typed variadic call
(`func f(args ...int32) ...; f(int32(1), int32(2))`) and a `try`/`catch`-caught runtime error
still work, since `call.go`'s argument-collection loop and `requiredTypeByteCode`'s shared
implementation were both touched.

