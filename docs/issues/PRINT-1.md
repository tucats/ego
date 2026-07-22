# PRINT-1: Unchecked type assertion in `formatValueForPrinting` panics on nil struct-array elements

| | |
| :-- | :-- |
| **Affected function** | `formatValueForPrinting` in `bytecode/print.go` |
| **Risk** | MEDIUM — any Ego program that prints a struct array before all elements are assigned |
| **Discovering test** | `Test_formatValueForPrinting_ArrayOfStructs` (tests the safe path only; nil path would panic the test binary) |
| **Status** | RESOLVED |

## PRINT-1: Original behavior

`formatValueForPrinting` handles `*data.Array` values whose element type is a
struct kind by iterating over the elements and casting each one to `*data.Struct`:

```go
for i := 0; i < actualValue.Len(); i++ {
    rowValue, _ := actualValue.Get(i)
    row := rowValue.(*data.Struct)   // ← unchecked assertion
    ...
}
```

`data.Array.Get(i)` returns `a.data[i]` directly.  When an array is created
with `data.NewArray(structType, n)`, the `data` slice is allocated but struct
elements are **not** initialized — only scalar kinds (bool, int, float, string)
get zero values.  Struct elements remain `nil`.

Calling `nil.(*data.Struct)` panics at runtime:

```text
panic: interface conversion: interface is nil, not *data.Struct
```

## PRINT-1: Conditions that trigger the panic

1. Ego code creates a struct array with a pre-allocated size but does not
   assign all elements before printing — e.g., `arr := make([]MyStruct, 3)`
   followed by partial assignment.
2. Test code uses `data.NewArray(structType, n)` without populating every slot.

Note: `data.Array.Make` (used by the Ego `make()` built-in) calls
`InstanceOfType` which correctly initializes struct elements with
`data.NewStruct(t)`.  Only direct calls to `data.NewArray` leave slots nil.

## PRINT-1: Fix

A nil guard and a type-ok check were added before the assertion.  Nil elements
and any non-struct elements are silently skipped, so a partially-initialized
array simply omits those rows from the output instead of panicking:

```go
rowValue, _ := actualValue.Get(i)
if rowValue == nil {
    continue
}
row, ok := rowValue.(*data.Struct)
if !ok {
    continue
}
```

`Test_formatValueForPrinting_ArrayOfStructs_NilElement_PRINT1` creates a
three-element struct array with only slots 0 and 2 populated, calls
`formatValueForPrinting`, and confirms that the output contains the two valid
rows without panicking.
