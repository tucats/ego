# MATH-7 — `moduloByteCode` `case int16:` asserts `v1.(int8)` when v1 is `int16` — panics

**Affected function:** `moduloByteCode`  
**File:** `bytecode/math.go`  
**Risk:** High — any modulo of two `int16` values panics  
**Discovered by:** `Test_moduloByteCode_Int16_MATH7`  
**Status: RESOLVED**

## MATH-7: Description

```go
case int16:
    ...
    return c.push(int16(v1.(int8)) % int16(v2.(int16)))
                         ↑ BUG: v1 is int16, not int8
```

## MATH-7: Fix

```go
case int16:
    if v2.(int16) == 0 {
        return c.runtimeError(errors.ErrDivisionByZero)
    }
    // MATH-7 fix: original cast v1.(int8) panicked; v1 is int16.
    return c.push(v1.(int16) % v2.(int16))
```

`Test_moduloByteCode_Int16` now asserts `int16(1)` for `10 % 3`.
