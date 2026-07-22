# RANGE-2 — `rangeNextByteCode` default case does not pop the range stack

**Affected function:** `rangeNextByteCode`  
**File:** `bytecode/range.go`  
**Risk:** Low — only reachable with a value type that bypassed `rangeInitByteCode`'s
type guard; in practice this path is not reached by well-formed Ego programs  
**Discovered by:** `Test_rangeNextByteCode_DefaultCase_LeavesStaleStackEntry_RANGE2`  
**Status: RESOLVED**

## RANGE-2: Description

The `default` case in `rangeNextByteCode` set `c.programCounter = destination`
but did not trim `c.rangeStack`, leaving a stale entry that could corrupt an
outer loop's state on its next `RangeNext` call.

## RANGE-2: Fix

Added the stack trim to the default case, matching every other exhaustion path:

```go
default:
    c.programCounter = destination
    c.rangeStack = c.rangeStack[:stackSize-1]  // added
```

Test renamed from `_LeavesStaleStackEntry_RANGE2` to `_PopsRangeStack_RANGE2`;
now asserts `len(tc.ctx.rangeStack) == 0`.
