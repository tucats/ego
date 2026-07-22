# BUILTIN-MAKE-1 — `Make` did not validate that `size` is non-negative

**Affected function:** `Make`
**File:** `builtins/make.go`
**Risk:** High
**Status: RESOLVED**

## Original MAKE-1 behavior

Passing a negative size to `Make` caused Go's runtime to panic with
"makeslice: len out of range" — an unrecoverable error that bypassed Ego's
`try/catch` mechanism.

## Fix for MAKE-1

A bounds check was added immediately after the size conversion:

```go
if size < 0 {
    return nil, errors.ErrInvalidValue.In("make").Context(size)
}
```

**Tests:** `Test_Make_NegativeSizeReturnsError`,
`Test_Make_ZeroSizeReturnsEmptyArray`
