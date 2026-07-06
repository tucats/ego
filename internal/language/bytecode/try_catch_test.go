package bytecode

// try_catch_test.go contains unit tests for the five bytecode handlers and
// one helper that implement Ego's try/catch exception mechanism:
//
//   Source file: bytecode/try.go
//     - tryByteCode       – push a new try-context frame onto the try stack
//     - willCatchByteCode – add one or more catchable error types to the top frame
//     - tryPopByteCode    – remove the top try-context frame (exit the try block)
//     - tryFlushByteCode  – discard the entire try stack (used before @fail, panic, etc.)
//
//   Source file: bytecode/catch.go
//     - handleCatch       – called after every instruction; if an error occurred,
//                           check whether the active try frame catches it
//
// # How try/catch works in Ego bytecode
//
// The compiled output for an Ego `try { ... } catch(e) { ... }` block looks
// roughly like this (pseudocode):
//
//	Push StackMarker("try")   ← marks the bottom of the try scope on the stack
//	Try  <catch-addr>         ← tryByteCode: push tryInfo{addr=catch-addr}
//	... try-block instructions ...
//	TryPop                    ← tryPopByteCode: remove the tryInfo on clean exit
//	Branch <after-catch>
//
//	<catch-addr>:
//	  ... catch-block instructions (can read __error for the caught value) ...
//
//	<after-catch>:
//
// When any instruction inside the try block returns a non-nil error, the run
// loop calls handleCatch(c, err).  handleCatch inspects the tryStack:
//
//  1. If the stack is empty, or the top frame's addr is 0, or the program is
//     no longer running: the error is passed through (not caught).
//  2. If the top frame has a non-empty catches list: only the listed error
//     types are caught; everything else passes through.
//  3. On a successful catch:
//     a. Items are popped from the execution stack until the "try" StackMarker
//        is consumed.  Any *CallFrame encountered during unwinding is popped
//        via callFramePop() so symbol tables and context state are restored.
//     b. c.programCounter is redirected to the catch address.
//     c. The tryInfo.addr is zeroed so the same catch block cannot be
//        re-entered by a secondary error.
//     d. The caught error is stored in the special symbol __error so the
//        catch block can inspect it.
//     e. handleCatch returns nil, signalling to the run loop that the error
//        has been handled.
//
// # How to read these tests
//
// Each test builds a testContext with newTestContext (see testhelpers_test.go)
// and calls the bytecode handlers directly.  The tests verify the state of
// c.tryStack, c.programCounter, the symbol table, and the execution stack
// after each operation.
//
// For handleCatch tests the test must also place a StackMarker("try") on the
// execution stack (simulating what the compiler emits) before triggering the
// error, so the unwind loop has a marker to stop at.
//
// # Bug history
//
// `TRYCATCH-1`  willCatchByteCode did not guard against negative integer
//               operands; catchSets[i-1] with i<0 caused a runtime panic.
//               Fixed: guard extended to `i < 0 || i > len(catchSets)`.
//               See docs/BYTECODE_ISSUES.md — `TRYCATCH-1`.
//
// `BUG-35`      handleCatch only ever inspected c.tryStack[len(c.tryStack)-1].
//               When an error was raised WHILE a catch block was executing (the
//               tryInfo entry for that level had already been zeroed by the
//               first catch), the check `addr > 0` failed and the error was
//               reported as uncaught even though an enclosing try/catch further
//               down the stack was still active and able to catch it. Fixed by
//               searching c.tryStack from the top down for the first frame with
//               a non-zero addr, and by truncating away now-orphaned inner
//               frames (whose TryPop will never execute, since control jumps
//               past them into the enclosing catch). See docs/ISSUES.md — `BUG-35`.

import (
	"testing"

	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
)

// ─────────────────────────────────────────────────────────────────────────────
// Section 1: tryByteCode — push a new try context
// ─────────────────────────────────────────────────────────────────────────────

// Test_tryByteCode_ValidAddress verifies the normal case: a valid integer
// operand pushes a new tryInfo with the given catch address onto c.tryStack.
func Test_tryByteCode_ValidAddress(t *testing.T) {
	tc := newTestContext(t)

	err := tryByteCode(tc.ctx, 42)

	tc.assertNoError(err)

	if len(tc.ctx.tryStack) != 1 {
		t.Fatalf("tryStack length: got %d, want 1", len(tc.ctx.tryStack))
	}

	if tc.ctx.tryStack[0].addr != 42 {
		t.Errorf("tryStack[0].addr: got %d, want 42", tc.ctx.tryStack[0].addr)
	}
}

// Test_tryByteCode_ZeroAddress verifies that addr=0 is accepted and pushed.
// An addr of 0 is special: handleCatch treats it as "catch disabled", so the
// compiler uses it to temporarily suppress catching (e.g. during @fail).
func Test_tryByteCode_ZeroAddress(t *testing.T) {
	tc := newTestContext(t)

	err := tryByteCode(tc.ctx, 0)

	tc.assertNoError(err)

	if len(tc.ctx.tryStack) != 1 || tc.ctx.tryStack[0].addr != 0 {
		t.Errorf("expected tryStack[0].addr=0, got %v", tc.ctx.tryStack)
	}
}

