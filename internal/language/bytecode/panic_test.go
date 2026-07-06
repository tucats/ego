package bytecode

// panic_test.go — unit tests for bytecode/panic.go, focused on the BUG-45 fix
// in unwindPanic's final, unrecovered-panic fallback path.
//
// # Background
//
// unwindPanic is called by the run loop whenever an instruction returns
// errors.ErrPanicActive (emitted by userPanicByteCode for the Ego panic()
// built-in). It walks the call-frame stack looking for a deferred function
// that calls recover(); if one is found, panicActive is cleared and execution
// resumes in the caller. If NO frame recovers the panic, unwindPanic prints a
// "panic: ..." message and call frames, then returns.
//
// # BUG-45
//
// The value returned in that final, unrecovered case used to be
// errors.ErrStop -- the exact same sentinel used when a context finishes
// running normally (falls off the end of its bytecode, or hits an explicit
// Stop). Every caller up the chain treats ErrStop as "nothing went wrong,
// don't propagate this" (see goroutine.go's GoRoutine, and
// invokeDeferredStatements/invokePanicDefers in defer.go), so an unrecovered
// panic was indistinguishable from ordinary success -- most visibly, a
// goroutine that panicked without recovering did not stop the parent
// program, even though real Go terminates the whole process in that case.
//
// The fix makes unwindPanic return errors.ErrPanicUnhandled (wrapping the
// panic message as context) instead of errors.ErrStop for the unrecovered
// case, so callers that specifically check "is this ErrStop?" now correctly
// treat it as a real error. The tests below exercise unwindPanic directly, in
// isolation from any caller; see goroutine_test.go for the GoRoutine-level
// consequence of this change.

import (
	"testing"

	"github.com/tucats/ego/internal/errors"
)

// Test_unwindPanic_UnrecoveredAtTopLevel_ReturnsPanicUnhandledNotStop is the
// core BUG-45 regression test. With no deferred functions at all (so nothing
// can possibly call recover()) and the context already at its outermost frame
// (framePointer == 0), unwindPanic must:
//
//  1. return a non-nil error that IS errors.ErrPanicUnhandled,
//  2. NOT return (or otherwise equal) errors.ErrStop -- the specific defect
//     this test guards against regressing to,
//  3. leave the context's running flag cleared, and
//  4. clear the panic state (panicActive/panicValue) since the panic has now
//     been fully and fatally handled by printing it.
func Test_unwindPanic_UnrecoveredAtTopLevel_ReturnsPanicUnhandledNotStop(t *testing.T) {
	tc := newTestContext(t)

	// Redirect the "panic: ..." message unwindPanic prints so the test
	// output stays clean; we don't assert on its exact text here.
	tc.ctx.EnableConsoleOutput(false)

	tc.ctx.panicActive = true
	tc.ctx.panicValue = "boom"
	tc.ctx.deferStack = nil // nothing can recover this panic
	tc.ctx.running.Store(true)

	err := tc.ctx.unwindPanic()

	if err == nil {
		t.Fatal("expected a non-nil error for an unrecovered panic, got nil")
	}

	if !errors.Equals(err, errors.ErrPanicUnhandled) {
		t.Errorf("got error %v, want errors.ErrPanicUnhandled", err)
	}

	if errors.Equals(err, errors.ErrStop) {
		t.Error("BUG-45 regression: unwindPanic returned errors.ErrStop for an unrecovered panic; " +
			"callers (e.g. GoRoutine) treat ErrStop as \"nothing went wrong\" and will not propagate it")
	}

	if tc.ctx.running.Load() {
		t.Error("expected running to be false after an unrecovered panic")
	}

	if tc.ctx.panicActive {
		t.Error("expected panicActive to be cleared after unwindPanic reports the fatal panic")
	}
}

// Test_unwindPanic_RecoveredAtTopLevel_StillReturnsErrStop verifies the fix
// did NOT touch the other, unrelated place unwindPanic returns errors.ErrStop:
// when a deferred function DOES successfully call recover() and the panicking
// frame was the context's outermost frame (framePointer == 0, so there is no
// caller within this context to resume in), the context has legitimately
// finished running normally -- this is not a fatal, unrecovered panic, and
// must keep returning errors.ErrStop exactly as before.
func Test_unwindPanic_RecoveredAtTopLevel_StillReturnsErrStop(t *testing.T) {
	tc := newTestContext(t)
	tc.ctx.EnableConsoleOutput(false)

	// A deferred closure that calls recover() as a statement, matching what
	// the compiler emits for a bare "recover()" statement (Recover + Drop).
	recoverFn := New("recovering-defer").Literal(true)
	recoverFn.Emit(Recover, nil)
	recoverFn.Emit(Drop, nil)

	tc.ctx.panicActive = true
	tc.ctx.panicValue = "boom"
	tc.ctx.deferStack = []deferStatement{
		{target: recoverFn, args: []any{}, name: "test:recovers"},
	}
	tc.ctx.running.Store(true)

	err := tc.ctx.unwindPanic()

	if !errors.Equals(err, errors.ErrStop) {
		t.Errorf("got error %v, want errors.ErrStop (panic was recovered at the outermost frame)", err)
	}

	if tc.ctx.running.Load() {
		t.Error("expected running to be false once the outermost frame's panic is recovered")
	}
}
