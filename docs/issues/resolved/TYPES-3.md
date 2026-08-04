# TYPES-3: Int-dispatch switch in `relaxedConformanceCheck` has unreachable cases

| | |
| :-- | :-- |
| **Affected function** | `relaxedConformanceCheck` in `bytecode/types.go` |
| **Risk** | LOW — current behavior is incorrect for non-`int` integer types (e.g., int16) but these cases are never reached, so no observable error is produced |
| **Discovering test** | `Test_requiredTypeByteCode_Int16Operand_MatchingValue_TYPES3` and `Test_requiredTypeByteCode_Int16Operand_WrongValue_TYPES3` |
| **Status** | RESOLVED |

## TYPES-3: Original behavior

The int-operand branch extracts an `int` from the operand and then switches on
the Ego kind of that value:

```go
if t, ok := i.(int); ok {
    switch data.TypeOf(t).Kind() {
    case data.Int16Kind:
        _, ok = v.(int16)
    case data.UInt16Kind:
        _, ok = v.(uint16)
    ...
    case data.IntKind:
        _, ok = v.(int)
    ...
    }
}
```

`t` is the result of `i.(int)` — a plain Go `int`.  `data.TypeOf(int(anything))`
always returns `data.IntType`, whose `.Kind()` is always `data.IntKind`.
Therefore:

- The `case data.IntKind` branch is the only one ever taken.
- All other cases (`Int16Kind`, `UInt16Kind`, `Int8Kind`, `Int32Kind`,
  `Int64Kind`, `Float32Kind`, `Float64Kind`, `ByteKind`, `BoolKind`,
  `StringKind`) are dead code and can never execute.

The practical consequence is that when `requiredTypeByteCode` receives an int
operand and a non-`int` value (e.g., `int16(5)`), the check `v.(int)` fails
and `ErrArgumentType` is returned even if int16 might be a valid match for the
intended type.

## TYPES-3: Fix

The `i.(int)` extraction was removed and replaced with a `switch i.(type)` that
dispatches on the exact Go type of the operand:

```go
switch i.(type) {
case int:
    _, kindOk = v.(int)
case int16:
    _, kindOk = v.(int16)
case uint16:
    _, kindOk = v.(uint16)
// … int8, int32, int64, byte, bool, float32, float64 …
default:
    kindOk = true   // non-integer operands pass through unchanged
}
```

Non-integer operands (such as `*data.Type` values handled by the block above)
hit the `default` branch, preserving the original pass-through behavior for
those cases.

`Test_requiredTypeByteCode_Int16Operand_MatchingValue_TYPES3` confirms that an
`int16(5)` value now passes when `int16(0)` is the operand, and
`Test_requiredTypeByteCode_Int16Operand_WrongValue_TYPES3` confirms that a
plain `int` value fails for the same operand.
