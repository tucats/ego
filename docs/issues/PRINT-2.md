# PRINT-2: `case *data.Function:` in `formatValueForPrinting` is unreachable dead code

| | |
| :-- | :-- |
| **Affected function** | `formatValueForPrinting` in `bytecode/print.go` |
| **Risk** | LOW — current behavior (functions printing their declaration string) is not harmful, but the suppression intent is never fulfilled |
| **Discovering test** | `Test_formatValueForPrinting_FunctionPointer_ProducesEmptyString` and `Test_formatValueForPrinting_FunctionValue_SuppressedAfterFix_PRINT2` |
| **Status** | RESOLVED |

## PRINT-2: Original behavior

`formatValueForPrinting` has an empty case for `*data.Function`:

```go
case *data.Function:
    // intentionally empty — s stays ""
```

The intent is to suppress function values so that `print myFunc` produces no
output.  However, Ego stores function values on the runtime stack as
`data.Function` (**value** type, not pointer).  The case only matches
`*data.Function` (**pointer** type).

Since nothing in the normal execution path ever pushes a `*data.Function`
pointer onto the stack, this case is dead code.  When a function value IS
passed to `printByteCode`, it is a `data.Function` value, which falls through
to the `default` branch and is formatted by `data.FormatUnquoted` as its
declaration string (e.g., `"myFunc()"`).

## PRINT-2: Fix

The case was broadened to cover both the value and pointer forms:

```go
case data.Function, *data.Function:
    // Function values — both value and pointer — are suppressed.
    // s stays as "" so nothing is written to the output.
```

`Test_formatValueForPrinting_FunctionValue_SuppressedAfterFix_PRINT2` confirms
that a `data.Function` value now produces empty output, and the existing
`Test_formatValueForPrinting_FunctionPointer_ProducesEmptyString` continues to
confirm the same for `*data.Function` pointers.
