# MEMBERS-6 — `getMemberValue` ignores the member name when the object is `*data.Type`

**Affected function:** `getMemberValue`  
**File:** `bytecode/member.go`  
**Risk:** Medium — any named member access on a `*data.Type` value silently
returns the type's string representation instead of the requested member's value
or an error  
**Discovered by:** `Test_memberByteCode_TypeObject_ReturnsTypeName_MEMBERS6`,
`Test_memberByteCode_TypeObject_IntType_MEMBERS6`  
**Status: RESOLVED**

## MEMBERS-6: Description

The original `*data.Type` fast path in `getMemberValue` returned `t.String()` for
every member access, completely ignoring the `name` parameter:

```go
// Original (buggy):
if t, ok := m.(*data.Type); ok {
    v = t.String()
    return v, nil   // name was never consulted
}
```

This meant `someType.NonExistent` succeeded silently and pushed the type name
string onto the stack rather than returning an error.

## MEMBERS-6: Fix

The fast path now performs a proper member lookup via `t.Function(name)`.  If a
function is registered on the type under that name it is returned; otherwise
`ErrUnknownMember` is reported:

```go
// MEMBERS-6 fix:
if t, ok := m.(*data.Type); ok {
    if fn := t.Function(name); fn != nil {
        return data.UnwrapConstant(fn), nil
    }
    return nil, c.runtimeError(errors.ErrUnknownMember).Context(name)
}
```

The two `_MEMBERS6` tests were renamed and updated:

- `Test_memberByteCode_TypeObject_UnregisteredName_Error` — asserts `ErrUnknownMember`
- `Test_memberByteCode_TypeObject_RegisteredFunction_OK` — positive case showing a registered function IS returned
- `Test_memberByteCode_PtrAny_PointingToType_UnregisteredName` — the *any recursive path also now returns `ErrUnknownMember`
