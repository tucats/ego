# FLOW-H1 — `for range` over an array iterates an immutable snapshot; mutation fails

**Affected files:**

- `bytecode/range.go` — `rangeInitByteCode`, `rangeNextArray`

**Description:**  
In Go, a `for i, v := range a` loop iterates over the array `a` directly and it is
perfectly legal to modify elements through the index during the loop body:

```go
a := []int{1, 2, 3}
for i := range a {
    a[i] *= 10     // modifies original; result: [10, 20, 30]
}
```

In Ego, `rangeInitByteCode` called `actual.SetReadonly(true)` on the original array
before iterating. Because the symbol `a` in the Ego symbol table pointed to the same
`*data.Array` object, any `a[i] = val` inside the loop body reached
`array.Set()` → `if a.immutable > 0 { return ErrImmutableArray }` → runtime error.

A secondary bug existed: if the loop was exited early via `break`,
`rangeNextArray`'s cleanup (`SetReadonly(false)`) was never called, leaving the
array permanently immutable for the rest of the function scope.

**Test file:** `tests/flow/for_range_advanced.ego` — test
`"flow: range over array allows element mutation"` verifies the fixed behavior, and
`"flow: range mutation and direct index loop produce same result"` confirms both loop
forms produce identical results.

**Resolution (May 2026):**  
Two lines removed from `bytecode/range.go`:

1. **`rangeInitByteCode` (line 89):** Removed `actual.SetReadonly(true)` from the
   `*data.Array` case. Ego arrays are fixed-size — there is no structural-modification
   hazard to guard against. Element writes inside the loop body now succeed, matching
   Go's slice range semantics.

2. **`rangeNextArray` (line 195):** Removed the corresponding `actual.SetReadonly(false)`
   call from the loop-exhaustion branch. Since the array is never marked immutable at
   range start, there is nothing to restore. This also eliminates the secondary bug
   where a `break` out of a range loop left the array permanently immutable.

The `immutable` counting semaphore on `data.Array` and the `SetReadonly` method are
unchanged; they continue to serve other legitimate read-only use cases (`_`-prefixed
variables, server runtime arrays, etc.).

