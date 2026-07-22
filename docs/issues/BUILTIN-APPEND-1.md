# BUILTIN-APPEND-1 — `Append` skipped type inference when first arg is a raw `[]any`

**Affected function:** `Append`
**File:** `builtins/append.go`
**Risk:** Low
**Status: RESOLVED**

## Original behavior

When the first argument was a raw `[]any` slice (rather than a `*data.Array`),
the element type `kind` was never updated from its initial `data.InterfaceType`
value.  The returned array was always typed as `[]interface{}` regardless of
the actual element types.

## Fix

After flattening the `[]any` into the result, `Append` now inspects the first
element and promotes `kind` to the uniform element type when all elements agree.
If the slice is empty or elements have mixed types, `kind` stays as
`InterfaceType` (the correct representation for a heterogeneous array):

```go
if len(array) > 0 && kind.IsInterface() {
    candidate := data.TypeOf(array[0])
    uniform := true
    for _, elem := range array[1:] {
        if !data.TypeOf(elem).IsType(candidate) {
            uniform = false
            break
        }
    }
    if uniform {
        kind = candidate
    }
}
```

**Tests:** `Test_Append_RawGoSliceUniformTypeInferred`,
`Test_Append_RawGoSliceMixedTypesStaysInterface`
