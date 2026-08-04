# BUG-69 — Struct field assignment was the one coercion boundary missing the constant-adapts-losslessly rule — **Resolved**

**Severity:** LOW

**Description:**  
Found while confirming, at the user's request, that the `ego.runtime.precision.error` setting
and the BUG-67/68 fixes together produce Go-like behavior in `--types strict` regardless of the
setting. Four boundaries were verified consistent (variable assignment, expressions, function
arguments, return values), but a fifth — struct field assignment (`p.X = value`) — was not: it
predates BUG-67/68 and was never updated to allow a compile-time constant literal to adapt
losslessly to the field's declared type. `internal/language/data/structs.go`'s `Struct.Set` and
its bytecode-level gatekeeper, `checkStructFieldStrictType` in
`internal/language/bytecode/structs.go`, rejected **any** kind mismatch in strict mode
unconditionally — even a literal that would fit exactly, which every other boundary already
allowed.

**Reproducer:**

```go
func main() {
    type Point struct {
        X int32
    }

    p := Point{X: int32(1)}
    var v int32 = 1

    p.X = 5 // struct field: rejected
    v = 5   // plain variable: succeeds
}
```

Run with `ego --types strict run`. **Actual output:**

```text
Error: at main(line 8), invalid or unsupported data type for this operation: int
Error: terminated with errors
```

**Expected output:** the identical conversion (`int` literal `5` into a declared `int32`)
should behave the same regardless of whether the destination is a struct field or a plain
variable — both should succeed, matching the plain-variable case, which was already correct.

**Notes:**  
Not a new class of bug — the same shape as BUG-67/68 (a coercion boundary missing the shared
`data.CoerceLossless` leniency), just a fifth location neither of those passes touched.

**Resolution (July 2026):**

- `internal/language/bytecode/structs.go`'s `storeIndexByteCode` — the `Store` opcode handler for
  `p.X = value` — now pops the value with `PopWithoutUnwrapping` instead of `Pop`, recording
  whether it was a `data.Immutable`-wrapped compile-time constant, for both the direct
  `*data.Struct` destination case and the `*any` pointer-indirection case (reached through a
  pointer receiver).
- `checkStructFieldStrictType` now takes that `valueIsConst` flag and returns `(any, error)`
  instead of just `error`: when the field-type check fails and the value is a numeric constant,
  it calls `data.CoerceLossless(value, data.InstanceOfType(fieldType))` and returns the coerced
  value for the caller to pass to `Struct.Set`, instead of rejecting outright. A non-constant
  mismatch, or a non-numeric constant, is still rejected exactly as before (BUG-33's original
  enforcement is unaffected).

Regression tests: `tests/types/bug69_struct_field_constant.ego` — a lossless constant now
succeeds and matches the plain-variable case exactly; a lossy constant, a non-constant
mismatch, and a non-numeric constant are all still rejected (guard tests); and the
pointer-receiver (`*any` indirection) code path is exercised separately from the direct
`*data.Struct` path, since both needed the identical fix.

Verified against `go test ./...` and `ego test tests/` / `ego test --types strict tests/`
(1305 `@test` blocks each, up from 1299 after BUG-68) with no regressions, including the
pre-existing BUG-33 struct field-enforcement tests.

