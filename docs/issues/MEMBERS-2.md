# MEMBERS-2 — `getMemberValue` returns raw `ErrFunctionReturnedVoid` when stack marker detected

**Affected function:** `getMemberValue`  
**File:** `bytecode/member.go`  
**Risk:** Low — the error that reaches user code lacks source-location information  
**Discovered by:** `Test_memberByteCode_StackMarkerAsObject`  
**Status: RESOLVED**

## MEMBERS-2: Description

When `getMemberValue` detected a `StackMarker` as the object, it returned the
error bare — inconsistent with all other error paths in the file.

## MEMBERS-2: Fix

```go
// MEMBERS-2 fix: was returning errors.ErrFunctionReturnedVoid bare.
if isStackMarker(m) {
    return nil, c.runtimeError(errors.ErrFunctionReturnedVoid)
}
```
