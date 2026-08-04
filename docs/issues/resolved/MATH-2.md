# MATH-2 — `multiplyByteCode` `case int16:` asserts `v1.(int8)` when v1 is `int16` — panics

**Affected function:** `multiplyByteCode`  
**File:** `bytecode/math.go`  
**Risk:** High — any multiplication of two `int16` values causes an unrecoverable
runtime panic  
**Discovered by:** `Test_multiplyByteCode_Int16_MATH2`  
**Status: RESOLVED**

## MATH-2: Description

After `data.Normalize` leaves two `int16` values unchanged (equal kinds), the
type switch entered `case int16:`.  The body asserted `v1.(int8)`:

```go
case int16:
    return c.push(int16(v1.(int8)) * int16(v2.(int16)))
                       ↑ BUG: v1 is int16, not int8
```

## MATH-2: Fix

```go
case int16:
    // MATH-2 fix: original cast v1.(int8) panicked because v1 is int16
    // after data.Normalize leaves two equal-kind values unchanged.
    return c.push(v1.(int16) * v2.(int16))
```

`Test_multiplyByteCode_Int16` now asserts `int16(12)` for `3 * 4`.