// Test_tryByteCode_NonIntOperand verifies that a non-integer operand
// (e.g. a string) returns a runtime error because data.Int cannot convert it.
func Test_tryByteCode_NonIntOperand(t *testing.T) {
	tc := newTestContext(t)

	err := tryByteCode(tc.ctx, "not-a-number")

	if err == nil {
		t.Fatal("expected error for non-integer operand, got nil")
	}
}

// Test_tryByteCode_NilOperand verifies that a nil operand (data.Int(nil)==0)
// pushes addr=0 without error.
func Test_tryByteCode_NilOperand(t *testing.T) {
	tc := newTestContext(t)

	err := tryByteCode(tc.ctx, nil)

	tc.assertNoError(err)

	if len(tc.ctx.tryStack) != 1 || tc.ctx.tryStack[0].addr != 0 {
		t.Errorf("expected tryStack[0].addr=0 for nil operand")
	}
}

// Test_tryByteCode_CatchesListStartsEmpty verifies that the new tryInfo's
// catches list is initialised as an empty (not nil) slice.  An empty list
// means "catch everything"; a nil list would cause a nil-pointer issue in
// handleCatch's range loop.
func Test_tryByteCode_CatchesListStartsEmpty(t *testing.T) {
	tc := newTestContext(t)

	_ = tryByteCode(tc.ctx, 10)

	if tc.ctx.tryStack[0].catches == nil {
		t.Error("catches slice is nil; expected an empty non-nil slice")
	}

	if len(tc.ctx.tryStack[0].catches) != 0 {
		t.Errorf("catches length: got %d, want 0", len(tc.ctx.tryStack[0].catches))
	}
}

