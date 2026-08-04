# MATH-9 — `notByteCode` multi-type case returns wrong result for zero values of non-`int` integer types

**Affected function:** `notByteCode`  
**File:** `bytecode/math.go`  
**Risk:** Medium — `!byte(0)`, `!int64(0)`, `!int32(0)`, `!int16(0)`, etc. all
return `false` instead of `true`  
**Discovered by:** `Test_notByteCode_Int64Zero_CurrentlyBroken_MATH9`,
`Test_notByteCode_ByteZero_CurrentlyBroken_MATH9`,
`Test_notByteCode_Int32Zero_CurrentlyBroken_MATH9`  
**Status: RESOLVED**

## MATH-9: Description

`notByteCode` used a multi-type case to handle all integer types at once:

```go
case byte, int8, int32, int16, uint32, uint16, uint, uint64, int, int64:
    return c.push(value == 0)
```

When multiple types appear in a single `case`, Go types the case variable (`value`)
as `any` (interface{}).  The comparison `value == 0` compiles as an interface
comparison where the untyped constant `0` takes its default type `int`.

In Go, two interface values are equal only when both their **dynamic type** and
**dynamic value** match.  So:

| Stack value | `value == 0` evaluated as | Result |
| :---------- | :------------------------ | :----- |
| `int(0)` | `int(0) == int(0)` | `true` ✓ |
| `int64(0)` | `int64(0) == int(0)` | `false` ✗ |
| `byte(0)` | `byte(0) == int(0)` | `false` ✗ |
| `int32(0)` | `int32(0) == int(0)` | `false` ✗ |

Only `int(0)` gave the correct answer.

## MATH-9: Fix

Replaced the single multi-type case with ten individual single-type cases.  In a
single-type case, the switch variable takes the matched concrete type, so
`value == 0` uses the correctly-typed zero literal for each width:

```go
// MATH-9 fix: splitting into individual cases makes 'value' typed,
// so value == 0 uses the correctly-typed zero literal.

case byte:
    return c.push(value == 0)   // value is byte; 0 → byte(0)

case int8:
    return c.push(value == 0)   // value is int8; 0 → int8(0)

case int16:
    return c.push(value == 0)
// ... (int32, uint16, uint32, int, uint, int64, uint64 follow the same pattern)
```

The three `_CurrentlyBroken_MATH9` tests were renamed to drop the suffix and
updated to assert `true` for zero values of each affected type.
