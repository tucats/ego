# COMPARE-3 — `int8` missing from signed-integer case in four ordering functions

**Affected functions:** `lessThanByteCode`, `greaterThanByteCode`,
`lessThanOrEqualByteCode`, `greaterThanOrEqualByteCode`  
**Files:** `bytecode/` — one source file per operator  
**Risk:** Low — ordering comparisons on `int8` values return `ErrInvalidType`
instead of a boolean result  
**Discovered by:** `Test_lessThanByteCode_Int8`,
`Test_greaterThanByteCode_Int8`,
`Test_lessThanOrEqualByteCode_Int8`,
`Test_greaterThanOrEqualByteCode_Int8`  
**Status: RESOLVED**

## COMPARE-3: Description

Every ordering function had an inner switch that handled signed integers via
`int64`-based comparison:

```go
// lessThanByteCode (identical pattern in >, <=, >=):
case byte, int32, int16, int, int64:   // ← int8 was missing
    x1, err := data.Int64(v1)
    x2, err := data.Int64(v2)
    result = x1 < x2
```

`int8` was absent.  After normalization two `int8` values remained `int8`, the
inner switch found no match, and the `default` case returned `ErrInvalidType`.

Compare with `genericEqualCompare` (inside `equalByteCode`) which correctly
listed `int8`:

```go
case byte, int8, int16, int32, int, int64:   // int8 present ✓
```

And `notEqualByteCode` which also included `int8` in its inner switch.

## COMPARE-3: Fix

`int8` was added to the signed-integer case in all four functions:

```go
case byte, int8, int32, int16, int, int64:
```

The four `_CurrentlyBroken` tests were renamed (dropping the suffix) and
updated to assert `nil` error and the correct boolean result.
