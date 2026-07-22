# EQUAL-2 — `getComparisonTerms` returns raw coerce error (no location info)

**Affected function:** `getComparisonTerms`  
**File:** `bytecode/equal.go`  
**Risk:** Low — coercion errors during constant-folding lack source location;
in practice `data.Coerce` never fails for two valid numeric values  
**Discovered by:** code review during `equal_test.go` development  
**Status: RESOLVED**

## EQUAL-2: Original behavior

The constant-coercion block returned the error from `data.Coerce` directly,
and the shared `err` variable meant the coerce error was also used for the
stack-pop operations above it:

```go
var err error   // shared with pop calls above
...
if k1 > k2 {
    v2, err = data.Coerce(v2, v1)
} else {
    v1, err = data.Coerce(v1, v2)
}
return v1, v2, err   // ← raw error, no c.runtimeError wrap
```

The `return v1, v2, err` at the end of the function leaked the raw
`data.Coerce` error to callers without module or line annotation, inconsistent
with all other error returns in the package.

## EQUAL-2: Fix

The coerce block was restructured to use a local `coerceErr` variable (not
shared with the pop-error path), explicitly test for failure, and wrap via
`c.runtimeError`:

```go
var coerceErr error

if k1 > k2 {
    v2, coerceErr = data.Coerce(v2, v1)
} else {
    v1, coerceErr = data.Coerce(v1, v2)
}

if coerceErr != nil {
    return nil, nil, c.runtimeError(coerceErr)
}
```

The final `return` was also changed from `return v1, v2, err` to
`return v1, v2, nil` — removing the vestigial use of `err` at that point in
the function, which could never be non-nil after the earlier explicit error
checks.

The coerce success path is exercised by
`Test_equalByteCode_ImmutableCoercion_PromotesConstantToFloat64` and
`Test_equalByteCode_ImmutableCoercion_PromotesConstantToInt32`.  A direct test
of the error wrap is not feasible because `data.Coerce` never returns an error
for two valid numeric values; the fix is defensive.
