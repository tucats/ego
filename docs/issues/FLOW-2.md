# FLOW-2 — `moduleByteCode` and `atLineByteCode` access `array[1]` without a bounds check

**Affected functions:** `moduleByteCode`, `atLineByteCode`  
**File:** `bytecode/flow.go`  
**Risk:** Low — a one-element array operand panics with "index out of range";
the compiler always emits two-element arrays for these opcodes  
**Discovered by:** code review of `flow.go` during the flow-test audit  
**Status: RESOLVED**

## FLOW-2: Original behavior

Both functions accepted a `[]any` array operand but accessed `array[1]`
unconditionally:

```go
// moduleByteCode (original):
if array, ok := i.([]any); ok {
    c.module = data.String(array[0])
    if t, ok := array[1].(*tokenizer.Tokenizer); ok {   // ← no len check → panic
        c.tokenizer = t
        t.Close()
    }
}

// atLineByteCode (original):
if array, ok := i.([]any); ok {
    if line, err = data.Int(array[0]); err != nil {
        return err
    }
    text = data.String(array[1])   // ← no len check → panic
}
```

If the compiler ever emits a one-element array for either opcode (e.g., when
the tokenizer is unavailable at compile time), `array[1]` panics at runtime
with "index out of range [1] with length 1".

## FLOW-2: Fix

A `len(array) > 1` guard was added before each `array[1]` access in both
functions, and the function-level comments were updated to document the
optional-slot contract:

```go
// moduleByteCode (fixed):
if array, ok := i.([]any); ok {
    c.module = data.String(array[0])
    // Only look for the tokenizer when a second element is actually present.
    if len(array) > 1 {
        if t, ok := array[1].(*tokenizer.Tokenizer); ok {
            c.tokenizer = t
            t.Close()
        }
    }
}

// atLineByteCode (fixed):
if array, ok := i.([]any); ok {
    if line, err = data.Int(array[0]); err != nil {
        return err
    }
    // Only read the source text when a second element is present.
    if len(array) > 1 {
        text = data.String(array[1])
    }
}
```

Regression tests confirm both functions handle a single-element array
without panicking and still set the primary field correctly:

- `Test_moduleByteCode_SingleElementArray` — module name set, no panic
- `Test_atLineByteCode_SingleElementArray` — line number set, source stays ""
