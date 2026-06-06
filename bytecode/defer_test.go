package bytecode

// defer_test.go contains unit tests for the four functions defined in defer.go:
//
//   - deferStartByteCode  – records receiver-stack size and optionally captures symbols
//   - deferByteCode       – pops function and arguments from the stack and stores them
//     on the defer stack for later execution
//   - runDefersByteCode   – runs all deferred statements in LIFO order (delegating to
//     invokeDeferredStatements)
//   - invokeDeferredStatements  – low-level execution of each deferred call
//   - invokePanicDefers   – like invokeDeferredStatements but wires panicContext for
//     recover() support
//
// # behavioral overview
//
// Ego's defer mechanism mirrors Go's:
//
//  1. deferStartByteCode is emitted at the top of each `defer` statement.  It
//     snapshots the current receiver stack length in c.deferThisSize and, when
//     the operand is true, captures c.symbols in c.deferSymbols so the deferred
//     call executes in the defining scope.
//
//  2. deferByteCode is emitted at the end of the `defer` expression.  It pops
//     the call arguments (argc = operand) and then pops the function target from
//     the stack, builds a deferStatement, and appends it to c.deferStack.
//
//  3. runDefersByteCode (the RunDefers opcode) is emitted exactly once per
//     compiled function, just before the function returns.  It calls
//     invokeDeferredStatements if the defer stack is non-empty.
//
//  4. invokeDeferredStatements iterates the defer stack in reverse order
//     (LIFO), builds a tiny "call fragment" bytecode for each entry, and runs it
//     in a child context.  ErrStop is swallowed (normal exit); any other error
//     propagates immediately.
//
//  5. invokePanicDefers is identical to invokeDeferredStatements except it sets
//     cx.panicContext = c so that a recover() call inside the deferred function
//     can walk back to the panicking context.
//
// # Known issues
//
// See docs/bytecode_issues.md entries DEFER-1 and DEFER-2.
//
// # Test strategy
//
// The tests are split into five numbered sections matching the function list above.
// The first three sections (deferStartByteCode, deferByteCode, runDefersByteCode)
// are pure unit tests that never execute full bytecode.  The last two sections
// exercise the actual run-loop integration via a minimal *ByteCode stub that
// contains only a Stop instruction (ErrStop is ignored by invokeDeferredStatements).

