package bytecode

// callCastFunction_test.go contains unit tests for callTypeCast in
// callCastFunction.go.
//
// # What callTypeCast does
//
// callTypeCast is called by callByteCode whenever the "function pointer" on
// the stack is a *data.Type rather than an actual function.  It implements
// type-cast syntax: `int(x)`, `float64(x)`, `time.Duration(n)`, etc.
//
// The function has two dispatch paths:
//
//  1. Struct-based types (Path A):
//     When the type is a StructKind, or a TypeKind whose base type is a
//     StructKind, the function handles special Go-native struct types that
//     cannot be constructed generically.  Currently one is handled:
//       - time.Duration  — wraps an int64 nanosecond count
//     Any other struct type returns ErrInvalidFunctionTypeCall.
//     (time.Month used to be handled here too, but it is now a plain scalar
//     type that casts through Path B like any other named integer type.)
//
//  2. Scalar / array types (Path B):
//     All other types delegate to builtins.Cast, which handles numeric
//     widening/narrowing, bool conversions, string formatting, interface
//     wrapping, and array element-type conversions.
//
// # How to read these tests
//
// Each test builds a testContext (see testhelpers_test.go), calls
// callTypeCast directly with a *data.Type and an argument slice, and then
// asserts either the error returned or the value pushed onto the stack.
//
// Helper function makeDurationType constructs a minimal *data.Type value that
// satisfies the native-name check inside callTypeCast without requiring the
// full runtime/time package to be imported.

import (
	"testing"
	"time"

	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
)

// ─── helpers ─────────────────────────────────────────────────────────────────

// makeDurationType returns a *data.Type that represents time.Duration for use
// in tests.  It has kind=TypeKind, base=StructKind, and
// NativeName()=="time.Duration" — exactly what callTypeCast checks in Path A.
//
// We construct it here rather than importing the full runtime/time package so
// that the test stays self-contained within the bytecode package.
func makeDurationType() *data.Type {
	return data.TypeDefinition("Duration", data.StructureType()).
		SetNativeName(defs.TimeDurationTypeName)
}

