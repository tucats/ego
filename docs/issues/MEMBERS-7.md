# MEMBERS-7 — `getMemberValue` silently returns `(nil, nil)` for a nil `*data.Type` behind `*any`

**Affected function:** `getMemberValue`  
**File:** `bytecode/member.go`  
**Risk:** Low — the nil `*data.Type` behind a `*any` case is an edge case
unlikely to appear in well-formed Ego code, but it causes a silent nil push  
**Discovered by:** `Test_memberByteCode_PtrAny_PointingToNilType_MEMBERS7`  
**Status: RESOLVED**

## MEMBERS-7: Description

When the value behind a `*any` was a nil `*data.Type`, `BaseType()` returned nil,
the `bv != nil` guard failed, and no return statement executed.  Control fell
through to the end of `getMemberValue`, which returned `(nil, nil)`.
`memberByteCode` then pushed nil onto the stack with no error.

## MEMBERS-7: Fix

An explicit error return was added for the nil-BaseType path:

```go
case *data.Type:
    if bv := actual.BaseType(); bv != nil {
        return getMemberValue(c, bv, name)
    }
    // MEMBERS-7 fix: return an error instead of falling through with (nil, nil).
    return nil, c.runtimeError(errors.ErrInvalidType).Context("nil type")
```

`Test_memberByteCode_PtrAny_PointingToNilType` (renamed from the `_MEMBERS7`
form) now asserts `ErrInvalidType`.