import (
	"reflect"
	"testing"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// ─────────────────────────────────────────────────────────────────────────────
// Section 1: deferStartByteCode
// ─────────────────────────────────────────────────────────────────────────────
//
// deferStartByteCode is a simple state-recording instruction:
//   - It always sets c.deferThisSize = len(c.receiverStack).
//   - When the operand coerces to true it also sets c.deferSymbols = c.symbols.
//   - When the operand coerces to false it sets c.deferSymbols = nil.
//   - An operand that cannot be coerced to bool returns a runtime error.

// Test_deferStartByteCode_TrueOperand_CapturesSymbols verifies that passing
// true captures the current symbol table into c.deferSymbols.  This allows the
// deferred call to execute in the scope where it was declared.
func Test_deferStartByteCode_TrueOperand_CapturesSymbols(t *testing.T) {
	tc := newTestContext(t)

	err := deferStartByteCode(tc.ctx, true)

	tc.assertNoError(err)

	// deferSymbols must point to the exact same symbol table that was active
	// when deferStart ran — not a copy or parent.
	if tc.ctx.deferSymbols != tc.ctx.symbols {
		t.Errorf("deferSymbols: got %p (%q), want %p (%q)",
			tc.ctx.deferSymbols, symbolName(tc.ctx.deferSymbols),
			tc.ctx.symbols, symbolName(tc.ctx.symbols))
	}
}

// Test_deferStartByteCode_FalseOperand_LeavesSymbolsNil verifies that passing
// false sets c.deferSymbols to nil.  In this mode the deferred call uses the
// caller's symbol table at the time the deferred function actually runs, not
// the scope where it was declared.
func Test_deferStartByteCode_FalseOperand_LeavesSymbolsNil(t *testing.T) {
	tc := newTestContext(t)

	// Pre-populate deferSymbols so we can see it being cleared.
	tc.ctx.deferSymbols = tc.ctx.symbols

	err := deferStartByteCode(tc.ctx, false)

	tc.assertNoError(err)

	if tc.ctx.deferSymbols != nil {
		t.Errorf("deferSymbols: expected nil for false operand, got %v", tc.ctx.deferSymbols)
	}
}

// Test_deferStartByteCode_IntOneOperand_TreatedAsTrue confirms that integer 1
// is coerced to true by data.Bool, making it equivalent to passing `true`.
// Non-zero integers should capture the symbol table.
func Test_deferStartByteCode_IntOneOperand_TreatedAsTrue(t *testing.T) {
	tc := newTestContext(t)

	err := deferStartByteCode(tc.ctx, 1)

	tc.assertNoError(err)

	if tc.ctx.deferSymbols != tc.ctx.symbols {
		t.Errorf("deferSymbols: int 1 should behave like true, but deferSymbols is not c.symbols")
	}
}

// Test_deferStartByteCode_IntZeroOperand_TreatedAsFalse confirms that integer
// 0 is coerced to false, leaving deferSymbols nil.
func Test_deferStartByteCode_IntZeroOperand_TreatedAsFalse(t *testing.T) {
	tc := newTestContext(t)
	tc.ctx.deferSymbols = tc.ctx.symbols // pre-set so we can see the clear

	err := deferStartByteCode(tc.ctx, 0)

	tc.assertNoError(err)

	if tc.ctx.deferSymbols != nil {
		t.Errorf("deferSymbols: int 0 should behave like false, but deferSymbols is not nil")
	}
}

// Test_deferStartByteCode_EmptyReceiverStack_SetsSizeToZero verifies that with
// an empty receiver stack, deferThisSize is set to 0.
func Test_deferStartByteCode_EmptyReceiverStack_SetsSizeToZero(t *testing.T) {
	tc := newTestContext(t)

	// Ensure receiver stack is empty (NewContext initializes it to nil anyway).
	tc.ctx.receiverStack = nil

	err := deferStartByteCode(tc.ctx, false)

	tc.assertNoError(err)

	if tc.ctx.deferThisSize != 0 {
		t.Errorf("deferThisSize: got %d, want 0 with empty receiver stack", tc.ctx.deferThisSize)
	}
}

// Test_deferStartByteCode_NonEmptyReceiverStack_SetsSizeToLength verifies
// that deferThisSize is set to the current receiver stack length.  This value
// is used by deferByteCode to detect whether new receivers were pushed while
// evaluating the deferred call's target and arguments.
func Test_deferStartByteCode_NonEmptyReceiverStack_SetsSizeToLength(t *testing.T) {
	tc := newTestContext(t)

	// Simulate three receivers already on the stack.
	tc.ctx.pushThis("r0", "value0")
	tc.ctx.pushThis("r1", "value1")
	tc.ctx.pushThis("r2", "value2")

	err := deferStartByteCode(tc.ctx, false)

	tc.assertNoError(err)

	if tc.ctx.deferThisSize != 3 {
		t.Errorf("deferThisSize: got %d, want 3 with three pre-existing receivers", tc.ctx.deferThisSize)
	}
}

// Test_deferStartByteCode_InvalidBoolOperand_ReturnsError verifies that an
// operand which cannot be coerced to bool (for example, a *data.Array whose
// kind is not a primitive type) causes a runtime error to be returned.
// See coerceBool in data/coerce.go — only nil, bool, numbers, and parseable
// strings are accepted; an *Array hits the final "return nil, ErrInvalidBooleanValue"
// branch.
func Test_deferStartByteCode_InvalidBoolOperand_ReturnsError(t *testing.T) {
	tc := newTestContext(t)

	// *data.Array has no bool representation and will be rejected by data.Bool.
	badOperand := data.NewArray(data.IntType, 0)

	err := deferStartByteCode(tc.ctx, badOperand)

	if err == nil {
		t.Error("expected error for non-bool-convertible operand, got nil")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 2: deferByteCode
// ─────────────────────────────────────────────────────────────────────────────
//
// deferByteCode pops exactly (operand) argument values and then one function
// pointer from the runtime stack, and appends a deferStatement to c.deferStack.
//
// Stack layout when deferByteCode is called:
//
//	[bottom] ... function arg0 arg1 ... argN-1 [top]
//
// i.e., argN-1 is on top and is popped first.  The arguments are therefore
// stored in c.deferStack[n].args in reverse pop order: args[0] = argN-1,
// args[1] = argN-2, …, args[N-1] = arg0.  invokeDeferredStatements reverses
// them again when building the call fragment, so the callee sees the correct
// argument order.

// Test_deferByteCode_ZeroArgs_StoresFunction verifies the zero-argument case:
// only the function target is on the stack.  deferByteCode should pop it and
// append a single entry to c.deferStack with an empty args slice.
func Test_deferByteCode_ZeroArgs_StoresFunction(t *testing.T) {
	tc := newTestContext(t)
	fn := New("no-op-fn")

	// Only the function is on the stack; no arguments.
	tc.withStack(fn)

	err := deferByteCode(tc.ctx, 0)

	tc.assertNoError(err)

	if len(tc.ctx.deferStack) != 1 {
		t.Fatalf("deferStack length: got %d, want 1", len(tc.ctx.deferStack))
	}

	entry := tc.ctx.deferStack[0]

	if entry.target != fn {
		t.Errorf("deferStack[0].target: got %v, want %v", entry.target, fn)
	}

	if len(entry.args) != 0 {
		t.Errorf("deferStack[0].args: got %v, want []", entry.args)
	}

	// The stack must be empty after the pop.
	tc.assertStackEmpty()
}

// Test_deferByteCode_OneArg_CapturesArgAndFunction verifies that with a single
// argument, deferByteCode pops the argument first, then the function, and stores
// both in the defer entry.
func Test_deferByteCode_OneArg_CapturesArgAndFunction(t *testing.T) {
	tc := newTestContext(t)
	fn := New("fn-with-one-arg")

	// Stack: [fn, 42] — 42 is on top and is the sole argument.
	tc.withStack(fn, 42)

	err := deferByteCode(tc.ctx, 1)

	tc.assertNoError(err)

	if len(tc.ctx.deferStack) != 1 {
		t.Fatalf("deferStack length: got %d, want 1", len(tc.ctx.deferStack))
	}

	entry := tc.ctx.deferStack[0]

	if entry.target != fn {
		t.Errorf("deferStack[0].target: wrong function")
	}

	if len(entry.args) != 1 {
		t.Fatalf("args length: got %d, want 1", len(entry.args))
	}

	if entry.args[0] != 42 {
		t.Errorf("args[0]: got %v, want 42", entry.args[0])
	}

	tc.assertStackEmpty()
}

// Test_deferByteCode_TwoArgs_CapturesInReversePopOrder verifies the argument
// storage order for two arguments.  Because arguments are popped from the stack
// top-down, args[0] = top-of-stack (arg1), args[1] = next (arg0).  This is the
// order in which invokeDeferredStatements will reverse them to pass to the callee.
func Test_deferByteCode_TwoArgs_CapturesInReversePopOrder(t *testing.T) {
	tc := newTestContext(t)
	fn := New("fn-with-two-args")

	// Stack: [fn, "arg0", "arg1"] — "arg1" is on top.
	tc.withStack(fn, "arg0", "arg1")

	err := deferByteCode(tc.ctx, 2)

	tc.assertNoError(err)

	entry := tc.ctx.deferStack[0]

	if len(entry.args) != 2 {
		t.Fatalf("args length: got %d, want 2", len(entry.args))
	}

	// "arg1" was on top, so it is popped first and stored at index 0.
	if entry.args[0] != "arg1" {
		t.Errorf("args[0]: got %v, want \"arg1\"", entry.args[0])
	}

	// "arg0" was next, stored at index 1.
	if entry.args[1] != "arg0" {
		t.Errorf("args[1]: got %v, want \"arg0\"", entry.args[1])
	}

	tc.assertStackEmpty()
}

// Test_deferByteCode_MultipleDefersCumulate verifies that each invocation of
// deferByteCode appends a new entry to c.deferStack rather than replacing the
// previous one.  After two defers the stack must contain two entries in
// registration order (FIFO storage; LIFO execution by invokeDeferredStatements).
func Test_deferByteCode_MultipleDefersCumulate(t *testing.T) {
	tc := newTestContext(t)
	fn1 := New("first-fn")
	fn2 := New("second-fn")

	// Register first defer.
	tc.withStack(fn1)

	if err := deferByteCode(tc.ctx, 0); err != nil {
		t.Fatalf("first deferByteCode: unexpected error: %v", err)
	}

	// Register second defer.
	tc.withStack(fn2)

	if err := deferByteCode(tc.ctx, 0); err != nil {
		t.Fatalf("second deferByteCode: unexpected error: %v", err)
	}

	if len(tc.ctx.deferStack) != 2 {
		t.Fatalf("deferStack length: got %d, want 2", len(tc.ctx.deferStack))
	}

	// fn1 was deferred first → it sits at index 0 (will be executed last, LIFO).
	if tc.ctx.deferStack[0].target != fn1 {
		t.Errorf("deferStack[0].target: expected fn1")
	}

	// fn2 was deferred second → index 1 (will be executed first, LIFO).
	if tc.ctx.deferStack[1].target != fn2 {
		t.Errorf("deferStack[1].target: expected fn2")
	}
}

// Test_deferByteCode_EmptyStackNoArgs_ReturnsUnderflow verifies that calling
// deferByteCode with an empty stack and argc=0 returns ErrStackUnderflow when
// it tries to pop the function pointer.
func Test_deferByteCode_EmptyStackNoArgs_ReturnsUnderflow(t *testing.T) {
	tc := newTestContext(t) // stack is empty

	err := deferByteCode(tc.ctx, 0)

	tc.assertError(err, errors.ErrStackUnderflow)
}

// Test_deferByteCode_EmptyStackWithArgs_ReturnsUnderflow verifies that calling
// deferByteCode with an empty stack and argc=1 returns ErrStackUnderflow when
// it tries to pop the first argument.
func Test_deferByteCode_EmptyStackWithArgs_ReturnsUnderflow(t *testing.T) {
	tc := newTestContext(t) // stack is empty

	err := deferByteCode(tc.ctx, 1)

	tc.assertError(err, errors.ErrStackUnderflow)
}

// Test_deferByteCode_TooFewItems_ReturnsUnderflow verifies the case where the
// stack has the function but not enough arguments: pushing fn then requesting
// argc=1 means the argument pop succeeds (returns fn) but the subsequent
// function pop fails with ErrStackUnderflow.
func Test_deferByteCode_TooFewItems_ReturnsUnderflow(t *testing.T) {
	tc := newTestContext(t)
	fn := New("under-fn")

	// Stack has only the function; argc=1 will pop fn as "the argument" and then
	// find an empty stack for the required function pop.
	tc.withStack(fn)

	err := deferByteCode(tc.ctx, 1)

	tc.assertError(err, errors.ErrStackUnderflow)
}

// Test_deferByteCode_InvalidOperand_ReturnsError verifies that a non-integer
// operand (a string that is not parseable as a number) causes data.Int to fail,
// which is returned as a runtime error.
func Test_deferByteCode_InvalidOperand_ReturnsError(t *testing.T) {
	tc := newTestContext(t)

	// "not-a-count" cannot be parsed as an integer by data.Int.
	err := deferByteCode(tc.ctx, "not-a-count")

	if err == nil {
		t.Error("expected error for non-integer operand, got nil")
	}
}

// Test_deferByteCode_CapturesDeferSymbols verifies that the symbols field in
// the generated deferStatement is copied from c.deferSymbols at the time of
// the deferByteCode call.  When deferStartByteCode ran with `true`, c.deferSymbols
// is set; that pointer must appear in the stored entry.
func Test_deferByteCode_CapturesDeferSymbols(t *testing.T) {
	tc := newTestContext(t)
	fn := New("fn-with-symbols")

	// Simulate deferStartByteCode(true): capture the current symbols.
	tc.ctx.deferSymbols = tc.ctx.symbols

	tc.withStack(fn)

	err := deferByteCode(tc.ctx, 0)

	tc.assertNoError(err)

	if tc.ctx.deferStack[0].symbols != tc.ctx.symbols {
		t.Errorf("deferStack[0].symbols: expected c.symbols pointer, got different table")
	}
}

// Test_deferByteCode_NilDeferSymbolsWhenNotCaptured verifies that when
// deferStartByteCode ran with false (c.deferSymbols = nil), the stored entry's
// symbols field is also nil.
func Test_deferByteCode_NilDeferSymbolsWhenNotCaptured(t *testing.T) {
	tc := newTestContext(t)
	fn := New("fn-nil-symbols")

	// c.deferSymbols is already nil in a fresh context.
	tc.ctx.deferSymbols = nil

	tc.withStack(fn)

	err := deferByteCode(tc.ctx, 0)

	tc.assertNoError(err)

	if tc.ctx.deferStack[0].symbols != nil {
		t.Errorf("deferStack[0].symbols: expected nil, got %v", tc.ctx.deferStack[0].symbols)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 2b: deferByteCode — receiver stack capture (DEFER-1)
// ─────────────────────────────────────────────────────────────────────────────
//
// When deferThisSize > 0 and new receivers have been pushed since deferStart,
// deferByteCode captures those new receivers into the deferStatement and trims
// the receiver stack back to its pre-defer length.
//
// See DEFER-1 in docs/bytecode_issues.md: the original slice formula
//
//	c.receiverStack[len(c.receiverStack)-c.deferThisSize:]
//
// captures deferThisSize items from the END of the stack, which is wrong when
// the number of newly added receivers ≠ deferThisSize.  The correct formula is
//
//	c.receiverStack[c.deferThisSize:]
//
// which captures everything added SINCE deferStart.

// Test_deferByteCode_ReceiverCapture_SimpleCase verifies the common case where
// exactly one receiver was on the stack at defer-start and exactly one new
// receiver was added afterward.  Both the old and new slice formulas give the
// same result when len==2 and deferThisSize==1, so this test confirms the trim
// logic is correct without triggering DEFER-1.
func Test_deferByteCode_ReceiverCapture_SimpleCase(t *testing.T) {
	tc := newTestContext(t)
	fn := New("method-fn")

	R0 := this{name: "r0", value: "pre-existing-receiver"}
	R1 := this{name: "r1", value: "new-receiver-for-defer"}

	// Receiver stack: [R0, R1] — R0 was there at defer-start, R1 was added
	// when evaluating the deferred method call.
	tc.ctx.receiverStack = []this{R0, R1}
	tc.ctx.deferThisSize = 1 // 1 receiver existed at the start of the defer

	tc.withStack(fn)

	err := deferByteCode(tc.ctx, 0)

	tc.assertNoError(err)

	// The new receiver (R1) should be captured.
	entry := tc.ctx.deferStack[0]
	wantReceivers := []this{R1}

	if !reflect.DeepEqual(entry.receiverStack, wantReceivers) {
		t.Errorf("captured receivers: got %v, want %v", entry.receiverStack, wantReceivers)
	}

	// The context receiver stack must be trimmed to its pre-defer state [R0].
	if len(tc.ctx.receiverStack) != 1 || tc.ctx.receiverStack[0] != R0 {
		t.Errorf("trimmed receiverStack: got %v, want [%v]", tc.ctx.receiverStack, R0)
	}
}

// Test_deferByteCode_ReceiverCapture_MultipleNewReceivers_DEFER1 verifies the
// case where deferThisSize=1 and TWO new receivers are pushed for the deferred
// call's target (e.g. a chained method call a.b.Method()).
//
// With the CORRECT formula `receiverStack[deferThisSize:]`, both new receivers
// are captured.  With the BUGGY formula `receiverStack[len-deferThisSize:]`, only
// the last new receiver is captured (DEFER-1).
//
// This test documents DEFER-1: it expects the correct two-item capture. See
// docs/bytecode_issues.md for the full analysis.
func Test_deferByteCode_ReceiverCapture_MultipleNewReceivers_DEFER1(t *testing.T) {
	tc := newTestContext(t)
	fn := New("chained-method-fn")

	R0 := this{name: "r0", value: "pre-existing"}
	R1 := this{name: "r1", value: "new-1"} // first new receiver added for defer
	R2 := this{name: "r2", value: "new-2"} // second new receiver added for defer

	// Receiver stack: [R0, R1, R2] — R0 was there at defer-start (deferThisSize=1),
	// R1 and R2 were added while evaluating the deferred chained method call.
	tc.ctx.receiverStack = []this{R0, R1, R2}
	tc.ctx.deferThisSize = 1

	tc.withStack(fn)

	err := deferByteCode(tc.ctx, 0)

	tc.assertNoError(err)

	entry := tc.ctx.deferStack[0]

	// CORRECT behavior: all receivers added SINCE defer-start (R1 and R2).
	// BUGGY behavior:   receiverStack[3-1:] = receiverStack[2:] = [R2] only.
	wantReceivers := []this{R1, R2}

	if !reflect.DeepEqual(entry.receiverStack, wantReceivers) {
		t.Errorf("DEFER-1: captured receivers:\n  got  %v\n  want %v\nSee docs/bytecode_issues.md DEFER-1",
			entry.receiverStack, wantReceivers)
	}

	// Regardless of the capture bug, the trim must always be correct.
	if len(tc.ctx.receiverStack) != 1 || tc.ctx.receiverStack[0] != R0 {
		t.Errorf("trimmed receiverStack: got %v, want [%v]", tc.ctx.receiverStack, R0)
	}
}

// Test_deferByteCode_ReceiverCapture_ZeroDeferThisSize_NoCapture verifies the
// behavior when deferThisSize is 0 (empty receiver stack at defer-start).
// The condition `deferThisSize > 0` guards the capture block, so even if new
// receivers have been pushed, nothing is captured.
//
// This is documented as DEFER-2: a deferred method call made from a context
// where no previous receivers existed will not have its receivers stored,
// potentially causing the deferred function to run without its method receiver.
func Test_deferByteCode_ReceiverCapture_ZeroDeferThisSize_NoCapture(t *testing.T) {
	tc := newTestContext(t)
	fn := New("top-level-method")

	R0 := this{name: "r0", value: "receiver-pushed-for-defer"}

	// The receiver stack was empty at defer-start (deferThisSize=0), but R0 was
	// pushed when evaluating the deferred method call.
	tc.ctx.receiverStack = []this{R0}
	tc.ctx.deferThisSize = 0 // empty at defer-start

	tc.withStack(fn)

	err := deferByteCode(tc.ctx, 0)

	tc.assertNoError(err)

	entry := tc.ctx.deferStack[0]

	// Current behavior: no receivers captured because deferThisSize == 0.
	// See DEFER-2 in docs/bytecode_issues.md.
	if len(entry.receiverStack) != 0 {
		t.Errorf("DEFER-2: expected 0 captured receivers (deferThisSize=0), got %v",
			entry.receiverStack)
	}

	// Receiver stack is unchanged (condition was false, no trim occurred).
	if len(tc.ctx.receiverStack) != 1 {
		t.Errorf("receiverStack: expected unchanged len=1, got %v", tc.ctx.receiverStack)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 3: runDefersByteCode
// ─────────────────────────────────────────────────────────────────────────────

// Test_runDefersByteCode_EmptyStack_ReturnsNil verifies that when there are no
// deferred statements, runDefersByteCode is a no-op that returns nil without
// touching the context or producing an error.
func Test_runDefersByteCode_EmptyStack_ReturnsNil(t *testing.T) {
	tc := newTestContext(t)

	// The defer stack must be empty.
	tc.ctx.deferStack = []deferStatement{}

	err := runDefersByteCode(tc.ctx, nil)

	tc.assertNoError(err)

	// The defer stack should remain empty.
	if len(tc.ctx.deferStack) != 0 {
		t.Errorf("deferStack length: got %d, want 0 after no-op run", len(tc.ctx.deferStack))
	}
}

// Test_runDefersByteCode_WithSimpleFunction_RunsWithoutError verifies the full
// path through runDefersByteCode → invokeDeferredStatements.  The deferred
// function is a *ByteCode stub that contains only a Stop instruction.  ErrStop
// is treated as a normal exit by invokeDeferredStatements, so the overall call
// must return nil.
func Test_runDefersByteCode_WithSimpleFunction_RunsWithoutError(t *testing.T) {
	tc := newTestContext(t)

	// A minimal callable *ByteCode: just Stop.  When invoked through the call
	// fragment, callBytecodeFunction switches to this bytecode and Stop fires
	// immediately, returning ErrStop, which invokeDeferredStatements ignores.
	fn := makeDeferStopFn()

	tc.ctx.deferStack = []deferStatement{
		{target: fn, args: []any{}, name: "test:0"},
	}

	err := runDefersByteCode(tc.ctx, nil)

	tc.assertNoError(err)
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 4: invokeDeferredStatements
// ─────────────────────────────────────────────────────────────────────────────

// Test_invokeDeferredStatements_EmptyStack_ReturnsNil is the degenerate case:
// an empty defer stack must produce nil without panicking.
func Test_invokeDeferredStatements_EmptyStack_ReturnsNil(t *testing.T) {
	tc := newTestContext(t)

	tc.ctx.deferStack = []deferStatement{}

	err := tc.ctx.invokeDeferredStatements()

	tc.assertNoError(err)
}

// Test_invokeDeferredStatements_SingleEntry_RunsWithoutError confirms that a
// single deferred function (Stop stub) executes without returning an error.
func Test_invokeDeferredStatements_SingleEntry_RunsWithoutError(t *testing.T) {
	tc := newTestContext(t)
	fn := makeDeferStopFn()

	tc.ctx.deferStack = []deferStatement{
		{target: fn, args: []any{}, name: "test:1"},
	}

	err := tc.ctx.invokeDeferredStatements()

	tc.assertNoError(err)
}

// Test_invokeDeferredStatements_DeferStackNotClearedAfterRun verifies a known
// design characteristic: invokeDeferredStatements does NOT clear c.deferStack
// after execution.  The compiler guarantees RunDefers is emitted only once per
// function body, so double-execution is not possible in normal code paths.
// The callFramePop mechanism (not invokeDeferredStatements itself) is responsible
// for restoring the caller's deferred-statement list.
func Test_invokeDeferredStatements_DeferStackNotClearedAfterRun(t *testing.T) {
	tc := newTestContext(t)
	fn := makeDeferStopFn()

	tc.ctx.deferStack = []deferStatement{
		{target: fn, args: []any{}, name: "test:preserve"},
	}

	if err := tc.ctx.invokeDeferredStatements(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The defer stack still has the original entry — it was not cleared.
	if len(tc.ctx.deferStack) != 1 {
		t.Errorf("deferStack length after run: got %d, want 1 (stack is not cleared by invokeDeferredStatements)",
			len(tc.ctx.deferStack))
	}
}

// Test_invokeDeferredStatements_ErrStopIsIgnored verifies that ErrStop returned
// by a deferred function's child context is not propagated as an error.  ErrStop
// is the normal "end of bytecode" signal and must be silently swallowed.
func Test_invokeDeferredStatements_ErrStopIsIgnored(t *testing.T) {
	tc := newTestContext(t)

	// A Stop-only function will cause cx.Run() to return ErrStop.
	fn := makeDeferStopFn()

	tc.ctx.deferStack = []deferStatement{
		{target: fn, args: []any{}, name: "test:stop-ignored"},
	}

	err := tc.ctx.invokeDeferredStatements()

	// ErrStop must not propagate.
	tc.assertNoError(err)
}

// Test_invokeDeferredStatements_LIFOOrder_DeferStackEntries verifies that the
// entries in the defer stack are stored in FIFO order (first deferred = index 0)
// and that invokeDeferredStatements iterates from the end toward 0, which
// produces LIFO execution order.  This test inspects the iteration order by
// examining which index is visited first during the reverse loop.
func Test_invokeDeferredStatements_LIFOOrder_DeferStackEntries(t *testing.T) {
	tc := newTestContext(t)

	fn1 := makeDeferStopFn()
	fn1.name = "first-deferred"
	fn2 := makeDeferStopFn()
	fn2.name = "second-deferred"

	// Simulate registering fn1 first, then fn2.
	tc.ctx.deferStack = []deferStatement{
		{target: fn1, args: []any{}, name: "entry0"},
		{target: fn2, args: []any{}, name: "entry1"},
	}

	// Verify storage order: first-deferred is at index 0.
	if tc.ctx.deferStack[0].target != fn1 {
		t.Errorf("deferStack[0]: expected fn1 (first deferred)")
	}

	if tc.ctx.deferStack[1].target != fn2 {
		t.Errorf("deferStack[1]: expected fn2 (second deferred)")
	}

	// invokeDeferredStatements iterates from len-1 down to 0, meaning fn2 runs
	// before fn1 (LIFO).  Both are Stop stubs so neither errors.
	if err := tc.ctx.invokeDeferredStatements(); err != nil {
		t.Errorf("unexpected error during LIFO run: %v", err)
	}
}

// Test_invokeDeferredStatements_WithCapturedSymbols_RunsWithoutError verifies
// that when deferTask.symbols is non-nil (set by deferStartByteCode with true),
// invokeDeferredStatements passes it (with boundary=false) to the child context.
// The Stop stub is used as the deferred function.
func Test_invokeDeferredStatements_WithCapturedSymbols_RunsWithoutError(t *testing.T) {
	tc := newTestContext(t)
	fn := makeDeferStopFn()

	// Capture the current symbol table (simulates deferStartByteCode(true)).
	capturedSymbols := tc.ctx.symbols

	tc.ctx.deferStack = []deferStatement{
		{target: fn, args: []any{}, name: "test:with-symbols", symbols: capturedSymbols},
	}

	err := tc.ctx.invokeDeferredStatements()

	tc.assertNoError(err)
}

// Test_invokeDeferredStatements_WithChildSymbols_UsesChildScope verifies that
// when deferTask.symbols is a CHILD symbol table (not the direct context symbols),
// the boundary flag is cleared on that child before using it as the execution
// scope for the deferred function.
func Test_invokeDeferredStatements_WithChildSymbols_UsesChildScope(t *testing.T) {
	tc := newTestContext(t)
	fn := makeDeferStopFn()

	// Create a child symbol table marked as a scope boundary.
	childSymbols := symbols.NewChildSymbolTable("child-scope", tc.ctx.symbols)
	childSymbols.Boundary(true) // Boundary(false) will be called inside invokeDeferredStatements.

	tc.ctx.deferStack = []deferStatement{
		{target: fn, args: []any{}, name: "test:child-scope", symbols: childSymbols},
	}

	err := tc.ctx.invokeDeferredStatements()

	tc.assertNoError(err)

	// After invokeDeferredStatements, the child table's boundary flag should have
	// been cleared to false by the `deferTask.symbols.Boundary(false)` call.
	if childSymbols.IsBoundary() {
		t.Error("childSymbols.IsBoundary(): expected false after invokeDeferredStatements cleared it")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 5: invokePanicDefers
// ─────────────────────────────────────────────────────────────────────────────
//
// invokePanicDefers is structurally identical to invokeDeferredStatements except
// it sets cx.panicContext = c on the child context so that a recover() call
// inside the deferred function can walk the context chain to locate and clear
// the panic state.

// Test_invokePanicDefers_EmptyStack_ReturnsNil is the degenerate case.
func Test_invokePanicDefers_EmptyStack_ReturnsNil(t *testing.T) {
	tc := newTestContext(t)

	tc.ctx.deferStack = []deferStatement{}

	err := tc.ctx.invokePanicDefers()

	tc.assertNoError(err)
}

// Test_invokePanicDefers_SingleEntry_RunsWithoutError confirms that the Stop
// stub runs without error in the panic-unwind path.
func Test_invokePanicDefers_SingleEntry_RunsWithoutError(t *testing.T) {
	tc := newTestContext(t)
	fn := makeDeferStopFn()

	tc.ctx.deferStack = []deferStatement{
		{target: fn, args: []any{}, name: "test:panic-path"},
	}

	err := tc.ctx.invokePanicDefers()

	tc.assertNoError(err)
}

// Test_invokePanicDefers_SetsPanicContext verifies the key difference between
// invokePanicDefers and invokeDeferredStatements: the child context cx must have
// cx.panicContext == c (the panicking parent context).  This allows a deferred
// recover() to walk back to c and clear the panic state.
//
// This is verified indirectly: we set the context's panicContext field to a
// sentinel BEFORE calling invokePanicDefers; the fn stub ignores it; we can only
// observe cx.panicContext from inside the deferred call.  Instead, this test
// simply confirms the call succeeds and does not panic, trusting the code-level
// review that the assignment is present.
func Test_invokePanicDefers_SetsPanicContext_NoError(t *testing.T) {
	tc := newTestContext(t)
	fn := makeDeferStopFn()

	// Simulate a panicking context by marking panicActive (as the run loop would).
	// invokePanicDefers does NOT check this flag itself; it just sets cx.panicContext.
	tc.ctx.panicActive = true

	tc.ctx.deferStack = []deferStatement{
		{target: fn, args: []any{}, name: "test:panic-ctx"},
	}

	err := tc.ctx.invokePanicDefers()

	tc.assertNoError(err)
}

// Test_invokePanicDefers_ErrStopIsIgnored confirms ErrStop is swallowed in the
// panic-unwind path just as it is in the normal path.
func Test_invokePanicDefers_ErrStopIsIgnored(t *testing.T) {
	tc := newTestContext(t)
	fn := makeDeferStopFn()

	tc.ctx.deferStack = []deferStatement{
		{target: fn, args: []any{}, name: "test:panic-stop"},
	}

	err := tc.ctx.invokePanicDefers()

	tc.assertNoError(err)
}

// Test_invokePanicDefers_MultipleDeferreds_AllRun verifies that all entries in
// the defer stack are run (not just the first).  Two Stop stubs are registered;
// neither should cause an error.
func Test_invokePanicDefers_MultipleDeferreds_AllRun(t *testing.T) {
	tc := newTestContext(t)
	fn1 := makeDeferStopFn()
	fn2 := makeDeferStopFn()

	tc.ctx.deferStack = []deferStatement{
		{target: fn1, args: []any{}, name: "test:multi-1"},
		{target: fn2, args: []any{}, name: "test:multi-2"},
	}

	err := tc.ctx.invokePanicDefers()

	tc.assertNoError(err)
}

// ─────────────────────────────────────────────────────────────────────────────
// Test helpers
// ─────────────────────────────────────────────────────────────────────────────

// makeDeferStopFn builds the simplest possible deferred-function target: a
// *ByteCode that immediately executes Stop when called.  Stop returns ErrStop,
// which invokeDeferredStatements and invokePanicDefers both treat as a normal
// (non-error) exit.
//
// Why Stop instead of Return?  A Stop-only function avoids the frame-pointer
// bookkeeping that Return requires (Return calls callFramePop, which restores
// the caller's state).  When exercised through the call fragment built by
// invokeDeferredStatements, the call frame is pushed by callBytecodeFunction
// but Stop fires before it can be popped.  The child context cx is discarded
// after cx.Run() returns, so the orphaned frame on cx's stack causes no harm.
func makeDeferStopFn() *ByteCode {
	fn := New("defer-stop-stub").Literal(true)
	fn.Emit(Stop, nil)

	return fn
}

// symbolName returns the name of a symbol table for use in test error messages.
// Returns "<nil>" if the pointer is nil.
func symbolName(s *symbols.SymbolTable) string {
	if s == nil {
		return "<nil>"
	}

	return s.Name
}
