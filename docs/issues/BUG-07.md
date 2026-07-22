# BUG-07 — Two-value channel receive `v, ok := <-ch` not supported

**Severity:** MEDIUM

**Description:**  
Go's standard idiom for detecting a closed channel is the two-value receive
form: `v, ok := <-ch`. In Ego, this produces `"incorrect number of return values"`.
Only the single-value form `v := <-ch` is supported.

**Reproducer:**

```go
import "fmt"

func main() {
    ch := make(chan, 2)
    ch <- 10
    close(ch)

    v, ok := <-ch   // ERROR: incorrect number of return values
    fmt.Println(v, ok)
}
```

**Actual output:**

```text
Error: at main(line 8), incorrect number of return values
```

**Expected output:**

```text
10 true
```

**Notes:**  
A second receive from the closed-and-drained channel should give `(<nil>, false)`.
Without this form, there is no reliable way in Ego to detect when a channel has
been closed.

**Resolution (June 2026):**

**Root cause:** A multi-target assignment such as `v, ok := <-ch` compiles its
left-hand side via `assignmentTargetList`, which always begins the generated
store bytecode with `StackCheck 2` — a check that exactly two values are present
above a stack marker before unpacking them to `v` and `ok`. For an ordinary
multi-return call (`a, b := f()`) the callee pushes both values itself. For a
channel receive, however, the only thing on the stack before this point was the
single channel object pushed by evaluating `<-ch`'s operand — one value, not two
— so `StackCheck 2` always failed with `"incorrect number of return values"`.
The single-value form `v := <-ch` was unaffected because it goes through a
different code path (`StoreChan`) that doesn't use `StackCheck`.

**Design:** A new `ReceiveChannel` bytecode instruction performs the actual
channel receive and leaves the stack in the shape `StackCheck 2` expects:

```text
[0]  StackMarker("receive")
[1]  ok bool      — true if a value was received, false if the channel
                     is closed and drained
[2]  datum any    — the received value, or nil when ok is false
```

`compileAssignment` detects the specific combination of a channel-receive RHS
(`<-` was consumed) together with a multi-target LHS (`c.flags.multipleTargets`)
and emits `ReceiveChannel` instead of leaving the plain `Load "ch"` result on the
stack. The existing `storeLValue` bytecode (already built by
`assignmentTargetList` for any multi-target assignment) is unchanged and unpacks
the two pushed values to `v` and `ok` exactly as it would for a function's
multi-value return.

**Fixes applied:**

`bytecode/opcodes.go` — added the `ReceiveChannel` opcode: a new constant, an
entry in `opcodeNames`, and a dispatch-table registration pointing at
`receiveChannelByteCode`.

`bytecode/store.go` — added `receiveChannelByteCode`: pops the channel value
(erroring with `ErrInvalidChannel` if the popped value isn't a `*data.Channel`),
calls `Receive()`, and pushes `[StackMarker("receive"), ok, datum]`. A closed,
drained channel (`Receive()` returning `ErrChannelNotOpen`) is translated to
`ok = false, datum = nil` rather than propagated as an error, matching Go's
"second value is the success flag" convention.

`compiler/assignment.go` — `compileAssignment`: captures whether the RHS began
with `<-` (`isChannelReceive`) before parsing the expression. When
`isChannelReceive && c.flags.multipleTargets` is true, emits `ReceiveChannel`
in place of the plain expression result and appends `storeLValue` as usual. The
single-value channel-receive path (`v := <-ch`) is untouched since
`multipleTargets` is false in that case.

**Tests added:**

`bytecode/store_test.go` — five new Go unit tests (Section 7):
`Test_receiveChannelByteCode_SuccessfulReceive`,
`Test_receiveChannelByteCode_ClosedChannel`,
`Test_receiveChannelByteCode_NonChannelValue`,
`Test_receiveChannelByteCode_StackMarker`, and
`Test_receiveChannelByteCode_StackLayout` — covering the success case, the
closed-channel case, error handling for non-channel and stack-marker inputs,
and the exact push order/count on the stack.

`compiler/assignment_test.go` — one new compile-time case verifying
`v, ok := <-ch` compiles without error.

`compiler/run_test.go` — `TestBUG07TwoValueChannelReceive`, a standalone Go
test (separate from `TestArbitraryCodeFragments`, which lacks builtins like
`make`) that injects a pre-built `*data.Channel` directly into the symbol table
and exercises three scenarios: a value is waiting (`ok == true`), the received
value itself is correct, and a closed channel yields `ok == false`. Because this
test compiles a fragment using a channel that was never declared with `make()`
in the Ego source, it pins `defs.UnknownVarSetting` to `false` for its duration
so its outcome doesn't depend on a developer's persisted `~/.ego/` profile
settings or on what other tests in the package happen to run first.

`tests/flow/two_value_receive.ego` — five Ego language tests: receiving a
buffered value (`ok == true`, correct value), draining multiple buffered values
in order, detecting a closed channel (`ok == false`, `v == nil`), draining all
buffered items before reporting closure, and receiving correctly when the
sender is a separate goroutine.

