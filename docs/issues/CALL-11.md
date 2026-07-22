# CALL-11 — Receiver-stack corruption when a package-function call is nested inside a receiver method call's arguments

**Affected functions:** `callByteCode`, `callNative`, `callRuntimeFunction`, `callBytecodeFunction`, `getThisByteCode`, `emitDeferredCall` (new), `formatUsingString`  
**Files:** `internal/language/bytecode/call.go`, `callNative.go`, `callRuntimeFunction.go`, `callBytecodeFunction.go`, `this.go`, `defer.go`, `context.go`, `internal/language/compiler/expr_reference.go`, `expr_function.go`, `compiler.go`, `internal/runtime/fmt/print.go`  
**Risk:** High when triggered — silently corrupts the receiver of an enclosing method call, producing a misleading "no function receiver" error (in principle it could instead dispatch to the wrong receiver's method rather than erroring, since it consumes whatever entry happens to be on top of the stack)  
**Discovered by:** manual testing while writing `rest` package documentation examples (`conn.Base("https://" + os.Hostname() + ...)`); re-validated and completed in a later session, which also found two further variants of the same bug family (see below)  
**Status: RESOLVED**

## CALL-11: Revalidation

Before implementing a fix, the previously-documented "still open" case was
re-confirmed still reproducible exactly as described (`f.WriteString("x" +
io.DirList("/tmp"))` → `no function receiver: WriteString`). Investigating it
turned up two further variants of the same underlying bug family that were
not previously documented:

- **A bare call could steal an enclosing receiver.** `callNative.go`'s
  no-receiver branch discarded a stale `receiverStack` entry
  *unconditionally* (the partial fix noted below), which fixed the
  nested-dot-call case but, symmetrically, meant a **bare** call — no dot
  syntax at all, e.g. `toa := strconv.Itoa; ...toa(42)...` nested inside a
  receiver call's arguments — would wrongly pop and discard the *enclosing*
  call's genuine receiver, since nothing distinguished "this call had its own
  `SetThis`" from "some other call's entry happens to be on top right now".
- **`fmt.Sprint`/`String()` formatting bypassed the whole mechanism.**
  `formatUsingString` (`internal/runtime/fmt/print.go`) invokes a type's
  `String()` method by directly constructing a `*bytecode.Context` and
  calling `Run()` on it — entirely bypassing `callByteCode`/
  `callBytecodeFunction`. It seeded the receiver via the old `PushThis` +
  `GetThis`-pops-`receiverStack` mechanism, so any fix that moved receiver
  consumption out of `GetThis` had to give this caller an equivalent way to
  stage a receiver (see `SetPendingReceiver` below); missing this initially
  broke `fmt.Sprint(x)` for any type with a custom `String()` method with
  "unknown identifier" errors inside `String()` itself.

## CALL-11: Description

The compiler emits a `SetThis` instruction for any `X.Y(...)` call syntax
(`compileDotReference` in `compiler/expr_reference.go`), pushing an entry onto
`Context.receiverStack`, regardless of whether `Y` turns out to be a genuine
receiver method or a plain package-scope function — the compiler cannot tell
the two apart at compile time.

A genuine receiver method call consumes that entry via `popThis()`:

- A native passthrough method with a declared receiver (`dp.Declaration.Type
  != nil`) pops it in `callNative.go`.
- A wrapper-style runtime function (`func(*symbols.SymbolTable, data.List)
  (any, error)`) pops it unconditionally in `callRuntimeFunction.go:55`.
- A user-defined Ego method (`func (r Receiver) Method()`) pops it via the
  `GetThis` opcode compiled into the method's own prologue.

Before this fix, a native passthrough function with **no** receiver
(`dp.Declaration.Type == nil`, e.g. `os.Hostname`, `strconv.Itoa`) never
called `popThis()` at all — `callNative.go` only popped in the receiver
branch. The pushed entry was left on `receiverStack` and later wrongly
consumed by the next genuine receiver call that ran `popThis()` — typically
an *enclosing* call, since the natural way to trigger this is nesting the
no-receiver call inside another call's own argument list:

```go
f, _ := os.Create("test.txt")
f.WriteString("x" + os.Hostname())   // "no function receiver: WriteString"
```

Bytecode for the inner expression shows exactly where things go wrong:

```text
Load "f"
SetThis                  <- pushes f's receiver entry (for WriteString)
Member "WriteString"
Push "x"
Load "os"
SetThis                  <- pushes a SECOND entry (for Hostname, which has no receiver)
Member "Hostname"
Call 0                   <- Hostname has no receiver; its entry is never popped (before the fix)
Add
Call 1                   <- WriteString pops "this" and wrongly gets os's leftover entry
```

**Fixed for native/wrapper functions:** `callNative.go`'s no-receiver branch
now discards the stale entry with `_, _ = c.popThis()` before calling
`CallDirect`, keeping the receiver stack balanced. This covers every
`IsNative: true` function (e.g. `os.Hostname`, `strconv.Itoa`) and every
`func(*symbols.SymbolTable, data.List) (any, error)` wrapper function (the
latter already popped unconditionally in `callRuntimeFunction.go`, so it was
never actually affected).

**Still open:** a plain, no-receiver, Ego-*source*-defined function (e.g.
anything in `lib/packages/*.ego`, such as `io.DirList`) reproduces the same
bug when nested the same way:

```go
f, _ := os.Create("test.txt")
f.WriteString("x" + io.DirList("/tmp"))   // "no function receiver: WriteString"
```

This dispatches through `callBytecodeFunction` (the `*ByteCode` case in
`call.go`), not `callNative`. Unlike native functions, a `*ByteCode` value has
no reliable "does this function have a receiver" signal available at the
call-dispatch point: `Declaration().Type` (the field native functions set to
`data.OwnType` for methods) is never populated for user-defined functions,
receiver or not — the compiler tracks "has a receiver" only internally, at
compile time, via `thisName.Spelling() != ""` in `compiler/function.go`, and
uses that solely to decide whether to emit a `GetThis` opcode into the
callee's own prologue. Nothing currently surfaces that fact to the
caller-side dispatch code in `callBytecodeFunction`.

## CALL-11: Why "unconditionally pop in callBytecodeFunction" alone is unsafe

The originally-suggested fix ("have `callBytecodeFunction` pop
`receiverStack` unconditionally, mirroring `callRuntimeFunction`") turns out
to be unsafe by itself, which the bare-call variant above demonstrates
concretely: `callRuntimeFunction`/`callNative` can get away with treating
"receiver pending" as a function of the *callee's* shape only because every
call that reaches them was necessarily made via `X.Y(...)` dot syntax — a
wrapper or native function is never reachable any other way. `*ByteCode`
values have no such guarantee: an Ego-source function can be called via dot
syntax (`io.DirList(...)`) *or* as a bare identifier (a local variable
holding a function value, or a directly-named function call). Popping
unconditionally on every `*ByteCode` call — regardless of which syntax
produced it — pops an entry that was never pushed for a bare call, stealing
whatever the *enclosing* call's own `SetThis` had legitimately pushed. The
fix therefore could not rely on the callee's type or shape at all; it needed
a signal, verified at the exact call site, of whether *this* call had a
`SetThis` of its own.

## CALL-11: Fix — compile-time receiver signal + single dispatch-point pop

The compiler already knows, unambiguously, whether it just emitted `SetThis`
for the call about to be compiled — it's the same condition
(`compileDotReference` peeking that `(` follows) that triggers the emission
in the first place. That fact just wasn't being threaded through to the
`Call` instruction itself. The fix carries it there and moves *all* receiver
consumption to the one place that can act on it correctly:

- **`compiler.go`:** added `flagSet.pendingReceiverCall`, set to `true`
  immediately after `compileDotReference` (`expr_reference.go`) emits
  `SetThis`.
- **`expr_function.go`:** `functionCall()` captures and resets this flag
  *before* compiling any argument expressions (arguments can contain their
  own nested dot-calls that set and consume the same flag for an unrelated
  call — capturing late would read the wrong call's answer). The `Call`
  instruction's operand becomes `[]any{argc, true}` when a receiver is
  pending, or the unchanged bare `argc` int otherwise — every other emission
  site (`macro.go`, `var.go`, `function.go`'s `$new` copy, `exit.go`,
  `directives.go`, `testing.go`) is untouched and keeps emitting the bare
  form, which `callByteCode` treats exactly as before: no receiver pending.
- **`call.go`:** `callByteCode` parses either operand shape, and — when (and
  only when) `hasReceiver` is `true` — pops exactly one entry off
  `receiverStack`, once, before dispatching to *any* callee kind (`*data.Type`
  casts, `error`, the default-invalid-call case, and the three real callables
  all just receive/ignore this single, correctly-gated pop). This is the only
  remaining place `receiverStack` is ever popped.
- **`callNative.go`, `callRuntimeFunction.go`:** both now take the popped
  `(receiverValue, receiverOK)` as plain parameters instead of calling
  `c.popThis()` themselves. A receiver-less native function simply ignores
  them; a receiver-requiring one uses `receiverValue` directly (or errors if
  `!receiverOK`, unchanged from before).
- **`callBytecodeFunction.go`:** also takes `(receiverValue, receiverOK)` as
  parameters and unconditionally stages them into two new `*Context` fields,
  `pendingReceiver`/`pendingReceiverOK` (`context.go`), overwriting whatever
  was staged for the *previous* `*ByteCode` call every single time — so no
  staleness can ever survive across calls. `callBytecodeFunction` still has
  no way to know whether the callee it's about to run has a receiver at all;
  it no longer needs to. If the callee has no `GetThis` (a plain function),
  the staged value is simply never read and is overwritten by the next call —
  an implicit, zero-cost discard.
- **`this.go`:** `getThisByteCode` (`GetThis`) no longer touches
  `receiverStack` — it reads and clears `pendingReceiver`/`pendingReceiverOK`
  instead, applying the existing byValue-copy/pointer-boxing logic unchanged.
  Since `GetThis` runs as literally the first real instruction of a receiver
  method's compiled prologue (right after `PushScope`/`ArgCheck`/`InPackage`/
  `Module`, before any instruction that could itself dispatch a nested call),
  it always consumes exactly the value `callBytecodeFunction` staged for
  *this* call, before anything else has a chance to overwrite it.
- **`defer.go`:** the synthesized bytecode that replays a deferred call
  (`invokeDeferredStatements`, `invokePanicDefers`) builds its own `Call`
  instruction by hand and has no `SetThis` of its own — it seeds the replay
  context's `receiverStack` directly from whatever `deferByteCode` captured
  at `defer`-statement time. The new `emitDeferredCall` helper mirrors
  `functionCall()`'s operand choice: the two-element `hasReceiver` form when
  `deferTask.receiverStack` captured an entry, the bare form otherwise.
- **`this.go` (`SetPendingReceiver`), `internal/runtime/fmt/print.go`:** the
  one caller outside the `bytecode` package that builds and runs a
  `*bytecode.Context` by hand to invoke a `String()` method
  (`formatUsingString`) has no `Call` instruction for `callByteCode` to gate
  a pop on either. It now calls the new exported `Context.SetPendingReceiver`
  directly instead of the old `PushThis`, staging the receiver the same way
  `callBytecodeFunction` would have.

This consolidates every receiver-stack push/pop into `SetThis`/`LoadThis`
(push) and `callByteCode` (the only remaining pop), removing the callee-side
`GetThis` pop entirely and eliminating the whole asymmetry class by
construction: a value is now popped if and only if a `SetThis` genuinely
preceded *this* call, decided at compile time, never guessed at runtime from
the callee's shape.

**Regression coverage:**

- Go: `internal/language/bytecode/call11_test.go` (new — end-to-end
  `callByteCode`-level reproductions of both the original nested-dot-call
  bug and the bare-call variant, a defer-replay test, and an
  `emitDeferredCall` operand-shape unit test), plus updated tests in
  `callNative_test.go`, `callRuntimeFunction_test.go`,
  `callBytecodeFunction_test.go`, and `this_test.go` reflecting the new
  parameter-passing contract.
- Ego: `tests/functions/chained_receivers.ego` (extended with three new
  `@test` blocks: Ego-source no-receiver function nested in a receiver call,
  a bare-call variant, and several nested calls of different kinds in one
  receiver call), `tests/defer/method_receivers.ego` (new "deferred receiver
  call still binds" test, combining the defer-replay fix with a nested
  Ego-source call), `tests/types/user_type_string.ego` (new test combining
  the `formatUsingString`/`SetPendingReceiver` fix with a nested receiver
  call).
- Full existing suites (`go test ./...`, `go test -race ./...`, `ego test
  tests/` — 1654 tests) pass with no regressions.

While re-validating this issue and building its regression coverage, an
unrelated pre-existing bug was also found and recorded separately: see
[BUG-91](#BUG-91) (a pointer-receiver method called via `defer` on an
auto-addressed variable mutates a copy, not the original — reproduces
identically with no CALL-11-style nesting involved).
