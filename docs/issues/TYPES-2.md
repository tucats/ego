# TYPES-2: `reflect.TypeOf(v).String()` panics when `v` is nil in `relaxedConformanceCheck`

| | |
| :-- | :-- |
| **Affected function** | `relaxedConformanceCheck` in `bytecode/types.go` |
| **Risk** | LOW — requires the RequiredType operand to be a string and the stack value to be nil simultaneously; uncommon in practice |
| **Discovering test** | `Test_relaxedConformanceCheck_NilValue_StringOperand_TYPES2` |
| **Status** | RESOLVED |

## TYPES-2: Original behavior

In the string-operand branch of `relaxedConformanceCheck`:

```go
if t, ok := i.(string); ok {
    if t != reflect.TypeOf(v).String() {  // ← panics when v is nil
        err = c.runtimeError(errors.ErrArgumentType)
    }
}
```

`reflect.TypeOf(nil)` returns a nil `reflect.Type`.  Calling `.String()` on a
nil `reflect.Type` panics:

```text
panic: runtime error: invalid memory address or nil pointer dereference
```

This is triggered when `requiredTypeByteCode` is called with a string operand
(a Go type name such as `"int"`) and the top-of-stack value is nil.

## TYPES-2: Fix

A nil guard was added before the reflect call:

```go
if t, ok := i.(string); ok {
    if v == nil || t != reflect.TypeOf(v).String() {  // ← v == nil guard added
        err = c.runtimeError(errors.ErrArgumentType)
    }
}
```

`Test_relaxedConformanceCheck_NilValue_StringOperand_TYPES2` calls
`relaxedConformanceCheck` with a string operand and a nil value, confirming
that an error is returned rather than a panic.
