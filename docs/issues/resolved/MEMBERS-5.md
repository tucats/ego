# MEMBERS-5 — `getPackageMemberValue` signature includes dead parameters `v any` and `found bool`

**Affected function:** `getPackageMemberValue`  
**File:** `bytecode/member.go`  
**Risk:** None — no correctness impact; the parameters are always zero-valued
when passed and are immediately overwritten inside the function  
**Discovered by:** code review during `member_test.go` comprehensive audit  
**Status: RESOLVED**

## MEMBERS-5: Description

`getPackageMemberValue` had two dead parameters — `v any` and `found bool` — that
were always passed as zero values and immediately overwritten inside the function.

## MEMBERS-5: Fix

The dead parameters were removed.  New signature:

```go
// MEMBERS-5 fix: removed dead parameters v any and found bool.
func getPackageMemberValue(name string, mv *data.Package, c *Context) (any, error)
```

The single call site in `getMemberValue` was updated accordingly:

```go
case *data.Package:
    return getPackageMemberValue(name, mv, c)
```

The function now declares its own local `v` and `found` variables.
