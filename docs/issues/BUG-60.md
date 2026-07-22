# BUG-60 — Type assertion to a function type fails to compile — **Resolved**

**Severity:** LOW

**Description:**  
Found while writing Go unit tests for the BUG-30 fix: asserting an `interface{}` value to a
function type (`x.(func() int)`) does not compile, even though Go allows a type assertion
target to be any type, including a function type. The workaround used in the BUG-30 tests
was to avoid the assertion entirely and rely on Ego's dynamic typing to call the value
directly.

**Reproducer:**

```go
func main() {
    var x any = func() int { return 42 }
    v := x.(func() int)
    fmt.Println(v())
}
```

**Actual output:**

```text
Error: at line 3:10, invalid identifier
Error: terminated with errors
```

**Expected output:**

```text
42
```

**Notes:**  
The same failure occurs whether the asserted-to function type appears alone
(`x.(func() int)`) or as part of a larger expression (`x.(func() int)()`), and regardless of
the function type's signature (no parameters, parameters, multiple returns). This is
distinct from a *value* of a named function type failing — declaring and calling a plain
`func() int` variable works fine; only using one as the *target type* of a type assertion
fails. Not yet root-caused against a specific line in the type-assertion parser
(`internal/language/compiler/expr_atom.go` and/or `internal/language/compiler/types.go` are
the likely locations, based on where other type-assertion target parsing lives), since this
was discovered incidentally rather than through a dedicated investigation.

**Resolution (July 2026):**

Two independent bugs had to be fixed for `x.(func() int)` to actually work end to end, not
just compile:

- **Compile-time parsing** — `compileUnwrap` (`internal/language/compiler/unwrap.go`)
  previously matched the assertion target with a hand-rolled check for exactly one
  `IDENTIFIER` token followed by `)`. `func` is a keyword token, not an `IDENTIFIER`, so any
  function-type target failed immediately with "invalid identifier" — before compilation ever
  reached a point where the signature (no parameters vs. parameters vs. multiple returns)
  could matter, which is why the bug appeared signature-independent. The fix retries, when the
  single-token check doesn't match, using `parseType("", true)` — the same general-purpose
  type parser `T(value)` type-cast expressions already use — which recognizes function types
  (and, as a side effect, pointer, slice, map, struct, and `interface{}` literal types, none of
  which compiled as assertion targets before either). Since a compound type has no single name
  to look up by string, the resolved `*data.Type` is passed as the `UnWrap` operand directly,
  instead of a name; `unwrapByteCode` (`internal/language/bytecode/types.go`) was extended to
  recognize a `*data.Type` operand and use it as-is, bypassing the by-name lookup used for the
  existing plain-identifier case (which is unchanged, to avoid any risk to already-working
  named-type and `.(type)` type-switch behavior).
- **Runtime type recognition** — fixing the parser exposed a second, independent bug: even
  with the corrected syntax, the assertion still failed at runtime with "type mismatch",
  because `data.TypeOf()` (`internal/language/data/types.go`) had no case for a compiled Ego
  function/closure value (represented at runtime as `*bytecode.ByteCode`, a type the `data`
  package cannot import without creating an import cycle) and fell through to its default
  case, misreporting every function/closure value as `interface{}`. This also meant
  `reflect.TypeOf(f)` on any Ego function value was wrong, independent of type assertions.
  Fixed by having `TypeOf`'s default case structurally detect any value exposing a
  `Declaration() *Declaration` method — which `*bytecode.ByteCode` does — and build a
  `FunctionKind` type from the returned declaration, without needing to import `bytecode` at
  all.

The related, but out-of-scope, limitations found while verifying this fix are tracked
separately as [BUG-70](#BUG-70) and [BUG-71](#BUG-71).

Regression tests: `tests/types/type_assertions.ego` gained eight new `@test` blocks covering
the original reproducer (including calling the asserted function immediately), a multi-return
function type, the comma-ok form (both matching and non-matching), and the bonus pointer/slice/
`interface{}` compound-type fixes, plus an explicit regression check that the `.(type)`
type-switch form (which shares `compileUnwrap`) is unaffected. Verified against `go test ./...`
and `ego test tests/` / `ego test --types strict tests/` / `ego test --types relaxed tests/`
(1313 `@test` blocks, up from 1305) with no regressions.