// Test_tryByteCode_StackGrows verifies that successive calls grow the try
// stack in LIFO order so the most-recent frame is always on top.
func Test_tryByteCode_StackGrows(t *testing.T) {
	tc := newTestContext(t)

	_ = tryByteCode(tc.ctx, 10)
	_ = tryByteCode(tc.ctx, 20)
	_ = tryByteCode(tc.ctx, 30)

	if len(tc.ctx.tryStack) != 3 {
		t.Fatalf("tryStack length: got %d, want 3", len(tc.ctx.tryStack))
	}

	// The most recently pushed frame (addr=30) must be on top.
	top := tc.ctx.tryStack[len(tc.ctx.tryStack)-1]
	if top.addr != 30 {
		t.Errorf("top frame addr: got %d, want 30", top.addr)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 2: willCatchByteCode — register catchable error types
// ─────────────────────────────────────────────────────────────────────────────
//
// willCatchByteCode appends error types to the top tryInfo so that handleCatch
// knows which errors to intercept.  An empty catches list means catch-all;
// calling willCatchByteCode with specific errors creates a selective filter.

// Test_willCatchByteCode_EmptyTryStack verifies that calling willCatch before
// any try block is open returns ErrTryCatchMismatch.
func Test_willCatchByteCode_EmptyTryStack(t *testing.T) {
	tc := newTestContext(t)

	err := willCatchByteCode(tc.ctx, errors.ErrAssert)

	tc.assertError(err, errors.ErrTryCatchMismatch)
}

// Test_willCatchByteCode_AllErrorsCatchSet verifies that passing int(0) clears
// the catches list, making the block catch everything (the default when no
// WillCatch instruction is emitted at all).
func Test_willCatchByteCode_AllErrorsCatchSet(t *testing.T) {
	tc := newTestContext(t)

	// First add a specific catch so there is something to clear.
	_ = tryByteCode(tc.ctx, 5)
	_ = willCatchByteCode(tc.ctx, errors.ErrAssert)

	if len(tc.ctx.tryStack[0].catches) == 0 {
		t.Fatal("precondition: expected a catch entry to be present before clearing")
	}

	// Now clear with int(0).
	err := willCatchByteCode(tc.ctx, 0)

	tc.assertNoError(err)

	if len(tc.ctx.tryStack[0].catches) != 0 {
		t.Errorf("after AllErrorsCatchSet: expected catches to be empty, got %d entries",
			len(tc.ctx.tryStack[0].catches))
	}
}

// Test_willCatchByteCode_OptionalCatchSet verifies that passing
// OptionalCatchSet (1) appends the predefined list of errors that the optional
// operator (?expr) is permitted to silence.
func Test_willCatchByteCode_OptionalCatchSet(t *testing.T) {
	tc := newTestContext(t)
	_ = tryByteCode(tc.ctx, 5)

	err := willCatchByteCode(tc.ctx, OptionalCatchSet)

	tc.assertNoError(err)

	// The OptionalCatchSet has multiple entries (ErrUnknownMember, ErrInvalidType, …).
	if len(tc.ctx.tryStack[0].catches) == 0 {
		t.Error("expected OptionalCatchSet entries to be appended")
	}
}

// Test_willCatchByteCode_OutOfRangeCatchSet verifies that requesting a catch
// set index larger than len(catchSets) returns ErrInternalCompiler.  This
// guards against compiler bugs that emit an invalid set index.
func Test_willCatchByteCode_OutOfRangeCatchSet(t *testing.T) {
	tc := newTestContext(t)
	_ = tryByteCode(tc.ctx, 5)

	// len(catchSets) == 1; requesting index 99 must fail.
	err := willCatchByteCode(tc.ctx, 99)

	tc.assertError(err, errors.ErrInternalCompiler)
}

// Test_willCatchByteCode_ErrorsErrorOperand verifies that passing a
// *errors.Error value registers it directly in the catches list.
func Test_willCatchByteCode_ErrorsErrorOperand(t *testing.T) {
	tc := newTestContext(t)
	_ = tryByteCode(tc.ctx, 5)

	err := willCatchByteCode(tc.ctx, errors.ErrAssert)

	tc.assertNoError(err)

	if len(tc.ctx.tryStack[0].catches) != 1 {
		t.Errorf("catches length: got %d, want 1", len(tc.ctx.tryStack[0].catches))
	}
}

// Test_willCatchByteCode_PlainErrorOperand verifies that a standard Go error
// (not *errors.Error) is wrapped with errors.New and appended.
func Test_willCatchByteCode_PlainErrorOperand(t *testing.T) {
	tc := newTestContext(t)
	_ = tryByteCode(tc.ctx, 5)

	// Use a stdlib fmt.Errorf-style error.
	plainErr := errors.New(errors.ErrAssert) // produces a plain error indirectly

	err := willCatchByteCode(tc.ctx, error(plainErr))

	tc.assertNoError(err)

	if len(tc.ctx.tryStack[0].catches) != 1 {
		t.Errorf("catches length: got %d, want 1", len(tc.ctx.tryStack[0].catches))
	}
}

// Test_willCatchByteCode_StringOperand verifies that a string operand is
// converted to an error via errors.Message and appended to the catches list.
func Test_willCatchByteCode_StringOperand(t *testing.T) {
	tc := newTestContext(t)
	_ = tryByteCode(tc.ctx, 5)

	err := willCatchByteCode(tc.ctx, "assert")

	tc.assertNoError(err)

	if len(tc.ctx.tryStack[0].catches) != 1 {
		t.Errorf("catches length: got %d, want 1", len(tc.ctx.tryStack[0].catches))
	}
}

// Test_willCatchByteCode_InvalidTypeOperand verifies that an unsupported
// operand type (e.g. float64) returns ErrInvalidType.
func Test_willCatchByteCode_InvalidTypeOperand(t *testing.T) {
	tc := newTestContext(t)
	_ = tryByteCode(tc.ctx, 5)

	err := willCatchByteCode(tc.ctx, 3.14)

	tc.assertError(err, errors.ErrInvalidType)
}

// Test_willCatchByteCode_MultipleErrorsAccumulate verifies that calling
// willCatch multiple times appends to the same catches list rather than
// replacing it.
func Test_willCatchByteCode_MultipleErrorsAccumulate(t *testing.T) {
	tc := newTestContext(t)
	_ = tryByteCode(tc.ctx, 5)

	_ = willCatchByteCode(tc.ctx, errors.ErrAssert)
	_ = willCatchByteCode(tc.ctx, errors.ErrInvalidType)

	if len(tc.ctx.tryStack[0].catches) != 2 {
		t.Errorf("catches length: got %d, want 2", len(tc.ctx.tryStack[0].catches))
	}
}

// Test_willCatchByteCode_NegativeInt_ReturnsError verifies the TRYCATCH-1
// fix: a negative integer operand is now rejected with ErrInternalCompiler
// instead of causing a runtime panic.
//
// Before the fix the guard was `i > len(catchSets)`, which passed for
// negative values.  The code then reached `catchSets[i-1]` with a negative
// index (e.g. -2 for i=-1), causing an unrecoverable index-out-of-range panic.
// The fix extends the guard to `i < 0 || i > len(catchSets)`.
func Test_willCatchByteCode_NegativeInt_ReturnsError(t *testing.T) {
	tc := newTestContext(t)
	_ = tryByteCode(tc.ctx, 5)

	panicked := false

	var callErr error

	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()

		callErr = willCatchByteCode(tc.ctx, -1)
	}()

	// After the TRYCATCH-1 fix the call must not panic.
	if panicked {
		t.Fatal("willCatchByteCode panicked on -1: TRYCATCH-1 fix was not applied")
	}

	// A clean ErrInternalCompiler must be returned instead.
	tc.assertError(callErr, errors.ErrInternalCompiler)
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 3: tryPopByteCode — remove the top try context
// ─────────────────────────────────────────────────────────────────────────────

// Test_tryPopByteCode_EmptyStack verifies that popping when no try block is
// open returns ErrTryCatchMismatch.
func Test_tryPopByteCode_EmptyStack(t *testing.T) {
	tc := newTestContext(t)

	err := tryPopByteCode(tc.ctx, nil)

	tc.assertError(err, errors.ErrTryCatchMismatch)
}

// Test_tryPopByteCode_SingleEntry verifies that removing the only try frame
// leaves the try stack completely empty.
func Test_tryPopByteCode_SingleEntry(t *testing.T) {
	tc := newTestContext(t)
	_ = tryByteCode(tc.ctx, 10)

	err := tryPopByteCode(tc.ctx, nil)

	tc.assertNoError(err)

	if len(tc.ctx.tryStack) != 0 {
		t.Errorf("tryStack length after pop: got %d, want 0", len(tc.ctx.tryStack))
	}
}

// Test_tryPopByteCode_MultipleEntries verifies that only the top frame is
// removed, leaving the frames below intact.
func Test_tryPopByteCode_MultipleEntries(t *testing.T) {
	tc := newTestContext(t)
	_ = tryByteCode(tc.ctx, 10) // bottom
	_ = tryByteCode(tc.ctx, 20) // top

	err := tryPopByteCode(tc.ctx, nil)

	tc.assertNoError(err)

	if len(tc.ctx.tryStack) != 1 {
		t.Fatalf("tryStack length after pop: got %d, want 1", len(tc.ctx.tryStack))
	}

	if tc.ctx.tryStack[0].addr != 10 {
		t.Errorf("remaining frame addr: got %d, want 10", tc.ctx.tryStack[0].addr)
	}
}

// Test_tryPopByteCode_ClearsErrorVariable verifies that tryPop deletes the
// __error symbol from the active symbol table, preventing the catch-block's
// error value from leaking into code after the try/catch.
func Test_tryPopByteCode_ClearsErrorVariable(t *testing.T) {
	tc := newTestContext(t)
	_ = tryByteCode(tc.ctx, 10)

	// Simulate __error being set by a previous catch.
	tc.ctx.symbols.SetAlways(defs.ErrorVariable, errors.ErrAssert)

	err := tryPopByteCode(tc.ctx, nil)

	tc.assertNoError(err)

	// __error must no longer be accessible.
	if _, found := tc.ctx.symbols.Get(defs.ErrorVariable); found {
		t.Error("__error still present in symbol table after tryPop")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 4: tryFlushByteCode — discard the entire try stack
// ─────────────────────────────────────────────────────────────────────────────

// Test_tryFlushByteCode_EmptyStack verifies that flushing an already-empty
// stack is a no-op (no error, stack stays empty).
func Test_tryFlushByteCode_EmptyStack(t *testing.T) {
	tc := newTestContext(t)

	err := tryFlushByteCode(tc.ctx, nil)

	tc.assertNoError(err)

	if len(tc.ctx.tryStack) != 0 {
		t.Errorf("tryStack not empty after flush: len=%d", len(tc.ctx.tryStack))
	}
}

// Test_tryFlushByteCode_MultipleEntries verifies that all frames are removed
// in a single call, regardless of how many are present.
func Test_tryFlushByteCode_MultipleEntries(t *testing.T) {
	tc := newTestContext(t)
	_ = tryByteCode(tc.ctx, 10)
	_ = tryByteCode(tc.ctx, 20)
	_ = tryByteCode(tc.ctx, 30)

	err := tryFlushByteCode(tc.ctx, nil)

	tc.assertNoError(err)

	if len(tc.ctx.tryStack) != 0 {
		t.Errorf("tryStack not empty after flush: len=%d", len(tc.ctx.tryStack))
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 5: handleCatch — the catch decision engine
// ─────────────────────────────────────────────────────────────────────────────
//
// handleCatch is called by the run loop after every instruction that returns
// a non-nil error.  The tests below set up each scenario by directly
// manipulating c.tryStack and the execution stack, then call handleCatch and
// verify the outcome.

// Test_handleCatch_NilError verifies that a nil error is a no-op: nothing
// changes and nil is returned.
func Test_handleCatch_NilError(t *testing.T) {
	tc := newTestContext(t)

	err := handleCatch(tc.ctx, nil)

	tc.assertNoError(err)
}

// Test_handleCatch_ErrStop verifies that ErrStop (which terminates the run
// loop) is treated identically to nil: it is not an error condition that
// should be caught, so handleCatch returns nil.
func Test_handleCatch_ErrStop(t *testing.T) {
	tc := newTestContext(t)

	err := handleCatch(tc.ctx, errors.ErrStop)

	tc.assertNoError(err)
}

// Test_handleCatch_ErrPanicActive verifies that an in-progress user panic
// (set by the Ego panic() built-in) bypasses all try/catch blocks.  The panic
// is expected to propagate up to the deferred-recover mechanism.
func Test_handleCatch_ErrPanicActive(t *testing.T) {
	tc := newTestContext(t)

	// Even if a try block is open, ErrPanicActive must not be caught.
	_ = tryByteCode(tc.ctx, 99)

	err := handleCatch(tc.ctx, errors.ErrPanicActive)

	if err == nil {
		t.Error("ErrPanicActive should not be caught — expected non-nil error, got nil")
	}
}

// Test_handleCatch_EmptyTryStack verifies that an error passes through when
// there is no active try block.
func Test_handleCatch_EmptyTryStack(t *testing.T) {
	tc := newTestContext(t)

	err := handleCatch(tc.ctx, errors.ErrAssert)

	if err == nil {
		t.Error("expected error to pass through with empty tryStack, got nil")
	}
}

// Test_handleCatch_AddrZeroNotCaught verifies that a tryInfo whose addr is 0
// does NOT redirect the error.  addr==0 means the catch block has already run
// or was intentionally disabled; the error must propagate.
func Test_handleCatch_AddrZeroNotCaught(t *testing.T) {
	tc := newTestContext(t)
	tc.ctx.tryStack = []tryInfo{{addr: 0, catches: []error{}}}

	err := handleCatch(tc.ctx, errors.ErrAssert)

	if err == nil {
		t.Error("addr==0: expected error to pass through, got nil")
	}
}

// Test_handleCatch_RunningFalseNotCaught verifies that when the context is
// no longer running (a fatal error turned off the run flag), errors are not
// caught even if a valid try block is present.
func Test_handleCatch_RunningFalseNotCaught(t *testing.T) {
	tc := newTestContext(t)
	tc.ctx.running.Store(false) // simulate a fatal / stopped context

	// Set up a try frame with a valid catch address.
	tc.ctx.tryStack = []tryInfo{{addr: 50, catches: []error{}}}
	// Place the "try" marker the unwind loop would need.
	_ = tc.ctx.push(NewStackMarker("try"))

	err := handleCatch(tc.ctx, errors.ErrAssert)

	if err == nil {
		t.Error("running=false: expected error to pass through, got nil")
	}
}

// Test_handleCatch_CatchAll_RedirectsPC verifies the core happy path:
// an empty catches list means "catch everything", so the error is absorbed,
// the program counter is redirected to the catch address, and nil is returned.
func Test_handleCatch_CatchAll_RedirectsPC(t *testing.T) {
	tc := newTestContext(t)
	tc.ctx.running.Store(true)

	// Push the "try" marker that the compiler places at the start of every try
	// block.  The unwind loop inside handleCatch pops items until it finds this.
	_ = tc.ctx.push(NewStackMarker("try"))

	const catchAddr = 77
	tc.ctx.tryStack = []tryInfo{{addr: catchAddr, catches: []error{}}}

	err := handleCatch(tc.ctx, errors.ErrAssert)

	tc.assertNoError(err)

	tc.assertProgramCounter(catchAddr)
}

// Test_handleCatch_CatchAll_StackUnwound verifies that items pushed ABOVE the
// "try" marker are removed during the catch.  Only the marker itself is
// consumed; nothing below the marker should be touched.
func Test_handleCatch_CatchAll_StackUnwound(t *testing.T) {
	tc := newTestContext(t)
	tc.ctx.running.Store(true)

	// Simulate the stack state midway through a try block:
	//   "sentinel" (below the try scope, must survive)
	//   StackMarker("try")
	//   "item-a" (accumulated inside the try block, must be removed)
	//   "item-b" (same)
	_ = tc.ctx.push("sentinel")
	_ = tc.ctx.push(NewStackMarker("try"))
	_ = tc.ctx.push("item-a")
	_ = tc.ctx.push("item-b")

	tc.ctx.tryStack = []tryInfo{{addr: 10, catches: []error{}}}

	err := handleCatch(tc.ctx, errors.ErrAssert)

	tc.assertNoError(err)

	// After the catch, only "sentinel" should remain.
	tc.assertTopStack("sentinel")
	tc.assertStackEmpty()
}

// Test_handleCatch_CatchAll_SetsErrorVariable verifies that the caught error
// is stored in the __error symbol so the catch block can inspect it.
func Test_handleCatch_CatchAll_SetsErrorVariable(t *testing.T) {
	tc := newTestContext(t)
	tc.ctx.running.Store(true)

	_ = tc.ctx.push(NewStackMarker("try"))
	tc.ctx.tryStack = []tryInfo{{addr: 10, catches: []error{}}}

	_ = handleCatch(tc.ctx, errors.ErrAssert)

	v, found := tc.ctx.symbols.Get(defs.ErrorVariable)
	if !found {
		t.Fatal("__error not set in symbol table after catch")
	}

	if v == nil {
		t.Error("__error is nil; expected the caught error")
	}
}

// Test_handleCatch_CatchAll_ZerosAddr verifies that after a successful catch,
// the tryInfo.addr is set to 0.  This prevents a second error inside the catch
// block from being re-redirected to the same catch address (infinite loop).
func Test_handleCatch_CatchAll_ZerosAddr(t *testing.T) {
	tc := newTestContext(t)
	tc.ctx.running.Store(true)

	_ = tc.ctx.push(NewStackMarker("try"))
	tc.ctx.tryStack = []tryInfo{{addr: 55, catches: []error{}}}

	_ = handleCatch(tc.ctx, errors.ErrAssert)

	if tc.ctx.tryStack[0].addr != 0 {
		t.Errorf("tryInfo.addr after catch: got %d, want 0", tc.ctx.tryStack[0].addr)
	}
}

// Test_handleCatch_SelectiveCatch_MatchingError verifies that when catches has
// a specific error and the incoming error matches it, the error IS caught.
func Test_handleCatch_SelectiveCatch_MatchingError(t *testing.T) {
	tc := newTestContext(t)
	tc.ctx.running.Store(true)

	_ = tc.ctx.push(NewStackMarker("try"))
	tc.ctx.tryStack = []tryInfo{{
		addr:    20,
		catches: []error{errors.ErrAssert}, // only catch ErrAssert
	}}

	// Pass ErrAssert — it is in the catch list, so it should be caught.
	err := handleCatch(tc.ctx, errors.ErrAssert)

	tc.assertNoError(err)
	tc.assertProgramCounter(20)
}

// Test_handleCatch_SelectiveCatch_NonMatchingError verifies that an error NOT
// in the selective catches list is NOT caught — it passes through so the caller
// (the run loop or an outer try block) can handle it.
func Test_handleCatch_SelectiveCatch_NonMatchingError(t *testing.T) {
	tc := newTestContext(t)
	tc.ctx.running.Store(true)

	_ = tc.ctx.push(NewStackMarker("try"))
	tc.ctx.tryStack = []tryInfo{{
		addr:    20,
		catches: []error{errors.ErrAssert}, // only catch ErrAssert
	}}

	// Pass ErrInvalidType — not in the catch list.
	err := handleCatch(tc.ctx, errors.ErrInvalidType)

	if err == nil {
		t.Error("non-matching error should not be caught — expected non-nil error, got nil")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 6: Integration — full try / catch / pop sequences
// ─────────────────────────────────────────────────────────────────────────────
//
// These tests exercise the bytecodes together as the compiler would emit them,
// verifying the complete lifecycle of a try/catch block.

// Test_Integration_NormalExecution simulates a try block that completes
// without any error: the try is opened, some work is done, and then tryPop
// is called.  The tryStack should be empty afterwards and __error should not
// exist.
func Test_Integration_NormalExecution(t *testing.T) {
	tc := newTestContext(t)

	// Open the try block.
	if err := tryByteCode(tc.ctx, 99); err != nil {
		t.Fatalf("tryByteCode: %v", err)
	}

	// Simulate successful work inside the try block (no error).
	_ = tc.ctx.push("result-value")

	// Close the try block cleanly.
	if err := tryPopByteCode(tc.ctx, nil); err != nil {
		t.Fatalf("tryPopByteCode: %v", err)
	}

	if len(tc.ctx.tryStack) != 0 {
		t.Errorf("tryStack not empty after clean exit: len=%d", len(tc.ctx.tryStack))
	}

	// __error must not exist after a clean exit.
	if _, found := tc.ctx.symbols.Get(defs.ErrorVariable); found {
		t.Error("__error unexpectedly present after clean try block exit")
	}
}

// Test_Integration_ErrorCaught simulates an error inside a try block, the
// catch redirect, and the final tryPop to clean up.
func Test_Integration_ErrorCaught(t *testing.T) {
	tc := newTestContext(t)
	tc.ctx.running.Store(true)

	// Compiler emits: Push StackMarker("try"), then Try <catch-addr>.
	_ = tc.ctx.push(NewStackMarker("try"))

	const catchAddr = 100
	if err := tryByteCode(tc.ctx, catchAddr); err != nil {
		t.Fatalf("tryByteCode: %v", err)
	}

	// Push some items that the try-block "executed" before the error.
	_ = tc.ctx.push("try-item-1")
	_ = tc.ctx.push("try-item-2")

	// Simulate the error that the run loop detected.
	if err := handleCatch(tc.ctx, errors.ErrAssert); err != nil {
		t.Fatalf("handleCatch returned unexpected error: %v", err)
	}

	// PC must now point at the catch block.
	tc.assertProgramCounter(catchAddr)

	// __error must be set for the catch block to read.
	if _, found := tc.ctx.symbols.Get(defs.ErrorVariable); !found {
		t.Error("__error not set after catch")
	}

	// Simulate the catch block running: the compiler ends with TryPop.
	if err := tryPopByteCode(tc.ctx, nil); err != nil {
		t.Fatalf("tryPopByteCode: %v", err)
	}

	// Everything cleaned up.
	if len(tc.ctx.tryStack) != 0 {
		t.Errorf("tryStack not empty after tryPop: len=%d", len(tc.ctx.tryStack))
	}

	if _, found := tc.ctx.symbols.Get(defs.ErrorVariable); found {
		t.Error("__error still present after tryPop")
	}
}

// Test_Integration_NestedTryBlocks simulates two nested try blocks.  An error
// is caught by the INNER block; after the inner block exits, the outer block
// is still active and able to catch further errors.
func Test_Integration_NestedTryBlocks(t *testing.T) {
	tc := newTestContext(t)
	tc.ctx.running.Store(true)

	// Open outer try.
	_ = tc.ctx.push(NewStackMarker("try")) // outer "try" marker
	_ = tryByteCode(tc.ctx, 200)           // outer catch addr

	// Open inner try.
	_ = tc.ctx.push(NewStackMarker("try")) // inner "try" marker
	_ = tryByteCode(tc.ctx, 100)           // inner catch addr

	// Error occurs: the inner block (top of tryStack) should catch it.
	if err := handleCatch(tc.ctx, errors.ErrAssert); err != nil {
		t.Fatalf("handleCatch: expected nil (inner catch), got %v", err)
	}

	tc.assertProgramCounter(100) // inner catch address

	// Pop the inner try.
	if err := tryPopByteCode(tc.ctx, nil); err != nil {
		t.Fatalf("inner tryPop: %v", err)
	}

	// The outer try frame must still be active.
	if len(tc.ctx.tryStack) != 1 {
		t.Errorf("tryStack length after inner pop: got %d, want 1", len(tc.ctx.tryStack))
	}

	if tc.ctx.tryStack[0].addr != 200 {
		t.Errorf("outer frame addr: got %d, want 200", tc.ctx.tryStack[0].addr)
	}
}

// Test_Integration_FlushThenNoMoreCatch verifies that tryFlush removes all
// pending catch frames so that a subsequent error is not caught.
func Test_Integration_FlushThenNoMoreCatch(t *testing.T) {
	tc := newTestContext(t)
	tc.ctx.running.Store(true)

	_ = tc.ctx.push(NewStackMarker("try"))
	_ = tryByteCode(tc.ctx, 50)

	// Flush all try frames (e.g., before a @fail or runtime panic).
	if err := tryFlushByteCode(tc.ctx, nil); err != nil {
		t.Fatalf("tryFlush: %v", err)
	}

	// Now an error should NOT be caught.
	err := handleCatch(tc.ctx, errors.ErrAssert)

	if err == nil {
		t.Error("expected error to pass through after flush, got nil")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 7: BUG-35 — an error raised inside a catch block must escape to the
// nearest enclosing try/catch instead of being reported as uncaught.
// ─────────────────────────────────────────────────────────────────────────────
//
// Before the fix, handleCatch only ever inspected c.tryStack[len(c.tryStack)-1].
// Once a frame's catch block had been entered, handleCatch zeroed that frame's
// addr (so the same catch could not be re-entered), but left it on top of the
// stack until its TryPop instruction ran. Any error raised while that catch
// block was still executing therefore found addr==0 on top and was reported
// as uncaught, even though an outer try/catch further down c.tryStack (with a
// live, non-zero addr) was available and should have caught it — exactly like
// Go's defer/recover: a panic raised while a recover handler is running still
// unwinds into the next outer recover.
//
// These tests reproduce the nested-try scenario from docs/ISSUES.md#BUG-35:
//
//	try {                  // outer: catch addr 200
//	    try {               // inner: catch addr 100
//	        5 / 0           // err1 — should be caught by inner
//	    } catch {
//	        10 / 0          // err2 — raised INSIDE inner's catch; must escape
//	    }                   //        to the outer try, not be lost
//	} catch (outer) {
//	    ...
//	}

// Test_handleCatch_BUG35_ErrorDuringCatchBlockEscalatesToOuterTry reproduces
// the full two-level scenario: the inner try catches err1 normally, and then
// while its catch block is executing, err2 is raised. err2 must be redirected
// to the outer try's catch address (200), not reported as uncaught.
func Test_handleCatch_BUG35_ErrorDuringCatchBlockEscalatesToOuterTry(t *testing.T) {
	tc := newTestContext(t)
	tc.ctx.running.Store(true)

	// Simulate the compiled layout of the nested try/catch:
	//   Push StackMarker("try")   ← outer
	//   Try 200                   ← outer catch address
	//   Push StackMarker("try")   ← inner
	//   Try 100                   ← inner catch address
	_ = tc.ctx.push(NewStackMarker("try")) // outer marker
	_ = tryByteCode(tc.ctx, 200)           // outer catch addr

	_ = tc.ctx.push(NewStackMarker("try")) // inner marker
	_ = tryByteCode(tc.ctx, 100)           // inner catch addr

	// err1: division by zero inside the inner try block. The inner frame
	// (top of tryStack, addr=100) must catch it.
	err1 := handleCatch(tc.ctx, errors.ErrDivisionByZero)
	tc.assertNoError(err1)
	tc.assertProgramCounter(100)

	// After catching err1, the inner frame's addr is zeroed (spent) but it is
	// still on the stack awaiting its TryPop — the outer frame is untouched.
	if len(tc.ctx.tryStack) != 2 {
		t.Fatalf("tryStack length after inner catch: got %d, want 2", len(tc.ctx.tryStack))
	}

	if tc.ctx.tryStack[1].addr != 0 {
		t.Errorf("inner frame addr after catch: got %d, want 0", tc.ctx.tryStack[1].addr)
	}

	if tc.ctx.tryStack[0].addr != 200 {
		t.Errorf("outer frame addr must be untouched: got %d, want 200", tc.ctx.tryStack[0].addr)
	}

	// err2: raised while the inner catch block is executing (e.g. `10 / 0`
	// inside `catch { ... }`). This is the BUG-35 regression case: the fix
	// must find the outer frame (addr=200) since the inner one is spent.
	err2 := handleCatch(tc.ctx, errors.ErrDivisionByZero)
	tc.assertNoError(err2)
	tc.assertProgramCounter(200)

	// The spent inner frame must be discarded (its TryPop will never run,
	// since control jumped past it directly into the outer catch block), and
	// the outer frame is now itself marked spent (addr==0) pending its own
	// TryPop once its catch block finishes.
	if len(tc.ctx.tryStack) != 1 {
		t.Fatalf("tryStack length after outer catch: got %d, want 1 (inner frame must be discarded)", len(tc.ctx.tryStack))
	}

	if tc.ctx.tryStack[0].addr != 0 {
		t.Errorf("outer frame addr after catching err2: got %d, want 0", tc.ctx.tryStack[0].addr)
	}
}

// Test_handleCatch_BUG35_NoEnclosingTry_ErrorPassesThrough verifies that when
// there is no enclosing try/catch left to escalate to (a single try/catch
// whose catch block itself raises an error), the error correctly passes
// through as uncaught rather than looping back into the same spent frame.
func Test_handleCatch_BUG35_NoEnclosingTry_ErrorPassesThrough(t *testing.T) {
	tc := newTestContext(t)
	tc.ctx.running.Store(true)

	_ = tc.ctx.push(NewStackMarker("try"))
	_ = tryByteCode(tc.ctx, 50)

	// First error: caught normally.
	err1 := handleCatch(tc.ctx, errors.ErrDivisionByZero)
	tc.assertNoError(err1)
	tc.assertProgramCounter(50)

	// Second error, raised inside the (only) catch block: there is no
	// enclosing try, so it must pass through uncaught.
	err2 := handleCatch(tc.ctx, errors.ErrDivisionByZero)
	if err2 == nil {
		t.Error("expected error raised inside catch block to pass through when no enclosing try exists, got nil")
	}
}

// Test_handleCatch_BUG35_DiscardsMultipleOrphanedFrames verifies the general
// case: if several inner try/catch levels are already spent (addr==0) when a
// new error arrives, handleCatch must skip all of them, catch at the first
// live (addr>0) frame found further down the stack, and discard every
// orphaned frame above it in one step (not just the immediate parent).
func Test_handleCatch_BUG35_DiscardsMultipleOrphanedFrames(t *testing.T) {
	tc := newTestContext(t)
	tc.ctx.running.Store(true)

	_ = tc.ctx.push(NewStackMarker("try")) // stack marker for the live (outer) frame

	tc.ctx.tryStack = []tryInfo{
		{addr: 300, catches: []error{}}, // outer: still live
		{addr: 0, catches: []error{}},   // middle: already spent
		{addr: 0, catches: []error{}},   // inner: already spent
	}

	err := handleCatch(tc.ctx, errors.ErrDivisionByZero)
	tc.assertNoError(err)
	tc.assertProgramCounter(300)

	if len(tc.ctx.tryStack) != 1 {
		t.Fatalf("tryStack length: got %d, want 1 (both orphaned frames discarded)", len(tc.ctx.tryStack))
	}

	if tc.ctx.tryStack[0].addr != 0 {
		t.Errorf("surviving (outer) frame addr: got %d, want 0", tc.ctx.tryStack[0].addr)
	}
}

// Test_handleCatch_BUG35_SelectiveCatchMismatchEscalatesToOuterTry covers the
// second, broader variant of the same defect: a LIVE inner frame (addr > 0)
// with a selective catches list (e.g. the internal try/catch compiled for the
// `?` optional operator, or a `catch` naming specific error types) that does
// NOT include the error that occurred. Before this fix, handleCatch only ever
// examined the innermost frame and, finding no match, reported the error as
// uncaught — even when an enclosing catch-all try/catch further down the
// stack was available and should have handled it. The fix must also verify
// that the inner frame's still-unconsumed "try" StackMarker is correctly
// skipped so the unwind stops at the OUTER frame's marker, not the inner one.
func Test_handleCatch_BUG35_SelectiveCatchMismatchEscalatesToOuterTry(t *testing.T) {
	tc := newTestContext(t)
	tc.ctx.running.Store(true)

	// Outer: catch-all (empty catches list), catch address 200.
	_ = tc.ctx.push(NewStackMarker("try")) // outer marker (bottom)
	_ = tryByteCode(tc.ctx, 200)

	// Inner: selective catch that only catches ErrAssert (e.g. compiled for
	// `?expr`), catch address 100. Its marker is still on the stack because
	// its try block never actually reaches its own catch.
	_ = tc.ctx.push(NewStackMarker("try")) // inner marker (top)
	_ = tryByteCode(tc.ctx, 100)
	_ = willCatchByteCode(tc.ctx, errors.ErrAssert)

	// Push a couple of items to simulate partial execution of the inner try
	// block before the error — these must be discarded during unwind.
	_ = tc.ctx.push("inner-item-1")
	_ = tc.ctx.push("inner-item-2")

	// Raise ErrDivisionByZero: NOT in the inner frame's selective catches
	// list, so the inner frame must be bypassed in favor of the outer,
	// catch-all frame.
	err := handleCatch(tc.ctx, errors.ErrDivisionByZero)
	tc.assertNoError(err)
	tc.assertProgramCounter(200)

	// The bypassed inner frame is discarded entirely (it was never entered,
	// so there is nothing to leave behind for a later TryPop).
	if len(tc.ctx.tryStack) != 1 {
		t.Fatalf("tryStack length: got %d, want 1 (bypassed inner frame discarded)", len(tc.ctx.tryStack))
	}

	if tc.ctx.tryStack[0].addr != 0 {
		t.Errorf("outer frame addr after catch: got %d, want 0", tc.ctx.tryStack[0].addr)
	}

	// Both the inner frame's marker and the outer frame's marker (plus the
	// two simulated items) must have been consumed; the execution stack
	// should now be empty.
	tc.assertStackEmpty()
}
