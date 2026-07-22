# BUILTIN-DELETE-1 — Hard-coded argument position in error context

**Affected function:** `Delete`
**File:** `builtins/delete.go`
**Risk:** Low
**Status: RESOLVED**

## Original DELETE-1 behavior

The `default` error case used the magic integer literal `1` to indicate the
first argument.  While technically correct, the literal had no name to
communicate its intent.

## Fix for DELETE-1

A local named constant `firstArgument = 1` was introduced at the call site,
making the intent explicit without changing the error output:

```go
const firstArgument = 1
return nil, errors.ErrInvalidType.In("delete").Context(
    fmt.Sprintf("argument %d: %s", firstArgument, data.TypeOf(v).String()))
```
