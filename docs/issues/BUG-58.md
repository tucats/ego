# BUG-58 — The `uint8()` cast function is not recognized despite being documented

**Severity:** LOW  **Status:** Fixed

**Description:**  
`docs/LANGUAGE.md` line 1115 explicitly lists `uint8()` (e.g. `uint8(240)`) as a valid cast
function. Calling it fails with an unknown-symbol error. `byte(5)`, its documented-equivalent
alias, works correctly, as do all other numeric cast functions (`byte`, `int8`, `int16`,
`uint16`, `int32`, `uint32`, `int`, `uint`, `int64`, `uint64`, `float32`, `float64`, `bool`,
`string`).

**Correction to an earlier note in this entry:** this write-up originally also claimed
`var x uint8` works correctly as a *type* declaration, with only the cast function missing.
Re-verified while fixing this bug: that claim was wrong — `var x uint8` failed with the exact
same `unknown symbol: uint8` error as the cast form, for the same underlying reason (see Fix
below). Both are fixed by the same change.

**Reproducer:**

```go
func main() {
    fmt.Println(uint8(5))
}
```

**Actual output:**

```text
Error: at line 2:12, unknown symbol: uint8
```

**Expected output:**

```text
5
```

**Notes:**  
Trivial, low-risk fix: register `uint8` as a cast-function alias for `byte`, matching how
the type name `uint8` is already recognized in variable declarations.

**Fix:**  
Root cause: both `var x uint8` declarations and `uint8(...)` cast-call expressions resolve a
type name through the exact same table, `data.TypeDeclarations`
(`internal/language/data/declarations.go`), linearly scanned by
`Compiler.compileKnownBaseType` (`internal/language/compiler/typeCompiler.go`) — the type
compiler's `parseType` calls it directly for `var` declarations, and the expression compiler's
`compileTypeCast` (`internal/language/compiler/expr_atom.go`) calls `parseType` too when it
sees `name(` at the start of an expression. The table had no `"uint8"` entry at all — only
`"byte"` — so neither path recognized it, and both fell through to plain symbol resolution,
producing the `unknown symbol: uint8` error. A `UInt8Token` already existed in
`internal/language/tokenizer/reserved.go`, but nothing in the compiler ever referenced it; it
was dead code, unconnected to type resolution.

Fixed by adding one entry to `TypeDeclarations` for `"uint8"` that points at the *same*
`byteModel`/`ByteType` singleton `"byte"` already uses, rather than defining a distinct type —
`uint8` is Go's real, canonical name for the type Go (and Ego) also spell `byte`, so they
should be indistinguishable, not merely convertible. `typeof(uint8(5))` correctly reports
`byte`, and `byte(5) == uint8(5)` is `true`.

Regression tests: `tests/types/scalar_types.ego` — `"types: uint8 zero value"` (replacing a
stale comment that had documented the missing feature as a permanent limitation) and
`"types: uint8 cast function is recognized (BUG-58)"`.