// makeUnknownStructType returns a struct-based type whose NativeName is NOT one
// of the two special cases (Duration/Month).  callTypeCast must reject it with
// ErrInvalidFunctionTypeCall.
func makeUnknownStructType() *data.Type {
	return data.TypeDefinition("Widget", data.StructureType()).
		SetNativeName("widgets.Widget")
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 1: Path B — scalar type casts via builtins.Cast
// ─────────────────────────────────────────────────────────────────────────────
//
// All non-struct types delegate to builtins.Cast.  The tests below verify the
// most common conversions: numeric coercion, string formatting, bool
// conversion, and interface wrapping.

// Test_callTypeCast_Int_FromInt verifies that casting an integer argument
// to data.IntType pushes the (possibly coerced) integer value.
func Test_callTypeCast_Int_FromInt(t *testing.T) {
	tc := newTestContext(t)

	err := callTypeCast(data.IntType, []any{42}, tc.ctx)

	tc.assertNoError(err)
	tc.assertTopStack(42)
}

// Test_callTypeCast_Int_FromFloat64 verifies that a float64 argument is
// truncated toward zero when cast to int.
func Test_callTypeCast_Int_FromFloat64(t *testing.T) {
	tc := newTestContext(t)

	err := callTypeCast(data.IntType, []any{float64(7.9)}, tc.ctx)

	tc.assertNoError(err)
	tc.assertTopStack(7)
}

// Test_callTypeCast_Int_FromNumericString verifies that a string containing
// a valid integer is parsed and cast to int.
func Test_callTypeCast_Int_FromNumericString(t *testing.T) {
	tc := newTestContext(t)

	err := callTypeCast(data.IntType, []any{"99"}, tc.ctx)

	tc.assertNoError(err)
	tc.assertTopStack(99)
}

// Test_callTypeCast_Float64_FromInt verifies integer-to-float64 widening.
func Test_callTypeCast_Float64_FromInt(t *testing.T) {
	tc := newTestContext(t)

	err := callTypeCast(data.Float64Type, []any{5}, tc.ctx)

	tc.assertNoError(err)
	tc.assertTopStack(float64(5))
}

// Test_callTypeCast_String_FromInt verifies that an integer is formatted as
// its decimal string representation when cast to string.
func Test_callTypeCast_String_FromInt(t *testing.T) {
	tc := newTestContext(t)

	err := callTypeCast(data.StringType, []any{123}, tc.ctx)

	tc.assertNoError(err)
	tc.assertTopStack("123")
}

// Test_callTypeCast_Bool_FromZero verifies that the integer 0 casts to false.
func Test_callTypeCast_Bool_FromZero(t *testing.T) {
	tc := newTestContext(t)

	err := callTypeCast(data.BoolType, []any{0}, tc.ctx)

	tc.assertNoError(err)
	tc.assertTopStack(false)
}

// Test_callTypeCast_Bool_FromOne verifies that the integer 1 casts to true.
func Test_callTypeCast_Bool_FromOne(t *testing.T) {
	tc := newTestContext(t)

	err := callTypeCast(data.BoolType, []any{1}, tc.ctx)

	tc.assertNoError(err)
	tc.assertTopStack(true)
}

// Test_callTypeCast_Interface_WrapsValue verifies that casting to
// data.InterfaceType wraps the concrete value in a data.Interface struct.
// This is how Ego boxes a concrete value into an interface{} variable.
func Test_callTypeCast_Interface_WrapsValue(t *testing.T) {
	tc := newTestContext(t)

	err := callTypeCast(data.InterfaceType, []any{42}, tc.ctx)

	tc.assertNoError(err)

	// data.Wrap(42) produces Interface{Value:42, BaseType:IntType}.
	tc.assertTopStack(data.Interface{Value: 42, BaseType: data.IntType})
}

// Test_callTypeCast_PathB_LeavesStackEmpty verifies that after an error in
// builtins.Cast (e.g. converting "not-a-number" to int fails), nothing is
// pushed onto the stack.
func Test_callTypeCast_PathB_ErrorLeavesStackEmpty(t *testing.T) {
	tc := newTestContext(t)

	// "not-a-number" cannot be coerced to int → error expected.
	err := callTypeCast(data.IntType, []any{"not-a-number"}, tc.ctx)

	if err == nil {
		t.Error("expected error for non-numeric string to int cast, got nil")
	}

	tc.assertStackEmpty()
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 2: Path A — time.Duration cast
// ─────────────────────────────────────────────────────────────────────────────
//
// callTypeCast handles time.Duration as a special case: it converts the
// argument to int64 and wraps it with time.Duration().  A Duration value in
// Ego represents an elapsed time in nanoseconds.

// Test_callTypeCast_Duration_FromInt64 verifies that an int64 argument is
// correctly wrapped as a time.Duration with the same nanosecond count.
func Test_callTypeCast_Duration_FromInt64(t *testing.T) {
	tc := newTestContext(t)

	const nanos = int64(5_000_000_000) // 5 seconds in nanoseconds
	err := callTypeCast(makeDurationType(), []any{nanos}, tc.ctx)

	tc.assertNoError(err)
	tc.assertTopStack(time.Duration(nanos))
}

// Test_callTypeCast_Duration_FromInt verifies that a plain int argument is
// coerced to int64 before being wrapped as time.Duration.
func Test_callTypeCast_Duration_FromInt(t *testing.T) {
	tc := newTestContext(t)

	err := callTypeCast(makeDurationType(), []any{1000}, tc.ctx)

	tc.assertNoError(err)
	tc.assertTopStack(time.Duration(1000))
}

// Test_callTypeCast_Duration_Zero verifies that the zero value is accepted
// and produces a zero-length duration.
func Test_callTypeCast_Duration_Zero(t *testing.T) {
	tc := newTestContext(t)

	err := callTypeCast(makeDurationType(), []any{0}, tc.ctx)

	tc.assertNoError(err)
	tc.assertTopStack(time.Duration(0))
}

// Test_callTypeCast_Duration_NonNumericArg verifies that a string argument
// that cannot be converted to int64 returns a runtime error.
func Test_callTypeCast_Duration_NonNumericArg(t *testing.T) {
	tc := newTestContext(t)

	err := callTypeCast(makeDurationType(), []any{"not-a-number"}, tc.ctx)

	if err == nil {
		t.Error("expected error for non-numeric Duration arg, got nil")
	}

	tc.assertStackEmpty()
}

// Test_callTypeCast_Duration_EmptyArgs verifies the CALL-7 fix: when
// callTypeCast is invoked with an empty argument slice for a Duration type,
// it must return ErrArgumentCount instead of panicking with an index-out-of-
// bounds error.
func Test_callTypeCast_Duration_EmptyArgs(t *testing.T) {
	tc := newTestContext(t)

	err := callTypeCast(makeDurationType(), []any{}, tc.ctx)

	tc.assertError(err, errors.ErrArgumentCount)
	tc.assertStackEmpty()
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 3: Path A — unrecognized struct type
// ─────────────────────────────────────────────────────────────────────────────
//
// A struct-based type whose NativeName is not "time.Duration" falls through to
// the default case and returns ErrInvalidFunctionTypeCall. (time.Month is no
// longer a struct type; it casts through Path B like any other scalar.)

// Test_callTypeCast_UnknownStructType verifies that a TypeKind-wrapping-Struct
// type with an unrecognized NativeName returns ErrInvalidFunctionTypeCall.
func Test_callTypeCast_UnknownStructType(t *testing.T) {
	tc := newTestContext(t)

	err := callTypeCast(makeUnknownStructType(), []any{42}, tc.ctx)

	tc.assertError(err, errors.ErrInvalidFunctionTypeCall)
	tc.assertStackEmpty()
}

// Test_callTypeCast_PlainStructType verifies the case where the type has
// kind=StructKind directly (not wrapped in TypeKind) and no NativeName.
// This also falls through to the default case.
func Test_callTypeCast_PlainStructType(t *testing.T) {
	tc := newTestContext(t)

	plainStruct := data.StructureType() // kind=StructKind, NativeName=""

	err := callTypeCast(plainStruct, []any{42}, tc.ctx)

	tc.assertError(err, errors.ErrInvalidFunctionTypeCall)
	tc.assertStackEmpty()
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 5: Stack invariants
// ─────────────────────────────────────────────────────────────────────────────

// Test_callTypeCast_SuccessLeavesExactlyOneResult verifies that exactly one
// value is pushed onto the stack on a successful cast.  A prior stack item
// must not be consumed and must remain below the result.
func Test_callTypeCast_SuccessLeavesExactlyOneResult(t *testing.T) {
	tc := newTestContext(t).withStack("pre-existing")

	err := callTypeCast(data.IntType, []any{7}, tc.ctx)

	tc.assertNoError(err)

	// The cast result must be on top.
	tc.assertTopStack(7)

	// The pre-existing item must still be on the stack underneath.
	tc.assertTopStack("pre-existing")

	tc.assertStackEmpty()
}

// Test_callTypeCast_ErrorDoesNotPushAnything verifies that when the cast fails
// the stack is not modified — no partial result is left behind.
func Test_callTypeCast_ErrorDoesNotPushAnything(t *testing.T) {
	tc := newTestContext(t).withStack("sentinel")

	// Invalid conversion — boolean cannot become an integer.
	err := callTypeCast(makeUnknownStructType(), []any{true}, tc.ctx)

	if err == nil {
		t.Error("expected error, got nil")
	}

	// The sentinel must still be the only item on the stack.
	tc.assertTopStack("sentinel")
	tc.assertStackEmpty()
}
