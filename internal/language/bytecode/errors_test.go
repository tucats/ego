package bytecode

import (
	"testing"

	"github.com/tucats/ego/internal/errors"
)

// Tests for throwByteCode, the runtime handler for the Throw opcode emitted
// by compileThrow (internal/language/compiler/throw.go) for the "throw"
// language extension. See docs/LANGUAGE.md's try/throw section.
//
// Unlike signalByteCode (used by the @error test directive, which always
// raises whatever value is on the stack), throwByteCode treats a nil -- or
// Ego's zero-value error -- as "nothing to throw" and returns without error,
// so that a real error condition popped from the stack is the only thing
// that becomes a catchable runtime error.

// Test_throwByteCode_NilValue_NoOp verifies that throwing a plain nil value
// is a no-op: no error is returned, and nothing is left on the stack.
func Test_throwByteCode_NilValue_NoOp(t *testing.T) {
	tc := newTestContext(t).withStack(nil)

	err := throwByteCode(tc.ctx, nil)

	tc.assertNoError(err)
	tc.assertStackEmpty()
}

// Test_throwByteCode_ZeroValueError_NoOp verifies that throwing Ego's
// zero-value error (a non-nil *errors.Error with no inner error, the value
// produced for "var e error" with no initializer) is also treated as
// nothing to throw, matching data.IsNil's definition of "nil" for errors.
func Test_throwByteCode_ZeroValueError_NoOp(t *testing.T) {
	tc := newTestContext(t).withStack(&errors.Error{})

	err := throwByteCode(tc.ctx, nil)

	tc.assertNoError(err)
	tc.assertStackEmpty()
}

// Test_throwByteCode_RealError_Raised verifies that throwing a genuine,
// non-nil error returns it as a catchable runtime error, preserving its
// identity (Code() still reports the original key).
func Test_throwByteCode_RealError_Raised(t *testing.T) {
	original := errors.ErrDivisionByZero.Clone()

	tc := newTestContext(t).withStack(original)

	err := throwByteCode(tc.ctx, nil)

	if err == nil {
		t.Fatal("expected throwByteCode to return a non-nil error, got nil")
	}

	egErr, ok := err.(*errors.Error)
	if !ok {
		t.Fatalf("expected *errors.Error, got %T (%v)", err, err)
	}

	if egErr.Code() != errors.ErrDivisionByZero.Code() {
		t.Errorf("expected Code() = %q, got %q", errors.ErrDivisionByZero.Code(), egErr.Code())
	}
}

// Test_throwByteCode_NativeGoError_Raised verifies that throwing a plain Go
// error (not already an *errors.Error) is wrapped and raised correctly.
func Test_throwByteCode_NativeGoError_Raised(t *testing.T) {
	tc := newTestContext(t).withStack(errors.Message("boom"))

	err := throwByteCode(tc.ctx, nil)

	if err == nil {
		t.Fatal("expected throwByteCode to return a non-nil error, got nil")
	}
}

// Test_throwByteCode_NonErrorValue_ReturnsArgumentTypeError verifies that
// throwing a value which is not an error at all (e.g. an int) produces
// ErrArgumentType rather than silently converting it into an error message
// the way signalByteCode's @error fallback does -- "throw" requires an
// actual error value.
func Test_throwByteCode_NonErrorValue_ReturnsArgumentTypeError(t *testing.T) {
	tc := newTestContext(t).withStack(42)

	err := throwByteCode(tc.ctx, nil)

	tc.assertError(err, errors.ErrArgumentType)
}

// Test_throwByteCode_EmptyStack verifies that popping from an empty stack
// surfaces an error rather than panicking.
func Test_throwByteCode_EmptyStack(t *testing.T) {
	tc := newTestContext(t)

	err := throwByteCode(tc.ctx, nil)

	if err == nil {
		t.Error("throwByteCode on empty stack: expected error, got nil")
	}
}

// Test_throwByteCode_StackMarker returns ErrFunctionReturnedVoid when a
// StackMarker is on the stack (e.g. a void function call result), mirroring
// the same guard used by requiredTypeByteCode and signalByteCode's callers.
func Test_throwByteCode_StackMarker(t *testing.T) {
	tc := newTestContext(t).withStack(NewStackMarker("void"))

	err := throwByteCode(tc.ctx, nil)

	tc.assertError(err, errors.ErrFunctionReturnedVoid)
}
