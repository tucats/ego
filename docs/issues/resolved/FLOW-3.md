# FLOW-3 — Pre-helper tests in `flow_test.go` used raw `&Context{}` struct literals

**Affected tests:** `Test_stopByteCode`, `Test_panicByteCode`, `Test_typeCast`,
`Test_localCallAndReturnByteCode`, `Test_branchFalseByteCode`,
`Test_branchTrueByteCode`  
**File:** `bytecode/flow_test.go`  
**Risk:** Low — the tests passed but bypassed the initialization that
`NewContext` performs, which could mask future bugs in that path  
**Discovered by:** code review of `flow_test.go` during the flow-test audit  
**Status: RESOLVED**

## FLOW-3: Original behavior

Six tests constructed a `*Context` with a raw struct literal, skipping
`NewContext`:

```go
ctx := &Context{
    stack:          make([]any, 5),
    stackPointer:   0,
    symbols:        symbols.NewSymbolTable("cast test"),
    programCounter: 1,
    bc:             &ByteCode{instructions: make([]instruction, 5), nextAddress: 5},
}
ctx.running.Store(true)
```

The `newTestContext(t)` helper (from `testhelpers_test.go`, mandated by
`CLAUDE.md`) creates a properly initialized context via `NewContext` with a
two-level root→local symbol table.  The raw literal bypasses that, which can
mask bugs in `NewContext` or in code that relies on the full initialization.

## FLOW-3: Fix

All six tests were rewritten to use `newTestContext` and the "with" builder
chain.  Key conversion notes:

- **`Test_stopByteCode`**: Trivial — `stopByteCode` only uses `c.running`.
  The companion `Test_stopByteCode_WithNewContext` was removed (it became
  redundant once the original was converted).
- **`Test_panicByteCode`** → **`Test_panicByteCode_OperandMode`**: The original
  test pre-loaded a stack item that was never consumed (the operand was always
  non-nil).  The converted test uses an empty stack to make the intent clear.
- **`Test_typeCast`** → **`Test_typeCast_IntToString` + `Test_typeCast_BoolToString`**:
  Split into two flat tests; `newTestContext` supplies the symbol table and
  bytecode that `callByteCode` needs.
- **`Test_localCallAndReturnByteCode`**: Uses `withBytecodeSize(5)` for the
  `localCallByteCode(ctx, 5)` call.  The saved-table-name assertion was updated
  from `"local call test"` (the old manual name) to `"test local"` (the name
  that `newTestContext` assigns to its local symbol table).
- **`Test_branchFalseByteCode`** and **`Test_branchTrueByteCode`**: Both use
  `withBytecodeSize(5)` and a shared `tc` across the three sub-cases so that
  the program-counter carry-over from sub-case 1 to sub-case 2 is preserved.
