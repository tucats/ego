# MATH-10 — `addByteCode` function comment incorrectly says "OR" for boolean operands

**Affected function:** `addByteCode`  
**File:** `bytecode/math.go`  
**Risk:** None — documentation only; the implementation is correct  
**Discovered by:** `Test_addByteCode_BoolAND_TrueAndTrue`,
`Test_addByteCode_BoolAND_MixedValues`  
**Status: RESOLVED**

## MATH-10: Description

The function-level comment on `addByteCode` said "OR" but the implementation
performs logical AND (`&&`):

```go
case bool:
    return c.push(v1.(bool) && v2.(bool))
```

## MATH-10: Fix

The comment was corrected and a cross-reference to `multiplyByteCode` (which
performs OR) was added:

```go
// addByteCode bytecode instruction processor. This removes the top two
// items and adds them together. For boolean values, this is an AND (&&)
// operation — note that multiplyByteCode performs OR (||) for booleans.
// MATH-10 fix: the previous comment incorrectly said "OR"; the implementation
// uses && (AND), which is what this comment now reflects.
```
