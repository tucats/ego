# OPTIMIZER-2 — `reflect.DeepEqual` for operand comparison is unnecessarily expensive

**Affected function:** `optimize`  
**File:** `bytecode/optimizer.go`  
**Risk:** Performance — `reflect.DeepEqual` is called for every non-string,
non-placeholder operand pair; it uses reflection even for simple integers  
**Status: RESOLVED**

## OPTIMIZER-2: Description

The pattern-match loop used `reflect.DeepEqual` for every non-string,
non-placeholder operand comparison.

## OPTIMIZER-2: Fix

The dedicated `operandEqual(a, b any) bool` helper was added.  It uses a
type-switch for `int`, `int64`, `float64`, `bool`, `string`, and `nil`,
falling back to `reflect.DeepEqual` only for composite types such as
`StackMarker` (which contains a `[]any` field and cannot be compared with `==`
directly).  The separate fast-path string check in the old match loop was
removed; `operandEqual` subsumes it.  `reflect` is still imported for the
fallback path.
