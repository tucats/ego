# LOAD-2 — `explodeByteCode` doc comment incorrectly described the operand as "a struct"

**Affected function:** `explodeByteCode`  
**File:** `bytecode/load.go`  
**Risk:** None — documentation only; behavior was correct  
**Discovered by:** `Test_explodeByteCode_NonMapStruct` in `bytecode/load_test.go`  
**Status: RESOLVED**

## LOAD-2: Original behavior

The function-level comment on `explodeByteCode` read:

```go
// explodeByteCode implements Explode. This accepts a struct on the top of
// the stack, and creates local variables for each of the members of the
// struct by their name.
```

The word "struct" is incorrect.  The implementation unconditionally asserts
the popped value as `*data.Map`:

```go
if m, ok := v.(*data.Map); ok {
```

A `*data.Struct` on the stack fails this assertion and falls through to the
`else` branch, returning `ErrInvalidType`.  The original comment would lead
a reader to believe that passing a struct to the Explode opcode was valid.

`Test_explodeByteCode_NonMapStruct` was added as a regression anchor: it
confirms that a `*data.Struct` is rejected with `ErrInvalidType` and documents
the gap between the comment and the implementation.

## LOAD-2: Fix

The comment was rewritten to describe the actual behavior accurately:

```go
// explodeByteCode implements Explode. This accepts a *data.Map on the top of
// the stack, and creates local variables for each of the key-value pairs in
// the map.  The map must have string keys; non-string keys are rejected with
// ErrWrongMapKeyType.  After creating the variables, a bool is pushed
// indicating whether the map was empty (true = empty, false = had entries).
```
