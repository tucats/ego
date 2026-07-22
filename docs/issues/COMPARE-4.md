# COMPARE-4 — Four comparison operators returned raw errors without `c.runtimeError` decoration

**Affected functions:** `lessThanByteCode`, `lessThanOrEqualByteCode`,
`greaterThanOrEqualByteCode`, `notEqualByteCode`  
**Files:** `bytecode/lessThan.go`, `bytecode/lessThanorEqual.go` (sic — "or" is lowercase in the file name),
`bytecode/greaterThanorEqual.go` (sic), `bytecode/notEqual.go`  
**Risk:** Low — error messages lack source-location context; correctness is not
affected  
**Discovered by:** comprehensive test audit in `bytecode/compare_test.go`
(Section 9–13 stack-underflow tests)  
**Status: RESOLVED**

## COMPARE-4: Original behavior

Every error return in the bytecode package is expected to be wrapped via
`c.runtimeError(err)`, which attaches the current module name and source line
so that error messages displayed to the user include a precise location.

`greaterThanByteCode` was already correct:

```go
// greaterThanByteCode — correct:
v1, v2, err := getComparisonTerms(c, i)
if err != nil {
    return c.runtimeError(err)   // ← decorated
}
...
v1, v2, err = data.Normalize(v1, v2)
if err != nil {
    return c.runtimeError(err)   // ← decorated
}
```

The other four comparison functions returned these errors raw:

```go
// lessThanByteCode, lessThanOrEqualByteCode, greaterThanOrEqualByteCode,
// and notEqualByteCode — original (buggy):
v1, v2, err := getComparisonTerms(c, i)
if err != nil {
    return err    // ← raw; no module/line annotation
}
...
v1, v2, err = data.Normalize(v1, v2)
if err != nil {
    return err    // ← raw; no module/line annotation
}
```

This meant that a stack-underflow error (`ErrStackUnderflow`) from any of those
four operators appeared without the source location that `greaterThanByteCode`'s
identical error would include.

## COMPARE-4: Fix

Both raw `return err` sites in each of the four affected functions were changed
to `return c.runtimeError(err)`, making the error-decoration pattern uniform
across all six comparison operators.

The four stack-underflow tests added in Sections 9–13 of `compare_test.go`
document this fix by asserting `ErrStackUnderflow` for each previously
inconsistent operator.
