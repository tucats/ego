# CREATE-2 — `addMissingFields` inverted error check skipped coerced-value write-back

**Affected function:** `addMissingFields`  
**File:** `bytecode/create.go`  
**Risk:** Medium — when a struct field has a coercible but mismatched type, the
coerced value was silently discarded and the field retained its original type  
**Discovered by:** code audit during `create_test.go` comprehensive review  
**Status: RESOLVED**

## CREATE-2: Description

`addMissingFields` coerces existing field values to the type declared in the
struct model.  The post-coercion error check was inverted:

```go
existingValue, err = data.Coerce(existingValue, fieldModel)
if err == nil {
    return err   // ← returned nil on SUCCESS, exiting without updating structMap
}

structMap[fieldName] = existingValue  // ← only reached on FAILURE
```

When coercion succeeded (`err == nil`):

- The function returned `nil` (no error) before writing the coerced value back.
- `structMap[fieldName]` retained the original pre-coercion value.

When coercion failed (`err != nil`):

- The function fell through to `structMap[fieldName] = existingValue`,
  storing the **un-coerced** value.

## CREATE-2: Accessibility constraint

The coercion block is guarded by `ft.Kind() != data.UndefinedKind`.
`data.Type.Field()` returns `UndefinedType` (kind = `UndefinedKind`) for any
type that is not a raw `StructKind` — including `TypeDefinition` wrappers
(kind = `TypeKind`).  The coercion path is therefore only reachable when the
struct model was created from a raw `data.StructureType`, not from a named
`data.TypeDefinition`.  The test uses a raw `StructureType` to ensure the path
is exercised.

## CREATE-2: Fix

The condition was corrected from `err == nil` to `err != nil`:

```go
existingValue, err = data.Coerce(existingValue, fieldModel)
if err != nil {
    return err   // bail out on failure
}

structMap[fieldName] = existingValue  // reached on success — stores coerced value
```

`Test_addMissingFields_FieldTypeCoercion_CREATE2` confirms that a `float64`
value in a field declared as `int` is coerced to `int` and written back.
`float64` is used rather than `int32` because `data.TypeOf(int32).IsType(IntType)`
returns `true` in Ego's type system (both are integer kinds), which would bypass
the coercion block entirely.
