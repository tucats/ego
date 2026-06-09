package bytecode

// return_test.go contains unit tests for returnByteCode in return.go.
//
// # What returnByteCode does
//
// returnByteCode is the instruction handler for the Return opcode.  It:
//   - Extracts zero, one, or N return values from the callee's stack.
//   - Syncs package-level symbol changes back to the package object.
//   - Restores the caller's context via callFramePop (or stops the run loop
//     when returning from the top-level program).
//   - Optionally signals the debugger when "break on return" was set.
//
// # Setting up a test with a call frame
//
// Most interesting paths require a live call frame on the stack.  Use
// callFramePush to create one, push any return values, then call
// returnByteCode:
//
//	tc := newTestContext(t)
//	tc.ctx.callFramePush("callee", &ByteCode{}, 0, false)
//	// push return value(s) here if needed
//	err := returnByteCode(tc.ctx, operand)
//
// After returnByteCode the context is back in the "caller" state.  Return
// values pushed by callFramePop appear at the top of tc.ctx's stack.
//
// # Resolved issues
//
// RETURN-1 (fixed): the guard now uses c.result (field) so a StackMarker on
// the stack correctly triggers ErrFunctionReturnedVoid.
// RETURN-2 (fixed): c.Pop() errors in the bool branch are now returned
// immediately rather than being silently overwritten by callFramePop.

import (
	"testing"

	"github.com/tucats/ego/errors"
)

// ─── Section 1: void return (nil operand) ────────────────────────────────────

// Test_returnByteCode_VoidReturn_TopLevel verifies that a void return at the
// top level (framePointer == 0, no active call frame) stops the run loop by
// setting c.running = false and returns no error.
func Test_returnByteCode_VoidReturn_TopLevel(t *testing.T) {
	tc := newTestContext(t)

	// Simulate main-program state: fp==0 (no saved call frame).
	tc.ctx.running.Store(true)

	err := returnByteCode(tc.ctx, nil)

	tc.assertNoError(err)

	if tc.ctx.running.Load() {
		t.Error("void top-level return: expected c.running=false, got true")
	}
}

// Test_returnByteCode_VoidReturn_WithFrame verifies that a void return from
// inside a called function pops the call frame, restores the caller context,
// and returns no error.
func Test_returnByteCode_VoidReturn_WithFrame(t *testing.T) {
	tc := newTestContext(t)
	callerBC := tc.ctx.bc

	tc.ctx.callFramePush("callee", &ByteCode{}, 0, false)

	err := returnByteCode(tc.ctx, nil)

	tc.assertNoError(err)

	// The context should be back in the caller's bytecode.
	if tc.ctx.bc != callerBC {
		t.Errorf("void return: bc not restored; got %v, want %v", tc.ctx.bc, callerBC)
	}
}

// Test_returnByteCode_VoidReturn_IntZeroOperand verifies that int(0) is treated
// as a void return — the condition `b > 0` is false, so it falls to the else
// (void) branch.
func Test_returnByteCode_VoidReturn_IntZeroOperand(t *testing.T) {
	tc := newTestContext(t)
	tc.ctx.callFramePush("callee", &ByteCode{}, 0, false)

	err := returnByteCode(tc.ctx, 0)

	tc.assertNoError(err)
}

// Test_returnByteCode_VoidReturn_LeavesCallerStackClean verifies that any extra
// values pushed by the callee are discarded on a void return — the caller's
// stack should only contain what it had before the call.
func Test_returnByteCode_VoidReturn_LeavesCallerStackClean(t *testing.T) {
	tc := newTestContext(t)

	// Caller pushes one item before the call.
	_ = tc.ctx.push("caller-value")

	tc.ctx.callFramePush("callee", &ByteCode{}, 0, false)

	// Callee pushes a stray value that it should NOT return.
	_ = tc.ctx.push("stray-callee-value")

	err := returnByteCode(tc.ctx, nil)

	tc.assertNoError(err)

	// The caller's item should still be on the stack and the callee's
	// stray item should have been discarded.
	tc.assertTopStack("caller-value")
	tc.assertStackEmpty()
}

