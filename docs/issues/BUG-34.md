# BUG-34 — Scalar pointer equality is broken: `==` and `!=` both return `false` for the same pair

**Severity:** HIGH

**Description:**  
Comparing two scalar pointers (e.g. `*int`) with `==` and with `!=` both produce `false`
for the same pair of pointers, violating the basic invariant that exactly one of `x == y`
and `x != y` must be true. `docs/LANGUAGE.md` line 115 documents that pointer identity
comparison "is not supported" and that `!=` "does not compare addresses," but does not
document that *both* operators silently return `false` unconditionally — that produces
silently-wrong branching logic in real code (e.g. `if pa != pb { ... }` never fires, even
though the two pointers are, in fact, different).

**Reproducer:**

```go
func main() {
    a := 5
    b := 5
    pa := &a
    pb := &b
    fmt.Println(pa == pb)
    fmt.Println(pa != pb)
}
```

**Actual output:**

```text
false
false
```

**Expected output:**

Given that pointer identity comparison is intentionally unsupported, the two operators
should at minimum be consistent negations of each other (or both should raise a clear
compile/runtime error) rather than silently agreeing on a value that makes `!=` unusable.

**Notes:**  
Root cause: `internal/language/bytecode/equal.go`/`notEqual.go` have explicit type-switch
cases only for `*data.Map`, `*data.Array`, `*data.Struct`, and `*data.Type` — there is no
case for scalar Go pointers such as `*int`/`*string`/`*bool` (the representation produced by
`&intVar`, per `internal/language/data/pointers.go:AddressOf`). These fall through to the
`default: genericEqualCompare` path, which finds no matching case in its inner switch and
leaves `result` at its zero value (`false`) for both operators.

**Resolution (July 2026):**  
Investigation found that `data.AddressOf` (the function referenced in the root-cause notes
above, with its per-scalar-type switch producing `*int`, `*string`, etc.) is not actually
called anywhere in the `&` operator's real execution path — it appears to be dead code. The
real implementation, `addressOfByteCode` (`internal/language/bytecode/types.go`), calls
`symbols.SymbolTable.GetAddress`, which always returns a `*any` — a raw Go pointer to the
named variable's storage slot in the symbol table — regardless of what concrete type is
stored there. So every Ego pointer, of any pointed-to type, arrives at `equalByteCode` /
`notEqualByteCode` as a `*any`, which is just as unhandled by the old type switches as the
scalar pointer types originally suspected.

The fix adds a new helper, `isPointerValue` (in `equal.go`), that reports whether a value is
any native Go pointer-kind value (covering `*any`, the double-pointer forms `AddressOf` would
return for Ego's own reference types, and, defensively, the scalar pointer types `AddressOf`
would produce if it were ever wired up). `equalByteCode`'s and `notEqualByteCode`'s `default`
cases now check this first:

- If both operands are pointers, they compare by **address identity** — `v1 == v2` in Go,
  which is safe (cannot panic) because pointer-kind values are always comparable, and
  differently-typed pointers are well-defined as simply unequal, not an error.
- If exactly one operand is a pointer, the result is always "not equal" (a pointer is never
  equal to a non-pointer value).
- Otherwise (neither is a pointer), control falls through to `genericEqualCompare` exactly
  as before.

This makes pointer comparison behave like Go: `pa == pa` is now `true`, `pa == pb` for two
different variables is `false` even when their values are equal, and `==`/`!=` are always
consistent negations of each other. `docs/LANGUAGE.md`'s "pointer identity comparison is not
supported" row was removed from the Go-vs-Ego differences table since this is no longer a
difference.

**Follow-up fix — `typeof()` / `reflect.Type()` on pointers:** while confirming the actual
pointer representation (`*any`) with `reflect.Type(&x)`, a second, related bug was found and
fixed in the same investigation: `internal/builtins/types.go` (`typeOf`, the `typeof()`
built-in) and `internal/runtime/reflect/type.go` (`describeType`, `reflect.Type()`) both had
a `case *any: return data.PointerType(data.InterfaceType), nil` — meaning every pointer,
regardless of what it pointed to, was reported as the generic `*interface{}`. `typeof(&x)`
for an `int` `x` and a `string` `x` were indistinguishable. Both functions now dereference
the pointer and recurse on the pointed-to value, wrapping the result in `data.PointerType(...)`,
so `typeof(&n)` for an `int` `n` now correctly reports `*int`, `reflect.Type(&aBox)` reports
`*Box` (for a struct type `Box`), and so on. A pointer to a not-yet-assigned `interface{}`
slot (dereferences to `nil`) still falls back to the old, generic answer.

Verified with new Go tests in `internal/language/bytecode/equal_test.go` (pointer identity
and consistent-negation cases for both `==` and `!=`), `internal/builtins/types_test.go`
and `internal/runtime/reflect/type_test.go` (pointer-type preservation for `typeof()` /
`reflect.Type()`), new Ego tests in `tests/types/pointer_ops.ego` and
`tests/reflect/type.ego`, and a full run of `tools/gotests.sh` plus `ego test` under all
three typing modes (strict/relaxed/dynamic).

