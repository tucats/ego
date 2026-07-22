# CALL-10 — `synthesizeDefinition` sets `MinArgCount = -1` for zero-parameter variadic functions

**Affected function:** `synthesizeDefinition`  
**File:** `bytecode/callRuntimeFunction.go`  
**Risk:** Low — the condition `len(args) < -1` is never true, so the -1 value
effectively permitted any number of arguments; the practical behavior was
correct by accident, but the intended minimum of 0 was not expressed  
**Discovered by:** `Test_synthesizeDefinition_Variadic_ZeroParams`  
**Status: RESOLVED**

## CALL-10: Original behavior

`synthesizeDefinition` computed the minimum argument count for a variadic
function as `len(Parameters) - 1`:

```go
} else {
    definition.MinArgCount = len(savedDefinition.Declaration.Parameters) - 1
    definition.MaxArgCount = 99999
}
```

The formula was correct for the common case — a two-parameter variadic
`func f(a int, b ...int)` with `len(params) = 2` yielded `MinArgCount = 1`.

When `len(Parameters) == 0` the formula yielded `-1`.  The guard
`len(args) < -1` was never true, so any call passed — correct behavior, but
achieved by accident rather than by design.

## CALL-10: Fix

A clamp was added immediately after the formula, with a comment explaining the
zero-param edge case:

```go
minCount := len(savedDefinition.Declaration.Parameters) - 1
// Clamp: a variadic function with no declared parameters requires at
// least 0 arguments, not -1 (CALL-10 fix).
if minCount < 0 {
    minCount = 0
}
definition.MinArgCount = minCount
```

`Test_synthesizeDefinition_Variadic_ZeroParams` now asserts
`MinArgCount == 0` instead of `-1`, and a comment in the source explains
why the clamp is needed.
