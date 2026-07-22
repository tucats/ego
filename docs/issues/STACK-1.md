# STACK-1 — `copyByteCode` pushes the integer literal `2` instead of the deep copy

**Affected function:** `copyByteCode`  
**File:** `bytecode/stack.go`  
**Risk:** High — every call to the Copy opcode produces a wrong stack layout:
the second stack slot holds the integer `2` rather than a deep copy of the
original value; any code that reads the copy gets `2` instead of the expected
duplicate  
**Discovered by:** `Test_copyByteCode_PushesIntegerTwo_CurrentlyBroken_STACK1`  
**Status: RESOLVED**

## STACK-1: Description

`copyByteCode` correctly marshalled the original value into `v2` via a JSON
round-trip but then pushed the integer literal `2` instead of `v2`.

## STACK-1: Fix

Changed `c.push(2)` to `c.push(v2)`:

```go
byt, _ := json.Marshal(v)
err = json.Unmarshal(byt, &v2)
_ = c.push(v2)           // was: c.push(2)
```

A function-level comment was added noting that `json.Unmarshal` produces
`float64` for all numeric values, so the copy of an integer has a different
Go type than the original.

Test renamed from `_PushesIntegerTwo_CurrentlyBroken_STACK1` to
`_PushesDeepCopy_STACK1`; now asserts `TOS == float64(99)` after copying
`int(99)`.
