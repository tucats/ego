# OPTIMIZER-7 — `executeFragment` creates full interpreter context for trivial constant folding

**Affected function:** `executeFragment`  
**File:** `bytecode/optimizer.go`  
**Risk:** Performance — each constant-fold optimization allocates a ByteCode,
SymbolTable, and Context object and runs the full interpreter for 2–3
instructions  
**Status: RESOLVED**

## OPTIMIZER-7: Description

Every constant-fold optimization (Add, Sub, Mul) previously called
`executeFragment` which builds a full interpreter context even for trivially
simple arithmetic.

## OPTIMIZER-7: Fix

`tryConstantArithmetic(op Opcode, v1, v2 any) (any, bool)` was added.  It
handles `int`, `int64`, `float64`, and string concatenation directly with
type assertions and native Go arithmetic — no allocations beyond the return
value.  The `optRunConstantFragment` handler in the replacement loop tries
this fast path first:

```go
if result, ok := tryConstantArithmetic(arithOp, v1, v2); ok {
    newInstruction.Operand = result
} else {
    v, err := b.executeFragment(idx, idx+patLen)
    ...
}
```

`executeFragment` is retained as the fallback for type-alias operands and any
non-numeric types that reach the constant-fold rules.
