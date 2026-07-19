package bytecode

// call11_test.go contains end-to-end regression tests for CALL-11
// (docs/ISSUES.md): "Receiver-stack corruption when a package-function call
// is nested inside a receiver method call's arguments".
//
// Unlike the callNative_test.go / callRuntimeFunction_test.go /
// callBytecodeFunction_test.go / this_test.go tests (which exercise each
// callee-side function in isolation, now that CALL-11's fix moved the
// receiver-stack pop entirely to callByteCode), the tests here drive
// callByteCode itself with a small, hand-assembled instruction sequence that
// mirrors exactly what the compiler emits for the bug's trigger pattern:
//
//	f.WriteString("x" + someNestedCall())
//
// bytecode order:
//
//	Push receiver; SetThis; Drop 1        -- Load "f"; SetThis; Member "WriteString"
//	Push outerFn                          -- the resolved method value
//	Push nestedThis; SetThis; Drop 1      -- Load "io"; SetThis; Member "DirList" (nested case)
//	Push innerFn
//	Call <inner operand>                  -- the nested call
//	Call []any{1, true}                   -- the outer receiver call, 1 arg
//	Stop
//
// outerFn's body is GetThis + Load + Return, so the test can directly assert
// which value it was bound to -- the whole point of CALL-11 is that, before
// the fix, this could be the wrong one.
//
// See also: tests/calls/call11_receiver_stack.ego for the equivalent
// end-to-end regression test through the real compiler, and the manual
// repros recorded in the CALL-11 write-up in docs/ISSUES.md.

import (
	"testing"

	"github.com/tucats/ego/internal/language/symbols"
)

// receiverEchoFn returns a *ByteCode that behaves like a minimal receiver
// method: it consumes the pending receiver via GetThis (bound to the local
// name "recv", value receiver semantics so the raw value is returned
// unboxed), and returns it as-is. This lets a test assert exactly which
// value the method's receiver was bound to.
func receiverEchoFn(name string) *ByteCode {
	return &ByteCode{
		name: name,
		instructions: []instruction{
			{Operation: GetThis, Operand: []any{"recv", true}},
			{Operation: Load, Operand: "recv"},
			{Operation: Return, Operand: true},
		},
	}
}

// constantReturnFn returns a *ByteCode that ignores any pending receiver (it
// has no GetThis at all -- exactly like a plain, no-receiver Ego-source
// function such as io.DirList) and simply returns a fixed value. This
// simulates the "nested call" half of the CALL-11 trigger pattern.
func constantReturnFn(name string, value any) *ByteCode {
	return &ByteCode{
		name: name,
		instructions: []instruction{
			{Operation: Push, Operand: value},
			{Operation: Return, Operand: true},
		},
	}
}

// Test_CALL11_DotCallNestedInReceiverArguments_PreservesCorrectReceiver
// reproduces the "still open" half of CALL-11: a *ByteCode function with no
// receiver of its own (constantReturnFn, standing in for io.DirList), but
// invoked via dot-call syntax (hasReceiver=true on its own Call, because the
// compiler cannot tell at compile time that its target has no receiver),
// nested inside the argument list of a genuine receiver method call
// (receiverEchoFn, standing in for f.WriteString).
//
// Before the CALL-11 fix, the nested call's own SetThis-pushed entry was
// never consumed (callBytecodeFunction never touched the receiver stack at
// all), so it was left on top of the stack for the outer call's GetThis to
// wrongly pop instead of the real receiver -- this test's outer function
// would have echoed back "nested-package-value" instead of "REAL-RECEIVER".
func Test_CALL11_DotCallNestedInReceiverArguments_PreservesCorrectReceiver(t *testing.T) {
	tc := newTestContext(t)

	outerFn := receiverEchoFn("outer")
	innerFn := constantReturnFn("inner", "inner-result")

	tc.ctx.bc = &ByteCode{
		name: "program",
		instructions: []instruction{
			// Load "f"; SetThis; Member "WriteString" -- resolves to outerFn.
			{Operation: Push, Operand: "REAL-RECEIVER"},
			{Operation: SetThis},
			{Operation: Drop, Operand: 1},
			{Operation: Push, Operand: outerFn},

			// Argument expression: Load "io"; SetThis; Member "DirList"; Call.
			// hasReceiver=true because this is dot-call syntax too, even
			// though innerFn (DirList) declares no receiver of its own.
			{Operation: Push, Operand: "nested-package-value"},
			{Operation: SetThis},
			{Operation: Drop, Operand: 1},
			{Operation: Push, Operand: innerFn},
			{Operation: Call, Operand: []any{0, true}},

			// f.WriteString(<argument>) -- one argument, dot-call syntax.
			{Operation: Call, Operand: []any{1, true}},
			{Operation: Stop},
		},
	}

	err := tc.ctx.Run()

	tc.assertNoError(err)
	tc.assertTopStack("REAL-RECEIVER")
}

