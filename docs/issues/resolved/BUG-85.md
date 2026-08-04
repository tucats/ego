# BUG-85 — `uuid.UUID`'s zero value (`var x uuid.UUID`) is an unusable struct with no native field, unlike Go's own valid nil-UUID zero value

**Severity:** MEDIUM

**Description:**  
Found during a documentation review of the `uuid` package. In Go, `uuid.UUID` is a
`[16]byte` array, so its zero value (`var x uuid.UUID`) is automatically the valid nil
UUID -- every method (`String()`, etc.) works on it with no special handling. Ego wraps
`uuid.UUID` as a `*data.Struct` carrying the real Go value under a "native" field
(`data.NewStruct(UUIDTypeDef).SetNative(...)`), and `UUIDTypeDef` had no registered
zero-value constructor (`Type.SetNew`, unused anywhere else in the codebase before this
fix). Without one, `Type.InstanceOf` -- the path `var` declarations and `new()` use to
build a zero value -- falls back to its generic `StructKind` case, an empty
`NewStruct(t)` with no native field at all. Every method that reads the native field
(`String()`, `Gibberish()`) then failed:

```go
var x uuid.UUID
fmt.Println(x.String())
// Error: invalid field name for type: native value

fmt.Println(x == uuid.Nil())
// false -- structurally different: x has no native field, uuid.Nil() has one
// (all zeros), so raw struct equality never matches even though both
// "should" represent the same nil UUID
```

**Fix:**  
Added an `init()` function in `internal/runtime/uuid/types.go` that calls
`UUIDTypeDef.SetNew(...)`, registering a constructor that returns
`data.NewStruct(UUIDTypeDef).SetNative(uuid.Nil)` -- the same shape `uuid.Nil()` already
produces. This has to be a separate `init()` rather than part of `UUIDTypeDef`'s own var
initializer, since a closure referencing `UUIDTypeDef` inside its own initializer
expression is an initialization cycle; `init()` runs after the var is fully assigned, so
the self-reference is fine there. `Type.InstanceOf` already had the necessary dispatch
(`if t.nativeName != "" && t.newFunction != nil { return t.New() }`) -- it was simply
never wired up for any type in the codebase before this fix.

Regression test `TestUUIDTypeDef_ZeroValueIsUsable`
(`internal/runtime/uuid/uuid_test.go`) calls `UUIDTypeDef.InstanceOf(UUIDTypeDef)`
directly and asserts `String()` on the result succeeds and returns the nil UUID text —
verified it fails with the original "invalid field name" error when the `init()` fix is
reverted. `tests/packages/uuid.ego` adds an Ego-level test asserting both
`x.String() == "00000000-0000-0000-0000-000000000000"` and `x == uuid.Nil()` for a bare
`var x uuid.UUID`.

