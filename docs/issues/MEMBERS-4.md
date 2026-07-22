# MEMBERS-4 — `memberByteCode` doc comment says "second a map" but the function handles many types

**Affected function:** `memberByteCode`  
**File:** `bytecode/member.go`  
**Risk:** None — documentation only; the implementation is correct  
**Discovered by:** code review during `member_test.go` comprehensive audit  
**Status: RESOLVED**

## MEMBERS-4: Description

The original doc comment read:

```go
// memberByteCode instruction processor. This pops two values from
// the stack (the first must be a string and the second a
// map) and indexes into the map to get the matching value
// and puts back on the stack.
```

"The second a map" is incorrect.  The function actually dispatches over
`*data.Struct`, `*data.Package`, `*data.Map` (only with extensions), `*any`,
`*data.Type`, and native Go types.  The name also does not have to come from the
stack — it can come from the instruction operand.

## MEMBERS-4: Fix

The comment was rewritten to accurately describe both name-source paths and all
supported object types.  See the updated `memberByteCode` doc comment in
`bytecode/member.go`.
