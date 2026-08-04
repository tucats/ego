# BUG-88 — Function-typed return collapses its coercion target to NilType

**Severity:** MEDIUM

**Description:**  
Under `--types strict`, any function that returns a function value failed at
runtime with a spurious `type mismatch`:

```ego
func makeAdder(base int) func(int) int {
    return func(x int) int { return base + x }
}
add10 := makeAdder(10)   // Error: at makeAdder(...), type mismatch: func(x int) int, nil
```

The compiler emits a `Coerce` instruction per declared return type so a returned
value is checked/converted to the declared type. In `compileReturnTypes`
(`internal/language/compiler/function.go`) that type was obtained by
round-tripping the parsed type through a zero-value **instance**:
`data.TypeOf(data.InstanceOfType(parsedType))`. This canonicalization works for
scalars (`int → 0 → IntType`) and user types, but a function type's zero value
is `nil` — `data.InstanceOfType` returns `nil` for `FunctionKind` (it has no
model) — so `data.TypeOf(nil)` collapsed to `NilType`. The `Coerce` was then
told to coerce the returned closure to `nil`, and strict-mode `requireMatch`
rejected every value with `type mismatch: <closure type>, nil`.

**Fix:**  
`compileReturnTypes` now parses the return type directly with `c.parseType`
and only applies the instance round-trip when the instance is non-`nil`,
falling back to the parsed type otherwise:

```go
t := theType
if k := data.InstanceOfType(theType); k != nil {
    t = data.TypeOf(k)
}
```

This preserves the existing canonicalization for scalar and user-defined types
while using the real function type as the coercion target. The now-unused
`typeDeclaration` helper (`internal/language/compiler/type.go`) — the only
caller of the old round-trip — was removed.

Regression coverage: `tests/functions/return_types.ego` — the
"returns: function type, unnamed, returns a closure", "returns: function type,
named", "returns: function type with multiple parameters", "returns: function
type with no parameters", and "returns: function type returning multiple values"
tests, all of which run under both `--types=strict` and `--types=dynamic`.

