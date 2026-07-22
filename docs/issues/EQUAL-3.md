# EQUAL-3 — `case nil:` branch in `equalByteCode`'s switch is dead code

**Affected function:** `equalByteCode`  
**File:** `bytecode/equal.go`  
**Risk:** None — the code is unreachable and has no runtime effect  
**Discovered by:** `Test_equalByteCode_NilNilHandledByGuard`,
`Test_equalByteCode_NilOneNilHandledByGuard`  
**Status: RESOLVED**

## EQUAL-3: Original behavior

`equalByteCode` contained two nil guards that ran before the type switch:

```go
if data.IsNil(v1) && data.IsNil(v2) {
    return c.push(true)
}
if data.IsNil(v1) || data.IsNil(v2) {
    return c.push(false)
}
```

`data.IsNil(nil)` returns `true` for a pure Go nil interface value, and
`getComparisonTerms` unwraps any `data.Immutable{Value: nil}` to a bare `nil`
before the guards run.  As a result, by the time the
`switch actual := v1.(type)` statement executed, `v1` was guaranteed to be
non-nil — and the switch contained:

```go
case nil:
    if err, ok := v2.(error); ok {
        result = errors.Nil(err)
    } else {
        result = (v2 == nil)
    }
```

This branch could never be reached.

## EQUAL-3: Fix

The `case nil:` branch was removed from the type switch.  The function-level
comment on `equalByteCode` was updated to explain the nil-guard invariant
explicitly, so the absence of the case is documented:

```go
// Nil handling is resolved by two guards before the type switch runs:
//   - both nil  → push true
//   - one nil   → push false
//
// These two guards also mean that by the time the type switch executes, v1
// is guaranteed to be non-nil.  There is therefore no `case nil:` branch in
// the switch — such a branch would be unreachable dead code (EQUAL-3).
```

`Test_equalByteCode_NilNilHandledByGuard` and
`Test_equalByteCode_NilOneNilHandledByGuard` confirm that nil comparisons
still produce the correct bool results after the dead code was removed.