// ─── Section 2: single-value bool return ────────────────────────────────────

// Test_returnByteCode_BoolReturn_PushesValueOnCallerStack verifies the most
// common return path: Return(true) pops one value from the callee's stack,
// stores it in c.result, and callFramePop re-pushes it onto the caller's stack.
func Test_returnByteCode_BoolReturn_PushesValueOnCallerStack(t *testing.T) {
	tc := newTestContext(t)
	tc.ctx.callFramePush("callee", &ByteCode{}, 0, false)

	// Callee pushes its return value.
	_ = tc.ctx.push("returned-value")

	err := returnByteCode(tc.ctx, true)

	tc.assertNoError(err)

	// Back in caller context: the return value should be on top of the stack.
	tc.assertTopStack("returned-value")
}

// Test_returnByteCode_BoolReturn_PreservesCallerStack verifies that the
// caller's pre-existing stack items are not disturbed when a value is returned.
func Test_returnByteCode_BoolReturn_PreservesCallerStack(t *testing.T) {
	tc := newTestContext(t)

	_ = tc.ctx.push("caller-bottom")

	tc.ctx.callFramePush("callee", &ByteCode{}, 0, false)
	_ = tc.ctx.push(42)

	err := returnByteCode(tc.ctx, true)

	tc.assertNoError(err)

	// Return value is on top, caller's original item is below.
	tc.assertTopStack(42)
	tc.assertTopStack("caller-bottom")
	tc.assertStackEmpty()
}

// Test_returnByteCode_BoolReturn_NilValue verifies that returning an explicit
// nil is valid — nil is a legal Ego return value.
func Test_returnByteCode_BoolReturn_NilValue(t *testing.T) {
	tc := newTestContext(t)
	tc.ctx.callFramePush("callee", &ByteCode{}, 0, false)
	_ = tc.ctx.push(nil)

	err := returnByteCode(tc.ctx, true)

	tc.assertNoError(err)
	tc.assertTopStack(nil)
}

// Test_returnByteCode_BoolReturn_StackMarker_ReturnsVoidError_RETURN1 verifies
// the RETURN-1 fix: when a StackMarker is on the stack and Return(true) is
// executed, the guard `isStackMarker(c.result)` fires and returns
// ErrFunctionReturnedVoid.
//
// Before the fix, the guard used c.Result (the bound method) instead of
// c.result (the field), so isStackMarker always returned false and the marker
// was silently stored as the function's return value.
func Test_returnByteCode_BoolReturn_StackMarker_ReturnsVoidError_RETURN1(t *testing.T) {
	tc := newTestContext(t)
	tc.ctx.callFramePush("callee", &ByteCode{}, 0, false)

	// Push a StackMarker where a real return value is expected.
	_ = tc.ctx.push(NewStackMarker("void-result"))

	err := returnByteCode(tc.ctx, true)

	tc.assertError(err, errors.ErrFunctionReturnedVoid)
}

// Test_returnByteCode_BoolReturn_EmptyStack_RETURN2 verifies the RETURN-2 fix:
// when Return(true) is executed with an empty callee stack, the Pop() error is
// now returned immediately rather than being silently overwritten by
// callFramePop.
//
// Before the fix, the error from Pop() was never checked and was replaced by
// whatever callFramePop returned, causing the original empty-stack error to be
// silently swallowed.
func Test_returnByteCode_BoolReturn_EmptyStack_RETURN2(t *testing.T) {
	tc := newTestContext(t)
	tc.ctx.callFramePush("callee", &ByteCode{}, 0, false)
	// Intentionally do NOT push a return value — empty callee stack.

	err := returnByteCode(tc.ctx, true)

	if err == nil {
		t.Error("RETURN-2 fix: expected error for empty stack at Return(true), got nil")
	}
}

// ─── Section 3: single-value int(1) return (named returns) ───────────────────

