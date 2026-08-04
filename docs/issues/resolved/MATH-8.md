# MATH-8 — `moduloByteCode` `case uint16:` asserts `v1.(int8)` when v1 is `uint16` — panics

**Affected function:** `moduloByteCode`  
**File:** `bytecode/math.go`  
**Risk:** High — any modulo of two `uint16` values panics  
**Discovered by:** `Test_moduloByteCode_Uint16_MATH8`  
**Status: RESOLVED**

## MATH-8: Description

```go
case uint16:
    ...
    return c.push(uint16(v1.(int8)) % uint16(v2.(uint16)))
                          ↑ BUG: v1 is uint16, not int8
```

## MATH-8: Fix

```go
case uint16:
    if v2.(uint16) == 0 {
        return c.runtimeError(errors.ErrDivisionByZero)
    }
    // MATH-8 fix: original cast v1.(int8) panicked; v1 is uint16.
    return c.push(v1.(uint16) % v2.(uint16))
```

`Test_moduloByteCode_Uint16` now asserts `uint16(1)` for `10 % 3`.
