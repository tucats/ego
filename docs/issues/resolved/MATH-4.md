# MATH-4 — `subtractByteCode` `case int8:` asserts `v1.(int16)` when v1 is `int8` — panics

**Affected function:** `subtractByteCode`  
**File:** `bytecode/math.go`  
**Risk:** High — any subtraction of two `int8` values panics  
**Discovered by:** `Test_subtractByteCode_Int8_MATH4`  
**Status: RESOLVED**

## MATH-4: Description

When two `int8` values are on the stack, `data.Normalize` left them as `int8`
(same kind).  The type switch entered `case int8:`, and the body asserted `v1.(int16)`:

```go
case int8:
    return c.push(int8(v1.(int16)) - int8(v2.(int8)))
                      ↑ BUG: v1 is int8, not int16
```

## MATH-4: Fix

```go
case int8:
    // MATH-4 fix: original cast v1.(int16) panicked because after
    // data.Normalize both values remain int8 (same kind is unchanged).
    return c.push(v1.(int8) - v2.(int8))
```

`Test_subtractByteCode_Int8` now asserts `int8(2)` for `5 - 3`.