// Test_returnByteCode_IntOne_PushesValueOnCallerStack verifies that Return(1)
// behaves identically to Return(true) for normal (non-marker) stack tops.
func Test_returnByteCode_IntOne_PushesValueOnCallerStack(t *testing.T) {
	tc := newTestContext(t)
	tc.ctx.callFramePush("callee", &ByteCode{}, 0, false)
	_ = tc.ctx.push("named-return")

	err := returnByteCode(tc.ctx, 1)

	tc.assertNoError(err)
	tc.assertTopStack("named-return")
}

// Test_returnByteCode_IntOne_DiscardsBelowMarker verifies the named-return
// marker removal.  Named-return functions push a StackMarker below the value
// to delimit the named-return slot.  Return(1) must pop the value, then pop
// (and discard) the marker so callFramePop does not misread it as an extra
// return value.
func Test_returnByteCode_IntOne_DiscardsBelowMarker(t *testing.T) {
	tc := newTestContext(t)
	tc.ctx.callFramePush("callee", &ByteCode{}, 0, false)

	// Named-return layout: marker pushed first, value pushed on top.
	_ = tc.ctx.push(NewStackMarker("named-return"))
	_ = tc.ctx.push("result-val")

	err := returnByteCode(tc.ctx, 1)

	tc.assertNoError(err)

	// After the return, only "result-val" should appear on the caller's stack
	// (the marker must have been discarded).
	tc.assertTopStack("result-val")
	tc.assertStackEmpty()
}

// ─── Section 4: multi-value int(N≥2) return ──────────────────────────────────

// Test_returnByteCode_IntTwo_TransportsValues verifies that Return(2) leaves
// two values on the callee's stack, does NOT pop them itself, and relies on
// callFramePop to transport them to the caller's stack.
func Test_returnByteCode_IntTwo_TransportsValues(t *testing.T) {
	tc := newTestContext(t)
	tc.ctx.callFramePush("callee", &ByteCode{}, 0, false)

	// Push two return values (bottom first, top last).
	_ = tc.ctx.push("first")
	_ = tc.ctx.push("second")

	err := returnByteCode(tc.ctx, 2)

	tc.assertNoError(err)

	// Both values should be on the caller's stack after the return.
	// "second" was pushed last, so it is on top.
	tc.assertTopStack("second")
	tc.assertTopStack("first")
	tc.assertStackEmpty()
}

// Test_returnByteCode_IntThree_TransportsThreeValues verifies the transport
// path for three return values.
func Test_returnByteCode_IntThree_TransportsThreeValues(t *testing.T) {
	tc := newTestContext(t)
	tc.ctx.callFramePush("callee", &ByteCode{}, 0, false)

	_ = tc.ctx.push(10)
	_ = tc.ctx.push(20)
	_ = tc.ctx.push(30)

	err := returnByteCode(tc.ctx, 3)

	tc.assertNoError(err)

	tc.assertTopStack(30)
	tc.assertTopStack(20)
	tc.assertTopStack(10)
	tc.assertStackEmpty()
}

// ─── Section 5: top-level returns (framePointer == 0) ────────────────────────

// Test_returnByteCode_BoolReturn_TopLevel verifies that Return(true) at top
// level pops the return value, stops the run loop, and returns no error.
func Test_returnByteCode_BoolReturn_TopLevel(t *testing.T) {
	tc := newTestContext(t)
	tc.ctx.running.Store(true)
	_ = tc.ctx.push("top-result")

	err := returnByteCode(tc.ctx, true)

	tc.assertNoError(err)

	if tc.ctx.running.Load() {
		t.Error("bool top-level return: expected c.running=false, got true")
	}

	// The result is stored in c.result (callFramePop is not called at top level).
	if tc.ctx.result != "top-result" {
		t.Errorf("bool top-level: c.result = %v, want %q", tc.ctx.result, "top-result")
	}
}

// Test_returnByteCode_IntReturn_TopLevel verifies that Return(1) at top level
// also stops the run loop.
func Test_returnByteCode_IntReturn_TopLevel(t *testing.T) {
	tc := newTestContext(t)
	tc.ctx.running.Store(true)
	_ = tc.ctx.push(999)

	err := returnByteCode(tc.ctx, 1)

	tc.assertNoError(err)

	if tc.ctx.running.Load() {
		t.Error("int(1) top-level return: expected c.running=false, got true")
	}
}

