package bytecode

// goroutine_test.go — unit tests for bytecode/goroutine.go's GoRoutine
// function, focused on the BUG-45 fix: an unrecovered panic() inside a
// goroutine must stop the parent context, exactly like an ordinary
// propagated runtime error does, instead of being treated as if the
// goroutine simply finished its work normally.
//
// GoRoutine is normally launched via "go GoRoutine(fx, parentCtx, args)" from
// goByteCode (the "go" statement). These tests call it directly and
// synchronously instead, so the parent context's state can be inspected
// deterministically immediately after it returns. Calling it directly
// bypasses goByteCode's goRoutineCompletion.Add(1), so each test must call
// that itself before invoking GoRoutine, to keep the package-level
// goRoutineCompletion WaitGroup balanced for any other test that uses it.

import (
	"testing"

	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/data"
)

// Test_GoRoutine_UnrecoveredPanic_StopsParentContext is the GoRoutine-level
// regression test for BUG-45. fx's body is a single UserPanic instruction --
// the same opcode the compiler emits for the Ego panic() built-in -- with
// nothing to recover it, so it reaches unwindPanic's fatal, top-of-stack
// fallback. Before the fix, that fallback returned errors.ErrStop, which
// GoRoutine's "if err != nil && !err.Is(errors.ErrStop)" check specifically
// excludes from stopping the parent -- so the panic was silently swallowed
// and the parent kept running. After the fix, the returned error is
// errors.ErrPanicUnhandled, which does not equal ErrStop, so it must now
// stop the parent context and record the error.
func Test_GoRoutine_UnrecoveredPanic_StopsParentContext(t *testing.T) {
	parent := newTestContext(t)
	parent.ctx.EnableConsoleOutput(false)
	parent.ctx.running.Store(true)

	fx := New("panicking-goroutine").Literal(true)
	fx.Emit(UserPanic, "goroutine panic")

	goRoutineCompletion.Add(1)
	GoRoutine(fx, parent.ctx, data.NewList())

	if parent.ctx.running.Load() {
		t.Error("BUG-45 regression: parent context is still marked running after an unrecovered goroutine panic")
	}

	if parent.ctx.goErr == nil {
		t.Fatal("expected parent.goErr to be set after an unrecovered goroutine panic, got nil")
	}

	if !errors.Equals(parent.ctx.goErr, errors.ErrPanicUnhandled) {
		t.Errorf("parent.goErr = %v, want errors.ErrPanicUnhandled", parent.ctx.goErr)
	}

	if errors.Equals(parent.ctx.goErr, errors.ErrStop) {
		t.Error("parent.goErr equals errors.ErrStop; the unrecovered panic was not distinguished from normal completion")
	}
}

// Test_GoRoutine_OrdinaryError_StopsParentContext is a regression guard for
// the behavior BUG-45's report used as its own point of comparison: an
// ordinary (non-panic) runtime error inside a goroutine already correctly
// stopped the parent context before this fix, and must continue to do so
// afterward. fx's body is a single TryPop instruction with no matching Try,
// which unconditionally returns errors.ErrTryCatchMismatch -- a plain
// runtime error, not a panic and not ErrStop.
func Test_GoRoutine_OrdinaryError_StopsParentContext(t *testing.T) {
	parent := newTestContext(t)
	parent.ctx.EnableConsoleOutput(false)
	parent.ctx.running.Store(true)

	fx := New("erroring-goroutine").Literal(true)
	fx.Emit(TryPop, nil)

	goRoutineCompletion.Add(1)
	GoRoutine(fx, parent.ctx, data.NewList())

	if parent.ctx.running.Load() {
		t.Error("expected parent context to stop running after an ordinary goroutine error")
	}

	if parent.ctx.goErr == nil {
		t.Fatal("expected parent.goErr to be set after an ordinary goroutine error, got nil")
	}

	if !errors.Equals(parent.ctx.goErr, errors.ErrTryCatchMismatch) {
		t.Errorf("parent.goErr = %v, want errors.ErrTryCatchMismatch", parent.ctx.goErr)
	}
}

// Test_GoRoutine_NormalCompletion_DoesNotStopParentContext is a regression
// guard for the ordinary, successful case: a goroutine that runs to
// completion without any error must NOT touch the parent context's running
// flag or goErr at all. fx's body is empty (falls off the end immediately),
// which is exactly the "normal completion" shape unwindPanic's ErrStop used
// to be indistinguishable from.
func Test_GoRoutine_NormalCompletion_DoesNotStopParentContext(t *testing.T) {
	parent := newTestContext(t)
	parent.ctx.EnableConsoleOutput(false)
	parent.ctx.running.Store(true)

	fx := New("normal-goroutine").Literal(true)

	goRoutineCompletion.Add(1)
	GoRoutine(fx, parent.ctx, data.NewList())

	if !parent.ctx.running.Load() {
		t.Error("a normally-completed goroutine must not clear the parent context's running flag")
	}

	if parent.ctx.goErr != nil {
		t.Errorf("a normally-completed goroutine must not set parent.goErr, got %v", parent.ctx.goErr)
	}
}
