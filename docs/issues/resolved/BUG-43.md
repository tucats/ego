# BUG-43 — `defer receiver.Method(args)` eagerly captures its arguments but not the receiver

**Severity:** MEDIUM

**Description:**  
`docs/LANGUAGE.md`'s `defer` documentation (and CLAUDE.md's notes on the FLOW-M4/BUG-16
fix) establish that a deferred call's arguments are evaluated eagerly, at the point the
`defer` statement runs, not when the deferred call actually executes. For a receiver-
qualified deferred call (`defer receiver.Method(args)`), the *arguments* are correctly
captured eagerly, but the *receiver* expression is not — it is re-resolved from the live
symbol table when the deferred call finally executes, matching neither Go semantics nor
Ego's own documented eager-argument-evaluation guarantee.

**Reproducer:**

```go
type Logger struct {
    prefix string
}

func (l Logger) Log(msg string) {
    fmt.Println(l.prefix, msg)
}

func main() {
    l := Logger{prefix: "A"}
    defer l.Log("deferred")
    l.prefix = "B"
    fmt.Println("main prefix now", l.prefix)
}
```

**Actual output:**

```text
main prefix now B
B deferred
```

**Expected output** (verified against real Go 1.24, `go run`):

```text
main prefix now B
A deferred
```

**Notes:**  
A control test confirms the *argument* (`msg`) to the same deferred call **is** eagerly
captured correctly — mutating `msg` after the `defer` statement does not affect the printed
value. Only the receiver is wrong. Root cause: `internal/language/compiler/defer.go`'s
`hoistDeferCallArguments` (the fix for BUG-16/FLOW-M4) only hoists the tokens *inside the
parentheses* of the deferred call into eagerly-evaluated temp variables. The receiver
portion of a dotted call (`l` in `l.Log(...)`) is left inside the wrapped closure
(`func(){ l.Log($1) }()`) and is therefore resolved from the live symbol table when the
deferred closure actually executes. This is a distinct, narrower gap than FLOW-M4 (which is
specifically about bare-identifier calls like `defer namedFunc(arg)`), confirmed still open
even though FLOW-M4/BUG-16 itself is fixed as of commit `e7e70637`.

**Resolution (July 2026):**  
Two changes were needed — the second was discovered only while verifying the first against
the bug's own reproducer, and broadens the fix slightly beyond the literal report, since it
turned out to be the same missing piece wearing a different hat:

1. **New `hoistDeferReceiver()` in `internal/language/compiler/defer.go`**, modeled directly
   on the existing `hoistDeferCallArguments()` (the BUG-16/FLOW-M4 fix) and called
   immediately after it in `compileDefer()`. For a dotted deferred call, it locates the last
   `.` before the call's `(` — the boundary between the receiver chain (`l`, or a
   multi-level chain like `wg.mu`) and the final method name — compiles just that receiver
   expression right now, in the compiler's normal (non-deferred) bytecode stream, and
   rewrites the token stream from `receiverChain.MethodName(args)` to
   `$tempName.MethodName(args)`. The closure-wrapping step that follows in `compileDefer()`
   then wraps a call whose receiver is a frozen temp variable instead of the original,
   possibly-later-mutated expression. A bare `defer namedFunc(args)` call (no `.` at all) has
   no receiver to freeze and is left untouched — that remains FLOW-M4's own, distinct,
   still-open gap.

2. **New `ValueCopy` opcode** (`internal/language/bytecode/opcodes.go`,
   `valueCopyByteCode` in `store.go`). Verifying step 1 against the bug's own reproducer
   initially still printed `B deferred` instead of `A deferred` — tracing the compiled
   bytecode showed `Load "l"` followed by `StoreAlways "$2"` was correctly running at
   *defer*-statement time and correctly capturing `Logger{prefix: "A"}`, but `$2` was later
   observed to hold `prefix: "B"` anyway. The cause: unlike `Store` and `CreateAndStore`
   (which both call `copyStructForValueSemantics` — the BUG-26 fix — so that binding a
   struct value to a new name always makes an independent copy), `StoreAlways` intentionally
   never copies; it's used throughout the compiler for cases where aliasing a live value is
   fine or even required (e.g. the "closures stored during a loop" fix relies on it). Both
   `hoistDeferCallArguments` and `hoistDeferReceiver` need `StoreAlways`'s *repeatability* —
   the same compiled `defer` statement re-running inside a loop must not fail the way
   `CreateAndStore`'s `c.create(name)` would (`ErrSymbolExists` on the second iteration) —
   but they also need the struct-copy guarantee that only `Store`/`CreateAndStore` provide.
   `ValueCopy` is a small new opcode that does exactly `copyStructForValueSemantics` on the
   top-of-stack value in place (an independent copy for a `*data.Struct`, unchanged for
   everything else, including `*data.Array`/`*data.Map` — those stay aliased, matching Go's
   own slice/map semantics). Both hoisting functions now emit `ValueCopy` immediately before
   their `StoreAlways`, giving their temp variables the same value-copy semantics as an
   ordinary `tempName := value` short variable declaration, without losing repeatability.

   This second change is not purely internal to the receiver fix: `hoistDeferCallArguments`
   had the identical latent defect for a **struct-valued argument** (not just a receiver) —
   confirmed separately with `defer show(structArg)` followed by mutating `structArg`, which
   printed the mutated value instead of the frozen one before this fix. `ValueCopy` fixes
   both call sites in one pass.

**Tests added:**

- `internal/language/bytecode/store_test.go` (Section 8, "valueCopyByteCode"):
  `Test_valueCopyByteCode_StructIsIndependentCopy`,
  `Test_valueCopyByteCode_NonStructIsUnchanged` (scalars pass through unchanged),
  `Test_valueCopyByteCode_ArrayIsAliased` (confirms arrays/slices are deliberately NOT
  copied), and `Test_valueCopyByteCode_EmptyStack`.
- `internal/language/compiler/defer_test.go`: `TestCompiler_compileDefer_ReceiverHoistedEagerly`
  (inspects the exact emitted bytecode shape for `defer l.Log(42)`, including both `ValueCopy`
  instructions and that the closure body never re-loads `l` directly),
  `TestCompiler_hoistDeferReceiver_BareFunctionCallIsNoOp`, and
  `TestCompiler_hoistDeferReceiver_MultiLevelChain` (`defer wg.mu.Lock()`). The pre-existing
  `TestCompiler_compileDefer_ArgumentsHoistedEagerly` was updated for the extra `ValueCopy`
  instruction now present in the argument-hoisting sequence.
- `tests/defer/method_receivers.ego`: `"defer: BUG-43, receiver is frozen at defer-statement
  time"` (the exact reproducer above), `"defer: BUG-43, no-argument deferred method call also
  freezes receiver"`, `"defer: BUG-43, multi-level receiver chain is frozen as a whole"`,
  `"defer: BUG-43, struct-valued argument is also frozen (not just the receiver)"`, `"defer:
  BUG-43, slice argument remains aliased like a real Go slice"` (guards against
  over-copying), and `"defer: BUG-43, repeated defer inside a loop does not error"` (guards
  the `StoreAlways`-repeatability requirement that ruled out using `CreateAndStore`).

