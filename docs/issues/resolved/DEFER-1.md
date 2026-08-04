# DEFER-1 — `deferByteCode` receiver slice captures wrong elements when new count ≠ deferThisSize

**Affected function:** `deferByteCode`  
**File:** `bytecode/defer.go`  
**Risk:** Medium — deferred method calls in deeply nested receiver contexts may
pass incorrect receiver values, causing the deferred function to operate on the
wrong object  
**Discovered by:** `Test_deferByteCode_ReceiverCapture_MultipleNewReceivers_DEFER1`  
**Status: RESOLVED**

## DEFER-1: Description

When `deferByteCode` detects that new receivers were pushed to the receiver stack
during evaluation of the deferred call's target (i.e. `c.deferThisSize > 0 &&
c.deferThisSize < len(c.receiverStack)`), it must:

1. **Capture** the newly added receivers into the `deferStatement`'s
   `receiverStack` slice so they are replayed when the defer executes.
2. **Trim** the live receiver stack back to its pre-defer length (`c.deferThisSize`)
   so the caller's receiver state is restored.

The trim is correct:

```go
c.receiverStack = c.receiverStack[:c.deferThisSize]
```

But the capture used the wrong formula:

```go
// BUGGY — captures the last deferThisSize elements, not the newly added ones:
receivers = c.receiverStack[len(c.receiverStack)-c.deferThisSize:]
```

**Why this is wrong:**  The newly added receivers start at index `deferThisSize`
(the pre-defer stack size).  The correct slice is:

```go
receivers = c.receiverStack[c.deferThisSize:]
```

The buggy formula, `receiverStack[len-deferThisSize:]`, only coincidentally
equals the correct formula when exactly `deferThisSize` new receivers were added
(i.e. `len == 2 * deferThisSize`).  Any other count produces a wrong slice:

| deferThisSize | New receivers | len | Buggy start | Correct start | Effect |
| :---: | :---: | :---: | :---: | :---: | :--- |
| 1 | 1 | 2 | 1 | 1 | ✓ coincidentally correct |
| 1 | 2 | 3 | 2 | 1 | ✗ misses first new receiver |
| 2 | 1 | 3 | 1 | 2 | ✗ includes one pre-existing receiver |
| 2 | 3 | 5 | 3 | 2 | ✗ misses first new receiver |

## DEFER-1: Fix

Changed the capture line from:

```go
receivers = c.receiverStack[len(c.receiverStack)-c.deferThisSize:]
```

to the correct slice that starts exactly at the pre-defer boundary:

```go
receivers = c.receiverStack[c.deferThisSize:]
```

`Test_deferByteCode_ReceiverCapture_MultipleNewReceivers_DEFER1` now passes:
with `deferThisSize=1` and two new receivers pushed, both `[R1, R2]` are
captured rather than only `[R2]`.
