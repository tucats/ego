# MATH-12 — `negateByteCode` has no case for `time.Duration`, so `-d` fails

**Affected function:** `negateByteCode`  
**File:** `bytecode/math.go`  
**Risk:** Medium — arithmetic negation of a `time.Duration` value (e.g. to build a
"before" offset from a "since" duration) fails outright, even though a `Duration` is
just an `int64` in Go and negating one is meaningful  
**Discovered by:** manual testing while adding `time.Time.SleepUntil()` and
constructing a target time in the past to test its no-op behavior  
**Status: RESOLVED**

## MATH-12: Description

`negateByteCode`'s arithmetic path is a Go type switch on the popped value's
concrete type (`byte`, `int16`, `uint16`, ..., `int64`, `float32`, `float64`, ...).
Go's type switch requires an exact type match — `time.Duration` (`type Duration
int64`) does not match a `case int64:` arm even though its underlying
representation is identical. With no `case time.Duration:` arm, negation fell
through to `default: return c.runtimeError(errors.ErrInvalidType)`.

```go
d, _ := time.ParseDuration("500ms")
neg := -d
// Error: invalid or unsupported data type for this operation
```

## MATH-12: Fix

Added a `case time.Duration:` arm (importing stdlib `"time"` into `math.go`),
matching the same `isConstant`-aware pattern used by the adjacent integer cases:

```go
case time.Duration:
    value = -value
    if isConstant {
        return c.push(data.Constant(value))
    }

    return c.push(value)
```

`Test_negateByteCode_TimeDuration` (`bytecode/math_test.go`) asserts
`-3s` negates to `3s`.
