# BUILTIN-TYPES-1 — `typeOf` returned a string for builtin functions, not a `*data.Type`

**Affected function:** `typeOf`
**File:** `builtins/types.go`
**Risk:** Low
**Status: RESOLVED**

## Original TYPES-1 behavior

Every other `typeOf` case returned a `*data.Type`.  The builtin-function case
returned the string `"<builtin>"`, making `typeof(someBuiltinFunc)` the only
path that violated the uniform return type.

## Fix for TYPES-1

The case now constructs a minimal `data.Function` descriptor with a
`"<builtin>"` name and returns a proper `*data.Type` of `FunctionKind`:

```go
builtinFn := data.Function{
    Declaration: &data.Declaration{Name: "<builtin>"},
}
return data.FunctionType(&builtinFn), nil
```

**Tests:** `Test_TypeOf_BuiltinFunctionReturnsFuncType`
