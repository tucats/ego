# CALL-8 — `makeNativeArrayArgument` missing `Int64Kind` and `Float32Kind` for `*data.Array` conversion

**Affected function:** `makeNativeArrayArgument`  
**File:** `bytecode/callNative.go`  
**Risk:** Low — passing a `*data.Array` of int64 or float32 elements to a
native function expecting `[]int64` or `[]float32` silently returned
`ErrInvalidType` instead of converting correctly  
**Discovered by:** `Test_makeNativeArrayArgument_Int64Kind`,
`Test_makeNativeArrayArgument_Float32Kind`  
**Status: RESOLVED**

## CALL-8: Original behavior

`makeNativeArrayArgument` converts a `*data.Array` to the equivalent native
Go slice so it can be passed to a Go function via reflection.  The switch on
element kind handled: `IntKind`, `Int16Kind`, `UInt16Kind`, `Int32Kind`,
`BoolKind`, `ByteKind`, `Float64Kind`, and `StringKind`.

Two kinds were absent: `Int64Kind` and `Float32Kind`.  A `*data.Array` of
int64 or float32 elements fell through to the `default` case and returned
`ErrInvalidType`.  The asymmetry was visible because native `[]int64` and
`[]float32` slices were already handled by direct pass-through, and
`convertFromNativeArray` already converted `[]int64` and `[]float32` back to
`*data.Array` on the return path.

## CALL-8: Fix

Two new cases were added to the switch, each with a clear comment explaining
why they were previously absent:

```go
case data.Float32Kind:
    // Added by CALL-8 fix — was missing despite []float32 pass-through above.
    arrayArgument := make([]float32, arg.Len())
    for i := 0; i < arg.Len(); i++ {
        v, _ := arg.Get(i)
        arrayArgument[i], err = data.Float32(v)
        ...
    }

case data.Int64Kind:
    // Added by CALL-8 fix — mirrors the existing []int64 return-path support.
    arrayArgument := make([]int64, arg.Len())
    ...
```

`Test_makeNativeArrayArgument_Int64Kind` and
`Test_makeNativeArrayArgument_Float32Kind` now assert that the conversion
succeeds and produces the expected concrete slice type.
