# MATH-6 — `divideByteCode` `case uint16:` asserts `v1.(int8)` when v1 is `uint16` — panics

**Affected function:** `divideByteCode`  
**File:** `bytecode/math.go`  
**Risk:** High — any division of two `uint16` values panics  
**Discovered by:** `Test_divideByteCode_Uint16_MATH6`  
**Status: RESOLVED**

## MATH-6: Description

```go
case uint16:
    ...
    return c.push(uint16(v1.(int8)) / uint16(v2.(uint16)))
                          ↑ BUG: v1 is uint16, not int8
```

## MATH-6: Fix

```go
case uint16:
    if v2.(uint16) == 0 {
        return c.runtimeError(errors.ErrDivisionByZero)
    }
    // MATH-6 fix: original cast v1.(int8) panicked; v1 is uint16.
    return c.push(v1.(uint16) / v2.(uint16))
```

`Test_divideByteCode_Uint16` now asserts `uint16(3)` for `9 / 3`.