// ─── Section 6: debugger break-on-return ─────────────────────────────────────

// Test_returnByteCode_BreakOnReturn_SignalsDebugger verifies that when the
// saved call frame had breakOnReturn=true (set by the debugger's "step out"
// command), returnByteCode returns ErrSignalDebugger after restoring the
// caller context.
//
// The flag lives in the saved *CallFrame on the stack.  SetBreakOnReturn sets
// it there so callFramePop restores it into c.breakOnReturn before the check.
func Test_returnByteCode_BreakOnReturn_SignalsDebugger(t *testing.T) {
	tc := newTestContext(t)

	tc.ctx.callFramePush("callee", &ByteCode{}, 0, false)

	// Mark the saved frame so the debugger will receive control on return.
	tc.ctx.SetBreakOnReturn()

	err := returnByteCode(tc.ctx, nil)

	tc.assertError(err, errors.ErrSignalDebugger)

	// The flag must be cleared after firing so a subsequent return does not
	// trigger the debugger again.
	if tc.ctx.breakOnReturn {
		t.Error("breakOnReturn was not cleared after signaling the debugger")
	}
}

// Test_returnByteCode_BreakOnReturn_NotSet verifies that when breakOnReturn
// is false (the normal case), no ErrSignalDebugger is returned.
func Test_returnByteCode_BreakOnReturn_NotSet(t *testing.T) {
	tc := newTestContext(t)
	tc.ctx.callFramePush("callee", &ByteCode{}, 0, false)

	err := returnByteCode(tc.ctx, nil)

	// No ErrSignalDebugger expected.
	tc.assertNoError(err)
}

// ─── Section 7: context restoration ─────────────────────────────────────────

// Test_returnByteCode_RestoresPC verifies that callFramePop restores the
// caller's program counter.  The caller had pc=10 when it called; after
// Return the pc should be back at 10.
func Test_returnByteCode_RestoresPC(t *testing.T) {
	tc := newTestContext(t)

	// Set pc=10 in the caller; callFramePush saves this.
	tc.ctx.programCounter = 10

	tc.ctx.callFramePush("callee", &ByteCode{}, 0, false)

	// Callee runs with pc=0; after Return the caller's pc=10 is restored.
	err := returnByteCode(tc.ctx, nil)

	tc.assertNoError(err)

	if tc.ctx.programCounter != 10 {
		t.Errorf("pc after return: got %d, want 10", tc.ctx.programCounter)
	}
}

// Test_returnByteCode_RestoresSymbolTable verifies that callFramePop restores
// the caller's symbol table pointer.
func Test_returnByteCode_RestoresSymbolTable(t *testing.T) {
	tc := newTestContext(t)
	callerSymbols := tc.ctx.symbols

	tc.ctx.callFramePush("callee", &ByteCode{}, 0, false)

	// Inside the call the symbol table is replaced with a new child scope.
	if tc.ctx.symbols == callerSymbols {
		t.Fatal("callFramePush did not replace the symbol table")
	}

	err := returnByteCode(tc.ctx, nil)

	tc.assertNoError(err)

	if tc.ctx.symbols != callerSymbols {
		t.Error("symbol table not restored after Return")
	}
}

// Test_returnByteCode_RestoresBytecode verifies that the caller's bytecode
// stream pointer is restored after Return.
func Test_returnByteCode_RestoresBytecode(t *testing.T) {
	tc := newTestContext(t)
	callerBC := tc.ctx.bc
	calleeBC := &ByteCode{name: "callee"}

	tc.ctx.callFramePush("callee", calleeBC, 0, false)

	if tc.ctx.bc != calleeBC {
		t.Fatal("callFramePush did not activate the callee bytecode")
	}

	err := returnByteCode(tc.ctx, nil)

	tc.assertNoError(err)

	if tc.ctx.bc != callerBC {
		t.Error("bytecode stream not restored after Return")
	}
}
