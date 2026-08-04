# RANGE-5 — `rangeInitByteCode` appends a stale entry to `rangeStack` even when the type-switch default sets an error

**Affected function:** `rangeInitByteCode`  
**File:** `bytecode/range.go`  
**Risk:** Low — only triggered by unsupported value types (already caught by
the type guard); a try/catch block could observe the stale entry  
**Discovered by:** `Test_rangeInitByteCode_UnsupportedType`  
**Status: RESOLVED**

## RANGE-5: Description

After the type-switch, `r.index = 0` and the `rangeStack` append ran
unconditionally even when the `default` case had already set an error.  A
`try/catch` block that caught the error and then executed `RangeNext` would
find the stale entry with an invalid value type.

## RANGE-5: Fix

The `rangeStack` push is now guarded by `if err == nil`, and the RANGE-3
`scopeDepth` assignment was incorporated into the same block:

```go
if err == nil {
    r.index = 0
    r.scopeDepth = c.blockDepth   // for RANGE-3 cleanup tracking
    c.rangeStack = append(c.rangeStack, &r)
}
```

`Test_rangeInitByteCode_UnsupportedType` now asserts
`len(tc.ctx.rangeStack) == 0` after an unsupported type error.
