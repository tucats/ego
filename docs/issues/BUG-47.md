# BUG-47 — Negative shift amounts silently flip the shift operator's direction instead of erroring

**Severity:** MEDIUM

**Description:**  
Shifting by a negative amount (`x << n` or `x >> n` where `n < 0`) silently reinterprets
the operator's direction (i.e. `x << n` behaves as `x >> |n|`) instead of raising an error.
Real Go panics with `"runtime error: negative shift amount"` for a non-constant negative
shift. This divergence is not listed in the "Key differences from Go" table.

**Reproducer:**

```go
func main() {
    n := -2
    fmt.Println(20 << n)
    fmt.Println(20 >> n)
}
```

**Actual output:**

```text
5
80
```

**Expected output** (verified against real Go, `go run`): a runtime error
(`"runtime error: negative shift amount"`), not a silently reversed result.

**Notes:**  
Root cause: `internal/language/compiler/expr_operators.go:117-124` compiles `<<` as
`Negate` + `BitShift`, so a single `BitShift` opcode
(`internal/language/bytecode/math.go:1122-1163`) can infer direction from the sign of its
operand (`shift < 0` → left, `shift >= 0` → right). This conflates the compiler's internal
sign-encoding trick with a genuinely negative user-supplied shift amount: negating an
already-negative `n` flips it positive, which the runtime then reads as "shift right,"
silently reversing the operator's semantics instead of raising the existing
`ErrInvalidBitShift` (which only fires for `shift < -64 || shift > 63`, not for an ordinary
negative shift that Go rejects).

**Resolution (July 2026):**  
The shift direction is no longer encoded in the sign of the shift amount. The
compiler (`internal/language/compiler/expr_operators.go`) now emits the
`BitShift` opcode with a boolean operand — `true` for `<<`, `false` for `>>` —
instead of the old `Negate` + `BitShift` sequence for left shifts. Because the
direction travels in the opcode, the runtime
(`internal/language/bytecode/math.go:bitShiftByteCode`) sees the user's literal
shift amount and can validate it:

- A negative amount returns the new `ErrNegativeShift` error (`negative shift
  amount`), matching Go's runtime panic, and is catchable via `try/catch`.
- The pre-existing upper bounds are preserved: a left shift accepts an amount up
  to 64 (so `1 << 64 == 0`, which `lib/packages/math` relies on for
  `MaxUint64`), and a right shift up to 63; larger amounts still return
  `ErrInvalidBitShift`.

Go-level tests were added in `internal/language/bytecode/math_test.go` (both
directions, boundary amounts, and negative-shift errors), and Ego-level tests in
`tests/datamodel/bitshift.ego`. `docs/LANGUAGE.md` now documents the operators'
non-negative shift-amount requirement.

