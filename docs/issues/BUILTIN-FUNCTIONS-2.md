# BUILTIN-FUNCTIONS-2 — `make()` declaration incorrectly declared return type as `int`

**Affected registration:** `"make"` entry in `FunctionDictionary`
**File:** `builtins/functions.go`
**Risk:** Medium
**Status: RESOLVED**

## Original FUNCTIONS-2 behavior

The `Returns` field for `"make"` was `[]*data.Type{data.IntType}`.  `Make`
actually returns a `[]any`, a `*data.Array`, or a `*data.Channel` — never an
`int`.  This mismatch exposed incorrect metadata to the compiler's type checker.

## Fix for FUNCTIONS-2

The `Returns` declaration was changed to `data.InterfaceType`, reflecting the
polymorphic return:

```go
Returns: []*data.Type{data.InterfaceType},
```
