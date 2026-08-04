# MATH-3 — `multiplyByteCode` `case uint16:` asserts `v1.(int8)` when v1 is `uint16` — panics

**Affected function:** `multiplyByteCode`  
**File:** `bytecode/math.go`  
**Risk:** High — any multiplication of two `uint16` values panics  
**Discovered by:** `Test_multiplyByteCode_Uint16_MATH3`  
**Status: RESOLVED**

## MATH-3: Description

Identical root cause as MATH-2 but in the `uint16` case:

```go
case uint16:
    return c.push(uint16(v1.(int8)) * uint16(v2.(uint16)))
                         ↑ BUG: v1 is uint16, not int8
```

## MATH-3: Fix

```go
case uint16:
    // MATH-3 fix: original cast v1.(int8) panicked; v1 is uint16
    // after data.Normalize leaves two equal-kind uint16 values unchanged.
    return c.push(v1.(uint16) * v2.(uint16))
```

`Test_multiplyByteCode_Uint16` now asserts `uint16(12)` for `3 * 4`.
