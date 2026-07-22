# `TRYCATCH-1` — `willCatchByteCode` panics on negative integer operands

**Affected function:** `willCatchByteCode`  
**File:** `bytecode/try.go`  
**Risk:** Medium — a negative integer operand slipped past the upper-bound guard
and caused a runtime index-out-of-bounds panic when accessing `catchSets`  
**Discovered by:** `Test_willCatchByteCode_NegativeInt_ReturnsError`  
**Status: RESOLVED**

## `TRYCATCH-1`: Original behavior

`willCatchByteCode` checked whether the integer operand was within the bounds
of the `catchSets` slice using only an upper-bound guard:

```go
if i > len(catchSets) {   // only rejects values too large
    return c.runtimeError(errors.ErrInternalCompiler)...
}
...
try.catches = append(try.catches, catchSets[i-1]...)
```

Negative values passed the guard (`-1 > 1` is false) and then caused an
unrecoverable panic when `catchSets[i-1]` accessed the slice at a negative
index:

```text
panic: runtime error: index out of range [-2]
```

## `TRYCATCH-1`: Fix

The guard was extended to check both directions, and a comment was added
explaining the valid range of operand values:

```go
// Valid values: 0 (catch-all), 1..len(catchSets) (named sets).
// Negative values slipped past the original guard — TRYCATCH-1 fix.
if i < 0 || i > len(catchSets) {
    return c.runtimeError(errors.ErrInternalCompiler).Context(...)
}
```

`Test_willCatchByteCode_NegativeInt_ReturnsError` confirms that `-1` now
returns `ErrInternalCompiler` without panicking.
