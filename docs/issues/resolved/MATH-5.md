# MATH-5 — `divideByteCode` `case int16:` asserts `v1.(int8)` when v1 is `int16` — panics

**Affected function:** `divideByteCode`  
**File:** `bytecode/math.go`  
**Risk:** High — any division of two `int16` values panics  
**Discovered by:** `Test_divideByteCode_Int16_MATH5`  
**Status: RESOLVED**

## MATH-5: Description

```go
case int16:
    ...
    return c.push(int16(v1.(int8)) / int16(v2.(int16)))
                         ↑ BUG: v1 is int16, not int8
```

## MATH-5: Fix

```go
case int16:
    if v2.(int16) == 0 {
        return c.runtimeError(errors.ErrDivisionByZero)
    }
    // MATH-5 fix: original cast v1.(int8) panicked; v1 is int16.
    return c.push(v1.(int16) / v2.(int16))
```

`Test_divideByteCode_Int16` now asserts `int16(3)` for `9 / 3`.
