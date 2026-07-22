# MEMBERS-3 — `getStructMemberValue` returns raw errors without `c.runtimeError` decoration

**Affected function:** `getStructMemberValue`  
**File:** `bytecode/member.go`  
**Risk:** Low — struct member-access errors (`ErrUnknownMember`,
`ErrSymbolNotExported`) lack module/line info  
**Discovered by:** `Test_memberByteCode_Struct_FieldNotFound_Error`,
`Test_memberByteCode_Struct_UnexportedField_OtherPackage_Error`  
**Status: RESOLVED**

## MEMBERS-3: Description

Both error returns in `getStructMemberValue` were bare, so struct member-access
errors lacked source-location context.

## MEMBERS-3: Fix

Both returns now use `c.runtimeError()`:

```go
// MEMBERS-3 fix:
return nil, c.runtimeError(errors.ErrUnknownMember).Context(name)
return nil, c.runtimeError(errors.ErrSymbolNotExported).Context(name)
```