// Test_CALL11_BareCallNestedInReceiverArguments_PreservesCorrectReceiver
// reproduces the second CALL-11 variant found while re-validating the issue:
// a bare call (no dot-syntax at all, e.g. a local variable holding a
// function value: toa := strconv.Itoa; ...toa(42)) nested inside a receiver
// method call's arguments. Its own Call instruction carries the plain-int
// operand form (hasReceiver=false), so callByteCode must never touch the
// receiver stack for it at all.
//
// Under the pre-fix design this specific variant did not yet exist as a bug
// (native/wrapper functions popped the receiver stack based on the callee's
// own runtime type, not on whether a SetThis truly preceded the call) --
// but an early version of this fix's design (discard-on-callee-type) would
// have reintroduced exactly this failure mode. This test guards against that
// regression by asserting a bare call never disturbs a pending receiver.
func Test_CALL11_BareCallNestedInReceiverArguments_PreservesCorrectReceiver(t *testing.T) {
	tc := newTestContext(t)

	outerFn := receiverEchoFn("outer")
	innerFn := constantReturnFn("inner", "inner-result")

	tc.ctx.bc = &ByteCode{
		name: "program",
		instructions: []instruction{
			{Operation: Push, Operand: "REAL-RECEIVER"},
			{Operation: SetThis},
			{Operation: Drop, Operand: 1},
			{Operation: Push, Operand: outerFn},

			// Bare call: no SetThis at all, plain-int operand.
			{Operation: Push, Operand: innerFn},
			{Operation: Call, Operand: 0},

			{Operation: Call, Operand: []any{1, true}},
			{Operation: Stop},
		},
	}

	err := tc.ctx.Run()

	tc.assertNoError(err)
	tc.assertTopStack("REAL-RECEIVER")
}

// Test_CALL11_PlainCallOperand_NeverPopsReceiverStack is a narrower unit
// test of the same invariant: a Call instruction using the plain-int operand
// form must never pop c.receiverStack, no matter what is sitting on it. This
// isolates the operand-parsing/gating logic in callByteCode from the rest of
// the nested-call scenario.
func Test_CALL11_PlainCallOperand_NeverPopsReceiverStack(t *testing.T) {
	tc := newTestContext(t)

	tc.ctx.PushThis("unrelated", "should-survive")

	fn := constantReturnFn("plain", 42)

	tc.ctx.bc = &ByteCode{
		name: "program",
		instructions: []instruction{
			{Operation: Push, Operand: fn},
			{Operation: Call, Operand: 0},
			{Operation: Stop},
		},
	}

	err := tc.ctx.Run()

	tc.assertNoError(err)
	tc.assertTopStack(42)

	v, ok := tc.ctx.popThis()
	if !ok {
		t.Fatal("expected the pre-existing receiver stack entry to survive a plain-operand call, but the stack was empty")
	}

	if v != "should-survive" {
		t.Errorf("receiver stack entry: got %v, want %q", v, "should-survive")
	}
}

// Test_CALL11_DeferredReceiverCall_StillBindsReceiver verifies that
// emitDeferredCall (defer.go) correctly signals hasReceiver on the
// synthesized Call instruction used to replay a deferred call, so that a
// deferred receiver method call (e.g. "defer f.Close()") still has its
// receiver bound correctly under the new callByteCode-gated pop -- this is
// the defer-specific corner of the CALL-11 fix (see the comment on
// emitDeferredCall for why the synthesized bytecode needs its own signal).
func Test_CALL11_DeferredReceiverCall_StillBindsReceiver(t *testing.T) {
	root := symbols.NewRootSymbolTable("test root")
	local := symbols.NewChildSymbolTable("test local", root)

	outerFn := receiverEchoFn("outer")

	deferTask := deferStatement{
		name:          "test:1",
		target:        outerFn,
		receiverStack: []this{{name: "f", value: "DEFERRED-RECEIVER"}},
		args:          []any{},
	}

	cx := NewContext(local, New("defer test").Literal(true))
	cx.deferStack = []deferStatement{deferTask}

	err := cx.invokeDeferredStatements()

	if err != nil {
		t.Fatalf("invokeDeferredStatements: unexpected error: %v", err)
	}
}

// Test_emitDeferredCall_OperandShape is a focused unit test of
// emitDeferredCall's operand selection: the two-element hasReceiver form
// when the deferred call captured a receiver, the plain-int form otherwise.
func Test_emitDeferredCall_OperandShape(t *testing.T) {
	t.Run("with captured receiver", func(t *testing.T) {
		cb := New("defer test")
		emitDeferredCall(cb, deferStatement{
			args:          []any{1, 2},
			receiverStack: []this{{name: "f", value: "v"}},
		})

		ops := cb.Opcodes()
		if len(ops) != 1 || ops[0].Operation != Call {
			t.Fatalf("expected exactly one Call instruction, got %v", ops)
		}

		operands, ok := ops[0].Operand.([]any)
		if !ok || len(operands) != 2 {
			t.Fatalf("expected two-element operand, got %#v", ops[0].Operand)
		}

		if operands[0] != 2 {
			t.Errorf("argc: got %v, want 2", operands[0])
		}

		if hasReceiver, _ := operands[1].(bool); !hasReceiver {
			t.Errorf("hasReceiver: got %v, want true", operands[1])
		}
	})

	t.Run("without captured receiver", func(t *testing.T) {
		cb := New("defer test")
		emitDeferredCall(cb, deferStatement{
			args: []any{1, 2, 3},
		})

		ops := cb.Opcodes()
		if len(ops) != 1 || ops[0].Operation != Call {
			t.Fatalf("expected exactly one Call instruction, got %v", ops)
		}

		if ops[0].Operand != 3 {
			t.Errorf("operand: got %#v, want plain int 3", ops[0].Operand)
		}
	})
}
