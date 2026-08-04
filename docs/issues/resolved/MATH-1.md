# MATH-1 — `exponentByteCode` returns 0 for signed integer `x^0` (should return 1)

**Affected function:** `exponentByteCode`  
**File:** `bytecode/math.go`  
**Risk:** Medium — `x^0` for any signed integer type silently returns 0; the
correct mathematical result is 1 for all non-zero bases  
**Discovered by:** `Test_exponentByteCode_SignedInt_PowerZero_CurrentlyBroken_MATH1`  
**Status: RESOLVED**

## MATH-1: Description

The signed-integer exponentiation path (matching `byte, int8, int16, int32, int,
int64`) contained an explicit fast-exit for the exponent-zero case:

```go
if vv2 == 0 {
    return c.push(0)   // BUG: x^0 should be 1
}
```

The unsigned-integer path directly below it handled the same case correctly:

```go
if vv2 == 0 {
    return c.push(uint64(1))   // was already correct
}
```

## MATH-1: Fix

Changed the signed-integer zero-exponent return to push `int64(1)`, matching the
type that the multiplication loop returns on success:

```go
if vv2 == 0 {
    // MATH-1 fix: x^0 == 1 for all non-zero bases.  The previous code
    // pushed the untyped literal 0 (which becomes int(0)), giving the
    // mathematically wrong result.  int64(1) matches the type that the
    // success path pushes after the multiplication loop.
    return c.push(int64(1))
}
```

`Test_exponentByteCode_SignedInt_PowerZero` now asserts `int64(1)` and confirms
the correct result.
