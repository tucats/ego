# BUILTIN-CAST-2 — `Cast` returned `ErrInvalidType` for a valid nil coercion result

**Affected function:** `Cast`
**File:** `builtins/cast.go`
**Risk:** Low
**Status: RESOLVED**

## Original CAST-2 behavior

After calling `data.Coerce`, the code tested `if v != nil` and returned
`ErrInvalidType` when the coercion succeeded but produced `nil`.  This was
incorrect: `data.Coerce` returning `(nil, nil)` is a valid success for certain
target types.

## Fix for CAST-2

The `if v != nil` guard was removed.  When `err == nil`, the coercion
succeeded; the result (including nil) is returned directly:

```go
v, err := data.Coerce(source, data.InstanceOfType(t))
if err != nil {
    return nil, errors.New(err).In(t.String())
}
return v, nil
```
