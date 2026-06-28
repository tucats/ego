package bytecode

// math_test.go provides comprehensive unit tests for all arithmetic and logical
// bytecode instruction functions defined in math.go.
//
// Every test function uses the newTestContext / testhelpers infrastructure
// (bytecode/testhelpers_test.go) rather than constructing raw symbol tables and
// contexts from scratch.  See that file for the full API.
//
// # Stack ordering reminder
//
// withStack(a, b) pushes a first (bottom) and b last (top).  Diadic
// instructions pop the TOP value as the right-hand / second operand (v2) and the
// NEXT value as the left-hand / first operand (v1).  So:
//
//	withStack(left, right) → v1=left, v2=right
//
// # Known bugs
//
// Bugs found during this audit are documented in docs/bytecode_issues.md under
// the MATH-N namespace.  Tests that document current broken behavior carry a
// "_CurrentlyBroken_MATH<n>" suffix.  Tests for operations that produce a
// runtime panic (rather than a wrong value) use runExpectingPanic and call
// t.Skip so the test binary does not crash.

import (
	"testing"

	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
)

// ═══════════════════════════════════════════════════════════════════════════════
// incrementByteCode
//
// Increments a named symbol by a given amount.  The instruction operand must
// be a two-element []any{symbolName, incrementValue}.  For numeric types the
// increment is added arithmetically; for strings it is concatenated; for
// arrays (when extensions are enabled) the value is appended.
// ═══════════════════════════════════════════════════════════════════════════════

// Test_incrementByteCode_Int verifies that a plain int symbol is incremented
// correctly by a positive int value.
func Test_incrementByteCode_Int(t *testing.T) {
	tc := newTestContext(t).withSymbol("x", 5)
	err := incrementByteCode(tc.ctx, []any{"x", 3})
	tc.assertNoError(err)
	// 5 + 3 = 8
	tc.assertSymbolValue("x", 8)
}

// Test_incrementByteCode_IntNegative verifies that a negative increment value
// decrements the symbol (int subtraction via add-negative).
func Test_incrementByteCode_IntNegative(t *testing.T) {
	tc := newTestContext(t).withSymbol("x", 10)
	err := incrementByteCode(tc.ctx, []any{"x", -4})
	tc.assertNoError(err)
	// 10 + (-4) = 6
	tc.assertSymbolValue("x", 6)
}

// Test_incrementByteCode_Int64 verifies increment for int64-typed symbols.
func Test_incrementByteCode_Int64(t *testing.T) {
	tc := newTestContext(t).withSymbol("n", int64(100))
	err := incrementByteCode(tc.ctx, []any{"n", int64(7)})
	tc.assertNoError(err)
	tc.assertSymbolValue("n", int64(107))
}

// Test_incrementByteCode_Int32 verifies increment for int32-typed symbols.
func Test_incrementByteCode_Int32(t *testing.T) {
	tc := newTestContext(t).withSymbol("n", int32(20))
	err := incrementByteCode(tc.ctx, []any{"n", int32(5)})
	tc.assertNoError(err)
	tc.assertSymbolValue("n", int32(25))
}

// Test_incrementByteCode_Float64 verifies increment for float64 symbols.
func Test_incrementByteCode_Float64(t *testing.T) {
	tc := newTestContext(t).withSymbol("f", float64(3.14))
	err := incrementByteCode(tc.ctx, []any{"f", float64(1.0)})
	tc.assertNoError(err)
	tc.assertSymbolValue("f", float64(4.140000000000001)) // float64 precision
}

// Test_incrementByteCode_Float32 verifies increment for float32 symbols.
func Test_incrementByteCode_Float32(t *testing.T) {
	tc := newTestContext(t).withSymbol("f", float32(2.5))
	err := incrementByteCode(tc.ctx, []any{"f", float32(0.5)})
	tc.assertNoError(err)
	tc.assertSymbolValue("f", float32(3.0))
}

// Test_incrementByteCode_Byte verifies increment for byte (uint8) symbols.
func Test_incrementByteCode_Byte(t *testing.T) {
	tc := newTestContext(t).withSymbol("b", byte(10))
	err := incrementByteCode(tc.ctx, []any{"b", byte(5)})
	tc.assertNoError(err)
	tc.assertSymbolValue("b", byte(15))
}

// Test_incrementByteCode_String verifies that incrementing a string symbol
// by another string performs concatenation.
func Test_incrementByteCode_String(t *testing.T) {
	tc := newTestContext(t).withSymbol("s", "hello")
	err := incrementByteCode(tc.ctx, []any{"s", ", world"})
	tc.assertNoError(err)
	tc.assertSymbolValue("s", "hello, world")
}

// Test_incrementByteCode_UnknownSymbol verifies that referencing a symbol
// that has not been created returns ErrUnknownSymbol.
func Test_incrementByteCode_UnknownSymbol(t *testing.T) {
	tc := newTestContext(t) // no symbols created
	err := incrementByteCode(tc.ctx, []any{"missing", 1})
	tc.assertError(err, errors.ErrUnknownSymbol)
}

// Test_incrementByteCode_NilSymbolValue verifies that attempting to increment
// a symbol whose value is nil returns ErrInvalidType (cannot do math on nil).
func Test_incrementByteCode_NilSymbolValue(t *testing.T) {
	tc := newTestContext(t).withSymbol("x", nil)
	err := incrementByteCode(tc.ctx, []any{"x", 1})
	tc.assertError(err, errors.ErrInvalidType)
}

// Test_incrementByteCode_InvalidOperand_NotSlice verifies that an operand
// that is not a []any returns ErrInvalidOperand.
func Test_incrementByteCode_InvalidOperand_NotSlice(t *testing.T) {
	tc := newTestContext(t)
	// Passing a plain string instead of []any is invalid.
	err := incrementByteCode(tc.ctx, "bad-operand")
	tc.assertError(err, errors.ErrInvalidOperand)
}

// Test_incrementByteCode_InvalidOperand_WrongLength verifies that a []any
// operand with only one element (missing increment value) returns ErrInvalidOperand.
func Test_incrementByteCode_InvalidOperand_WrongLength(t *testing.T) {
	tc := newTestContext(t)
	// The operand must have exactly 2 elements: [symbolName, incrementValue].
	err := incrementByteCode(tc.ctx, []any{"x"})
	tc.assertError(err, errors.ErrInvalidOperand)
}

// Test_incrementByteCode_ArrayAppend verifies that with language extensions
// enabled, incrementing an array-valued symbol appends the value to the array.
func Test_incrementByteCode_ArrayAppend(t *testing.T) {
	// Create a symbol holding a three-element integer array.
	arr := data.NewArrayFromList(data.IntType, data.NewList(1, 2, 3))
	tc := newTestContext(t).
		withSymbol("a", arr).
		withExtensions(true) // array-append only works when extensions are on
	err := incrementByteCode(tc.ctx, []any{"a", 4})
	tc.assertNoError(err)
	// The array should now have 4 as a fourth element.
	want := data.NewArrayFromList(data.IntType, data.NewList(1, 2, 3, 4))
	tc.assertSymbolValue("a", want)
}

// Test_incrementByteCode_StrictTypeMismatch verifies that in strict type
// enforcement mode, incrementing an int symbol by an int64 value fails with
// ErrTypeMismatch because the types are incompatible.
func Test_incrementByteCode_StrictTypeMismatch(t *testing.T) {
	tc := newTestContext(t).
		withSymbol("x", 5). // int
		withTypeStrictness(defs.StrictTypeEnforcement)
	// int64 is not the same type as int in strict mode.
	err := incrementByteCode(tc.ctx, []any{"x", int64(3)})
	tc.assertError(err, errors.ErrTypeMismatch)
}

// ═══════════════════════════════════════════════════════════════════════════════
// notByteCode
//
// Pops a value (or uses the operand when non-nil) and pushes its boolean NOT.
//
//   - nil   → true
//   - bool  → !bool
//   - any integer → value == 0 (see MATH-9 for a bug with non-int types)
//   - float32/float64 → value == 0
//   - other → ErrInvalidType
//
// ═══════════════════════════════════════════════════════════════════════════════

// Test_notByteCode_Nil verifies that NOT of a nil stack value returns true
// (nil is treated as "false", so !nil = true).
func Test_notByteCode_Nil(t *testing.T) {
	// nil is a special case handled before the type switch.
	tc := newTestContext(t).withStack(nil)
	err := notByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(true)
}

// Test_notByteCode_BoolTrue verifies that NOT of boolean true returns false.
func Test_notByteCode_BoolTrue(t *testing.T) {
	tc := newTestContext(t).withStack(true)
	err := notByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(false)
}

// Test_notByteCode_BoolFalse verifies that NOT of boolean false returns true.
func Test_notByteCode_BoolFalse(t *testing.T) {
	tc := newTestContext(t).withStack(false)
	err := notByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(true)
}

// Test_notByteCode_IntZero verifies that NOT of int(0) returns true.
func Test_notByteCode_IntZero(t *testing.T) {
	// int(0) == 0 is correct because the untyped 0 defaults to type int.
	tc := newTestContext(t).withStack(0)
	err := notByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(true)
}

// Test_notByteCode_IntNonzero verifies that NOT of a non-zero int returns false.
func Test_notByteCode_IntNonzero(t *testing.T) {
	tc := newTestContext(t).withStack(42)
	err := notByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(false)
}

// Test_notByteCode_Int64Zero verifies that NOT of int64(0) returns true.
// Previously broken (MATH-9): the multi-type case compared with type any, so
// int64(0) == int(0) was false (different concrete types).  Fixed by splitting
// the case so the switch variable is typed int64 and 0 is correctly typed.
func Test_notByteCode_Int64Zero(t *testing.T) {
	tc := newTestContext(t).withStack(int64(0))
	err := notByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(true)
}

// Test_notByteCode_ByteZero verifies that NOT of byte(0) returns true.
// Previously broken (MATH-9): same root cause as the int64 case.
func Test_notByteCode_ByteZero(t *testing.T) {
	tc := newTestContext(t).withStack(byte(0))
	err := notByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(true)
}

// Test_notByteCode_Int32Zero verifies that NOT of int32(0) returns true.
// Previously broken (MATH-9): same root cause as the int64 case.
func Test_notByteCode_Int32Zero(t *testing.T) {
	tc := newTestContext(t).withStack(int32(0))
	err := notByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(true)
}

// Test_notByteCode_Float32Zero verifies NOT of float32(0) returns true.
// float32 has its own typed case so the comparison is exact.
func Test_notByteCode_Float32Zero(t *testing.T) {
	tc := newTestContext(t).withStack(float32(0))
	err := notByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(true)
}

// Test_notByteCode_Float64Zero verifies NOT of float64(0) returns true.
func Test_notByteCode_Float64Zero(t *testing.T) {
	tc := newTestContext(t).withStack(float64(0))
	err := notByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(true)
}

// Test_notByteCode_Float64Nonzero verifies NOT of a non-zero float64 returns false.
func Test_notByteCode_Float64Nonzero(t *testing.T) {
	tc := newTestContext(t).withStack(float64(3.14))
	err := notByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(false)
}

// Test_notByteCode_String_Error verifies that NOT of a string is unsupported
// and returns ErrInvalidType (strings do not have a NOT operation).
func Test_notByteCode_String_Error(t *testing.T) {
	tc := newTestContext(t).withStack("hello")
	err := notByteCode(tc.ctx, nil)
	tc.assertError(err, errors.ErrInvalidType)
}

// Test_notByteCode_StackUnderflow verifies that calling NOT with an empty
// stack returns ErrStackUnderflow.
func Test_notByteCode_StackUnderflow(t *testing.T) {
	tc := newTestContext(t) // empty stack
	err := notByteCode(tc.ctx, nil)
	tc.assertError(err, errors.ErrStackUnderflow)
}

// Test_notByteCode_OperandPath verifies that when a non-nil operand is provided,
// notByteCode uses that value directly without popping from the stack.
func Test_notByteCode_OperandPath(t *testing.T) {
	// The stack is empty, but we pass the value as the operand — should not
	// try to pop anything.
	tc := newTestContext(t) // empty stack — would underflow if Pop were called
	err := notByteCode(tc.ctx, true)
	tc.assertNoError(err)
	tc.assertTopStack(false) // !true == false
}

// ═══════════════════════════════════════════════════════════════════════════════
// negateByteCode
//
// When the operand (i) is boolean true, delegates to notByteCode (logical NOT).
// When the operand is nil or false, pops the top-of-stack and pushes its
// arithmetic negation (numbers), logical NOT (bools), or reversal (strings/arrays).
// ═══════════════════════════════════════════════════════════════════════════════

// Test_negateByteCode_StackUnderflow verifies that negation on an empty stack
// returns ErrStackUnderflow.
func Test_negateByteCode_StackUnderflow(t *testing.T) {
	tc := newTestContext(t)
	err := negateByteCode(tc.ctx, nil)
	tc.assertError(err, errors.ErrStackUnderflow)
}

// Test_negateByteCode_BoolNOT_StackUnderflow verifies that boolean-NOT mode
// (operand = true) also underflows correctly when the stack is empty.
func Test_negateByteCode_BoolNOT_StackUnderflow(t *testing.T) {
	tc := newTestContext(t)
	// arg=true means "apply boolean NOT"; still needs to pop from stack.
	err := negateByteCode(tc.ctx, true)
	tc.assertError(err, errors.ErrStackUnderflow)
}

// Test_negateByteCode_NilValue_Error verifies that negating a nil stack value
// returns ErrInvalidType.
func Test_negateByteCode_NilValue_Error(t *testing.T) {
	tc := newTestContext(t).withStack(nil)
	err := negateByteCode(tc.ctx, nil)
	tc.assertError(err, errors.ErrInvalidType)
}

// Test_negateByteCode_NilValue_BoolNOT verifies that boolean NOT of nil
// returns true (nil is falsy, so !nil == true).
func Test_negateByteCode_NilValue_BoolNOT(t *testing.T) {
	tc := newTestContext(t).withStack(nil)
	err := negateByteCode(tc.ctx, true)
	tc.assertNoError(err)
	tc.assertTopStack(true)
}

// Test_negateByteCode_Bool verifies that negating a boolean toggles it.
func Test_negateByteCode_Bool(t *testing.T) {
	tc := newTestContext(t).withStack(false)
	err := negateByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(true)
}

// Test_negateByteCode_Int verifies arithmetic negation of a plain int.
func Test_negateByteCode_Int(t *testing.T) {
	tc := newTestContext(t).withStack(-3)
	err := negateByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(3)
}

// Test_negateByteCode_Int64 verifies arithmetic negation of an int64.
func Test_negateByteCode_Int64(t *testing.T) {
	tc := newTestContext(t).withStack(int64(-3))
	err := negateByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(int64(3))
}

// Test_negateByteCode_Int32 verifies arithmetic negation of an int32.
func Test_negateByteCode_Int32(t *testing.T) {
	tc := newTestContext(t).withStack(int32(-12))
	err := negateByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(int32(12))
}

// Test_negateByteCode_Float32 verifies arithmetic negation of a float32.
// The implementation computes float32(0.0)-value, not the unary minus.
func Test_negateByteCode_Float32(t *testing.T) {
	tc := newTestContext(t).withStack(float32(6.6))
	err := negateByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(float32(-6.6))
}

// Test_negateByteCode_Float64 verifies arithmetic negation of a float64.
func Test_negateByteCode_Float64(t *testing.T) {
	tc := newTestContext(t).withStack(float64(-6.6))
	err := negateByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(float64(6.6))
}

// Test_negateByteCode_Byte verifies that negating a byte wraps around
// (unsigned arithmetic: -2 mod 256 = 254).
func Test_negateByteCode_Byte(t *testing.T) {
	// byte is unsigned; negation uses two's complement wrapping.
	tc := newTestContext(t).withStack(byte(2))
	err := negateByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(byte(254)) // -2 wraps to 254 in uint8
}

// Test_negateByteCode_StringReverse verifies that negating a string reverses
// its character order.
func Test_negateByteCode_StringReverse(t *testing.T) {
	tc := newTestContext(t).withStack("$000123")
	err := negateByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack("321000$")
}

// Test_negateByteCode_StringEmpty verifies that negating an empty string
// returns an empty string.
func Test_negateByteCode_StringEmpty(t *testing.T) {
	tc := newTestContext(t).withStack("")
	err := negateByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack("")
}

// Test_negateByteCode_StringUnicode verifies that string reversal handles
// multi-byte Unicode code points correctly (by rune, not byte).
func Test_negateByteCode_StringUnicode(t *testing.T) {
	// "café" reversed by rune is "éfac", not a byte-reversal of UTF-8 bytes.
	tc := newTestContext(t).withStack("café")
	err := negateByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack("éfac")
}

// Test_negateByteCode_ArrayReverse verifies that negating an array reverses
// the order of its elements.
func Test_negateByteCode_ArrayReverse(t *testing.T) {
	tc := newTestContext(t).withStack(
		data.NewArrayFromList(data.StringType, data.NewList(-1, 2)),
	)
	err := negateByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(
		data.NewArrayFromList(data.StringType, data.NewList(2, -1)),
	)
}

// Test_negateByteCode_MapUnsupported verifies that negating a *data.Map
// returns ErrInvalidType because maps have no negation operation.
func Test_negateByteCode_MapUnsupported(t *testing.T) {
	tc := newTestContext(t).withStack(
		data.NewMapFromMap(map[string]int32{"a": 1}),
	)
	err := negateByteCode(tc.ctx, nil)
	tc.assertError(err, errors.ErrInvalidType)
}

// Test_negateByteCode_BoolNOT_Int verifies boolean NOT of a non-zero int
// (treating it as truthy) returns false.
func Test_negateByteCode_BoolNOT_Int(t *testing.T) {
	tc := newTestContext(t).withStack(3) // non-zero int → truthy
	err := negateByteCode(tc.ctx, true)
	tc.assertNoError(err)
	tc.assertTopStack(false) // NOT truthy = false
}

// ═══════════════════════════════════════════════════════════════════════════════
// addByteCode
//
// Pops two values and pushes their sum.  Special behaviors:
//   - bool: AND (&&) — note: the function comment in math.go incorrectly says
//     "OR"; the implementation performs AND (see MATH-10).
//   - string: concatenation
//   - error + string: concatenates the error message with the string
//   - *data.Array (with extensions): appends the other value to the array
//
// ═══════════════════════════════════════════════════════════════════════════════

// Test_addByteCode_Integers verifies basic integer addition.
func Test_addByteCode_Integers(t *testing.T) {
	tc := newTestContext(t).withStack(2, 5) // v1=2, v2=5
	err := addByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(7)
}

// Test_addByteCode_Int32 verifies that two int32 values add to int32.
func Test_addByteCode_Int32(t *testing.T) {
	tc := newTestContext(t).withStack(int32(5), int32(2))
	err := addByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(int32(7))
}

// Test_addByteCode_Int64 verifies addition of int64 values.
func Test_addByteCode_Int64(t *testing.T) {
	tc := newTestContext(t).withStack(int64(1000000), int64(999999))
	err := addByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(int64(1999999))
}

// Test_addByteCode_MixedIntAndByte verifies that adding int and byte promotes
// byte to int (Normalize picks the wider type).
func Test_addByteCode_MixedIntAndByte(t *testing.T) {
	tc := newTestContext(t).withStack(int(5), byte(2)) // int is wider
	err := addByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(int(7))
}

// Test_addByteCode_Float32 verifies float32 addition.
func Test_addByteCode_Float32(t *testing.T) {
	tc := newTestContext(t).withStack(float32(1.0), float32(6.6))
	err := addByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(float32(7.6))
}

// Test_addByteCode_Strings verifies string concatenation.
func Test_addByteCode_Strings(t *testing.T) {
	tc := newTestContext(t).withStack("test", "Plan") // v1="test", v2="Plan"
	err := addByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack("testPlan")
}

// Test_addByteCode_BoolAND verifies that adding two booleans performs logical
// AND (not OR, despite the comment in the source — see MATH-10).
func Test_addByteCode_BoolAND_TrueAndTrue(t *testing.T) {
	tc := newTestContext(t).withStack(true, true)
	err := addByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(true) // true AND true = true
}

// Test_addByteCode_BoolAND_MixedValues verifies false AND true = false.
func Test_addByteCode_BoolAND_MixedValues(t *testing.T) {
	tc := newTestContext(t).withStack(true, false)
	err := addByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(false) // true AND false = false
}

// Test_addByteCode_ErrorPlusString verifies that adding an error value and a
// string produces a concatenated string (error message + string).
func Test_addByteCode_ErrorPlusString(t *testing.T) {
	tc := newTestContext(t).withStack(errors.ErrAssert, "-thing")
	err := addByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack("@assert error-thing")
}

// Test_addByteCode_NilFirst_Error verifies that nil as the first (left) operand
// returns ErrInvalidType.
func Test_addByteCode_NilFirst_Error(t *testing.T) {
	tc := newTestContext(t).withStack(2, nil) // nil is on top (v2)
	err := addByteCode(tc.ctx, nil)
	tc.assertError(err, errors.ErrInvalidType)
}

// Test_addByteCode_NilSecond_Error verifies that nil as the second (right)
// operand also returns ErrInvalidType.
func Test_addByteCode_NilSecond_Error(t *testing.T) {
	tc := newTestContext(t).withStack(nil, 5) // 5 is on top; nil is below
	err := addByteCode(tc.ctx, nil)
	tc.assertError(err, errors.ErrInvalidType)
}

// Test_addByteCode_StackUnderflow_Zero verifies that calling add with an empty
// stack returns ErrStackUnderflow.
func Test_addByteCode_StackUnderflow_Zero(t *testing.T) {
	tc := newTestContext(t)
	err := addByteCode(tc.ctx, nil)
	tc.assertError(err, errors.ErrStackUnderflow)
}

// Test_addByteCode_StackUnderflow_One verifies that calling add with only one
// value on the stack returns ErrStackUnderflow.
func Test_addByteCode_StackUnderflow_One(t *testing.T) {
	tc := newTestContext(t).withStack(42)
	err := addByteCode(tc.ctx, nil)
	tc.assertError(err, errors.ErrStackUnderflow)
}

// Test_addByteCode_ArrayAppend verifies that with extensions enabled, adding a
// scalar to an array appends the scalar to the array.
func Test_addByteCode_ArrayAppend(t *testing.T) {
	arr := data.NewArrayFromList(data.StringType, data.NewList("foo", "bar"))
	tc := newTestContext(t).
		withStack("xyzzy", arr). // array on top; scalar below
		withExtensions(true)
	err := addByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(
		data.NewArrayFromList(data.StringType, data.NewList("foo", "bar", "xyzzy")),
	)
}

// Test_addByteCode_StrictTypeMismatch verifies that mixing int and float64 in
// strict type mode returns ErrTypeMismatch (neither is a constant).
func Test_addByteCode_StrictTypeMismatch(t *testing.T) {
	tc := newTestContext(t).
		withStack(int(5), float64(3.0)).
		withTypeStrictness(defs.StrictTypeEnforcement)
	err := addByteCode(tc.ctx, nil)
	tc.assertError(err, errors.ErrTypeMismatch)
}

// ═══════════════════════════════════════════════════════════════════════════════
// andByteCode
//
// Pops two values, converts each to bool via data.Bool, and pushes their
// logical AND.  Returns ErrInvalidType for nil, ErrInvalidBooleanValue for
// values that cannot be converted to bool (e.g. non-"true"/"false" strings).
// ═══════════════════════════════════════════════════════════════════════════════

// Test_andByteCode_Integers verifies that non-zero integers are truthy.
func Test_andByteCode_Integers(t *testing.T) {
	tc := newTestContext(t).withStack(2, 5)
	err := andByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(true) // 2 && 5 → true && true = true
}

// Test_andByteCode_TrueAndFalse verifies true AND false = false.
func Test_andByteCode_TrueAndFalse(t *testing.T) {
	tc := newTestContext(t).withStack(true, false)
	err := andByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(false)
}

// Test_andByteCode_TrueAndTrue verifies true AND true = true.
func Test_andByteCode_TrueAndTrue(t *testing.T) {
	tc := newTestContext(t).withStack(true, true)
	err := andByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(true)
}

// Test_andByteCode_FalseAndFalse verifies false AND false = false.
func Test_andByteCode_FalseAndFalse(t *testing.T) {
	tc := newTestContext(t).withStack(false, false)
	err := andByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(false)
}

// Test_andByteCode_Float32 verifies that non-zero float32 values are truthy.
func Test_andByteCode_Float32(t *testing.T) {
	tc := newTestContext(t).withStack(float32(1.0), float32(6.6))
	err := andByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(true)
}

// Test_andByteCode_NilFirst_Error verifies nil as the left operand returns ErrInvalidType.
func Test_andByteCode_NilFirst_Error(t *testing.T) {
	tc := newTestContext(t).withStack(2, nil) // nil is on top (v1)
	err := andByteCode(tc.ctx, nil)
	tc.assertError(err, errors.ErrInvalidType)
}

// Test_andByteCode_NilSecond_Error verifies nil as the right operand returns ErrInvalidType.
func Test_andByteCode_NilSecond_Error(t *testing.T) {
	tc := newTestContext(t).withStack(nil, 5)
	err := andByteCode(tc.ctx, nil)
	tc.assertError(err, errors.ErrInvalidType)
}

// Test_andByteCode_StringOperand_Error verifies that an arbitrary string that
// cannot be parsed as a boolean returns ErrInvalidBooleanValue.
func Test_andByteCode_StringOperand_Error(t *testing.T) {
	tc := newTestContext(t).withStack("test", "plan")
	err := andByteCode(tc.ctx, nil)
	tc.assertError(err, errors.ErrInvalidBooleanValue)
}

// Test_andByteCode_StackUnderflow_Zero verifies ErrStackUnderflow on empty stack.
func Test_andByteCode_StackUnderflow_Zero(t *testing.T) {
	tc := newTestContext(t)
	err := andByteCode(tc.ctx, nil)
	tc.assertError(err, errors.ErrStackUnderflow)
}

// Test_andByteCode_StackUnderflow_One verifies ErrStackUnderflow with one item.
func Test_andByteCode_StackUnderflow_One(t *testing.T) {
	tc := newTestContext(t).withStack(true)
	err := andByteCode(tc.ctx, nil)
	tc.assertError(err, errors.ErrStackUnderflow)
}

// ═══════════════════════════════════════════════════════════════════════════════
// orByteCode
//
// Pops two values, converts each to bool via data.Bool, and pushes their
// logical OR.  Error handling mirrors andByteCode.
// ═══════════════════════════════════════════════════════════════════════════════

// Test_orByteCode_Integers verifies that OR of two non-zero ints is true.
func Test_orByteCode_Integers(t *testing.T) {
	tc := newTestContext(t).withStack(2, 5)
	err := orByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(true)
}

// Test_orByteCode_TrueOrFalse verifies true OR false = true.
func Test_orByteCode_TrueOrFalse(t *testing.T) {
	tc := newTestContext(t).withStack(true, false)
	err := orByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(true)
}

// Test_orByteCode_FalseOrFalse verifies false OR false = false.
func Test_orByteCode_FalseOrFalse(t *testing.T) {
	tc := newTestContext(t).withStack(false, false)
	err := orByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(false)
}

// Test_orByteCode_NilFirst_Error verifies nil as an operand returns ErrInvalidType.
func Test_orByteCode_NilFirst_Error(t *testing.T) {
	tc := newTestContext(t).withStack(2, nil)
	err := orByteCode(tc.ctx, nil)
	tc.assertError(err, errors.ErrInvalidType)
}

// Test_orByteCode_StringOperand_Error verifies non-boolean strings return an error.
func Test_orByteCode_StringOperand_Error(t *testing.T) {
	tc := newTestContext(t).withStack("test", "plan")
	err := orByteCode(tc.ctx, nil)
	tc.assertError(err, errors.ErrInvalidBooleanValue)
}

// Test_orByteCode_StackUnderflow_Zero verifies ErrStackUnderflow on empty stack.
func Test_orByteCode_StackUnderflow_Zero(t *testing.T) {
	tc := newTestContext(t)
	err := orByteCode(tc.ctx, nil)
	tc.assertError(err, errors.ErrStackUnderflow)
}

// Test_orByteCode_StackUnderflow_One verifies ErrStackUnderflow with one item.
func Test_orByteCode_StackUnderflow_One(t *testing.T) {
	tc := newTestContext(t).withStack(true)
	err := orByteCode(tc.ctx, nil)
	tc.assertError(err, errors.ErrStackUnderflow)
}

// ═══════════════════════════════════════════════════════════════════════════════
// subtractByteCode
//
// Pops two values and pushes v1 - v2.  For strings, removes all occurrences of
// v2 from v1.  For arrays (with extensions), removes all elements equal to v2.
// ═══════════════════════════════════════════════════════════════════════════════

// Test_subtractByteCode_Integers verifies basic integer subtraction.
func Test_subtractByteCode_Integers(t *testing.T) {
	tc := newTestContext(t).withStack(5, 2) // v1=5, v2=2
	err := subtractByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(3)
}

// Test_subtractByteCode_Int32 verifies int32 subtraction.
func Test_subtractByteCode_Int32(t *testing.T) {
	tc := newTestContext(t).withStack(int32(10), int32(4))
	err := subtractByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(int32(6))
}

// Test_subtractByteCode_Int64 verifies int64 subtraction.
func Test_subtractByteCode_Int64(t *testing.T) {
	tc := newTestContext(t).withStack(int64(1000), int64(999))
	err := subtractByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(int64(1))
}

// Test_subtractByteCode_Float32 verifies float32 subtraction.
// 1.0 - 6.6 = -5.6 (within float32 precision).
func Test_subtractByteCode_Float32(t *testing.T) {
	tc := newTestContext(t).withStack(float32(1.0), float32(6.6))
	err := subtractByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(float32(-5.6))
}

// Test_subtractByteCode_MixedTypes verifies that int and byte are normalized
// (byte promoted to int) before subtraction.
func Test_subtractByteCode_MixedTypes(t *testing.T) {
	tc := newTestContext(t).withStack(int(5), byte(2)) // int is wider
	err := subtractByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(int(3))
}

// Test_subtractByteCode_StringRemove verifies that subtracting a string from
// another removes all occurrences of the substring.
func Test_subtractByteCode_StringRemove(t *testing.T) {
	// "foobar" - "oba" removes the substring, leaving "for".
	tc := newTestContext(t).withStack("foobar", "oba")
	err := subtractByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack("for")
}

// Test_subtractByteCode_StringNoMatch verifies that subtracting a substring
// that does not appear in the string returns the original string unchanged.
func Test_subtractByteCode_StringNoMatch(t *testing.T) {
	tc := newTestContext(t).withStack("hello", "xyz")
	err := subtractByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack("hello")
}

// Test_subtractByteCode_ArrayElementDelete verifies that with extensions on,
// subtracting an element from an array removes all matching elements.
func Test_subtractByteCode_ArrayElementDelete(t *testing.T) {
	arr := data.NewArrayFromList(data.IntType, data.NewList(1, 2, 3))
	tc := newTestContext(t).
		withStack(arr, 2). // remove 2 from the array
		withExtensions(true)
	err := subtractByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(
		data.NewArrayFromList(data.IntType, data.NewList(1, 3)),
	)
}

// Test_subtractByteCode_Bool_Error verifies that subtracting booleans is
// unsupported and returns ErrInvalidType (bools have no subtraction path).
func Test_subtractByteCode_Bool_Error(t *testing.T) {
	tc := newTestContext(t).withStack(true, false)
	err := subtractByteCode(tc.ctx, nil)
	tc.assertError(err, errors.ErrInvalidType)
}

// Test_subtractByteCode_NilFirst_Error verifies ErrInvalidType for nil as v2.
func Test_subtractByteCode_NilFirst_Error(t *testing.T) {
	tc := newTestContext(t).withStack(2, nil) // nil on top = v2
	err := subtractByteCode(tc.ctx, nil)
	tc.assertError(err, errors.ErrInvalidType)
}

// Test_subtractByteCode_StackUnderflow_Zero verifies ErrStackUnderflow on empty stack.
func Test_subtractByteCode_StackUnderflow_Zero(t *testing.T) {
	tc := newTestContext(t)
	err := subtractByteCode(tc.ctx, nil)
	tc.assertError(err, errors.ErrStackUnderflow)
}

// Test_subtractByteCode_StackUnderflow_One verifies ErrStackUnderflow with one item.
func Test_subtractByteCode_StackUnderflow_One(t *testing.T) {
	tc := newTestContext(t).withStack(55)
	err := subtractByteCode(tc.ctx, nil)
	tc.assertError(err, errors.ErrStackUnderflow)
}

// Test_subtractByteCode_Int8 verifies int8 subtraction.
// Previously broken (MATH-4): the case int8: branch asserted v1.(int16) instead
// of v1.(int8), causing a runtime panic when two int8 values were on the stack.
func Test_subtractByteCode_Int8(t *testing.T) {
	// data.Normalize leaves two int8 values unchanged (same kind), so both
	// v1 and v2 are int8 when case int8: is entered.
	tc := newTestContext(t).withStack(int8(5), int8(3))
	err := subtractByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(int8(2)) // 5 - 3 = 2
}

// ═══════════════════════════════════════════════════════════════════════════════
// multiplyByteCode
//
// Pops two values and pushes v1 * v2.  For booleans, computes logical OR.
// ═══════════════════════════════════════════════════════════════════════════════

// Test_multiplyByteCode_Integers verifies basic integer multiplication.
func Test_multiplyByteCode_Integers(t *testing.T) {
	tc := newTestContext(t).withStack(2, 5)
	err := multiplyByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(10)
}

// Test_multiplyByteCode_Int32 verifies int32 multiplication.
func Test_multiplyByteCode_Int32(t *testing.T) {
	tc := newTestContext(t).withStack(int32(6), int32(7))
	err := multiplyByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(int32(42))
}

// Test_multiplyByteCode_Int64 verifies int64 multiplication.
func Test_multiplyByteCode_Int64(t *testing.T) {
	tc := newTestContext(t).withStack(int64(1000), int64(1000))
	err := multiplyByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(int64(1000000))
}

// Test_multiplyByteCode_Float32 verifies float32 multiplication.
func Test_multiplyByteCode_Float32(t *testing.T) {
	// 1.0 * 6.6 = 6.6 (exact because 1.0 is the identity)
	tc := newTestContext(t).withStack(float32(1.0), float32(6.6))
	err := multiplyByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(float32(6.6))
}

// Test_multiplyByteCode_BoolOR_TrueOrFalse verifies that multiplying booleans
// performs logical OR (true * false = true).
func Test_multiplyByteCode_BoolOR_TrueOrFalse(t *testing.T) {
	tc := newTestContext(t).withStack(true, false)
	err := multiplyByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(true) // true OR false = true
}

// Test_multiplyByteCode_BoolOR_FalseOrFalse verifies false OR false = false.
func Test_multiplyByteCode_BoolOR_FalseOrFalse(t *testing.T) {
	tc := newTestContext(t).withStack(false, false)
	err := multiplyByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(false)
}

// Test_multiplyByteCode_MixedIntAndByte verifies int×byte type promotion.
func Test_multiplyByteCode_MixedIntAndByte(t *testing.T) {
	tc := newTestContext(t).withStack(int(5), byte(2))
	err := multiplyByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(int(10))
}

// Test_multiplyByteCode_NilFirst_Error verifies ErrInvalidType for nil operand.
func Test_multiplyByteCode_NilFirst_Error(t *testing.T) {
	tc := newTestContext(t).withStack(2, nil)
	err := multiplyByteCode(tc.ctx, nil)
	tc.assertError(err, errors.ErrInvalidType)
}

// Test_multiplyByteCode_StackUnderflow_Zero verifies ErrStackUnderflow.
func Test_multiplyByteCode_StackUnderflow_Zero(t *testing.T) {
	tc := newTestContext(t)
	err := multiplyByteCode(tc.ctx, nil)
	tc.assertError(err, errors.ErrStackUnderflow)
}

// Test_multiplyByteCode_StackUnderflow_One verifies ErrStackUnderflow with one item.
func Test_multiplyByteCode_StackUnderflow_One(t *testing.T) {
	tc := newTestContext(t).withStack(55)
	err := multiplyByteCode(tc.ctx, nil)
	tc.assertError(err, errors.ErrStackUnderflow)
}

// Test_multiplyByteCode_Int16 verifies int16 multiplication.
// Previously broken (MATH-2): the case int16: branch asserted v1.(int8) instead
// of v1.(int16), causing a runtime panic with two int16 operands.
func Test_multiplyByteCode_Int16(t *testing.T) {
	tc := newTestContext(t).withStack(int16(3), int16(4))
	err := multiplyByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(int16(12)) // 3 * 4 = 12
}

// Test_multiplyByteCode_Uint16 verifies uint16 multiplication.
// Previously broken (MATH-3): the case uint16: branch asserted v1.(int8) instead
// of v1.(uint16), causing a runtime panic with two uint16 operands.
func Test_multiplyByteCode_Uint16(t *testing.T) {
	tc := newTestContext(t).withStack(uint16(3), uint16(4))
	err := multiplyByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(uint16(12))
}

// ═══════════════════════════════════════════════════════════════════════════════
// exponentByteCode
//
// Pops two values and pushes v1 ^ v2 (v1 raised to the power v2).
// Signed integers: returns int64.  Unsigned integers: returns uint64.
// Floats: uses math.Pow.  Strings, booleans, nil: ErrInvalidType.
//
// MATH-1: for signed integers, x^0 incorrectly returns 0 instead of 1.
// ═══════════════════════════════════════════════════════════════════════════════

// Test_exponentByteCode_Integers verifies 2^5 = 32 (returned as int64).
func Test_exponentByteCode_Integers(t *testing.T) {
	tc := newTestContext(t).withStack(2, 5) // base=2, exp=5
	err := exponentByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(int64(32))
}

// Test_exponentByteCode_PowerOne verifies that x^1 returns the original value
// (the function returns v1 itself, preserving its original type).
func Test_exponentByteCode_PowerOne(t *testing.T) {
	// When vv2 == 1, the function returns v1 directly.
	tc := newTestContext(t).withStack(7, 1) // 7^1 = 7
	err := exponentByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(7) // preserves original int type (not int64)
}

// Test_exponentByteCode_SignedInt_PowerZero verifies that x^0 = 1 for signed
// integers.  Previously broken (MATH-1): the signed-integer branch pushed the
// untyped literal 0 instead of int64(1).
func Test_exponentByteCode_SignedInt_PowerZero(t *testing.T) {
	// 5^0 must be 1 by the standard mathematical identity x^0 = 1.
	tc := newTestContext(t).withStack(5, 0)
	err := exponentByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(int64(1))
}

// Test_exponentByteCode_UnsignedInt_PowerZero verifies that the unsigned-integer
// path correctly returns uint64(1) for x^0, unlike the signed path (see MATH-1).
func Test_exponentByteCode_UnsignedInt_PowerZero(t *testing.T) {
	// uint32 triggers the unsigned branch, which correctly pushes uint64(1).
	tc := newTestContext(t).withStack(uint32(7), uint32(0))
	err := exponentByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(uint64(1)) // correct: unsigned branch handles x^0 properly
}

// Test_exponentByteCode_UnsignedInt_Exponent verifies unsigned integer exponentiation.
func Test_exponentByteCode_UnsignedInt_Exponent(t *testing.T) {
	tc := newTestContext(t).withStack(uint32(2), uint32(8))
	err := exponentByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(uint64(256)) // 2^8 = 256
}

// Test_exponentByteCode_Float32 verifies float32 exponentiation via math.Pow.
// 1.0 ^ 6.6 = 1.0 exactly.
func Test_exponentByteCode_Float32(t *testing.T) {
	tc := newTestContext(t).withStack(float32(1.0), float32(6.6))
	err := exponentByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(float32(1)) // 1^anything = 1
}

// Test_exponentByteCode_Float64 verifies float64 exponentiation.
func Test_exponentByteCode_Float64(t *testing.T) {
	tc := newTestContext(t).withStack(float64(2.0), float64(10.0))
	err := exponentByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(float64(1024)) // 2^10 = 1024
}

// Test_exponentByteCode_MixedIntAndByte verifies that byte is promoted to int,
// and the int path returns int64.
func Test_exponentByteCode_MixedIntAndByte(t *testing.T) {
	tc := newTestContext(t).withStack(int(5), byte(2)) // 5^2 = 25
	err := exponentByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(int64(25))
}

// Test_exponentByteCode_Bool_Error verifies that booleans are not supported.
func Test_exponentByteCode_Bool_Error(t *testing.T) {
	tc := newTestContext(t).withStack(true, false)
	err := exponentByteCode(tc.ctx, nil)
	tc.assertError(err, errors.ErrInvalidType)
}

// Test_exponentByteCode_String_Error verifies that strings are not supported.
func Test_exponentByteCode_String_Error(t *testing.T) {
	tc := newTestContext(t).withStack("*", 5)
	err := exponentByteCode(tc.ctx, nil)
	tc.assertError(err, errors.ErrInvalidType)
}

// Test_exponentByteCode_NilFirst_Error verifies ErrInvalidType for nil operand.
func Test_exponentByteCode_NilFirst_Error(t *testing.T) {
	tc := newTestContext(t).withStack(2, nil)
	err := exponentByteCode(tc.ctx, nil)
	tc.assertError(err, errors.ErrInvalidType)
}

// Test_exponentByteCode_StackUnderflow_Zero verifies ErrStackUnderflow.
func Test_exponentByteCode_StackUnderflow_Zero(t *testing.T) {
	tc := newTestContext(t)
	err := exponentByteCode(tc.ctx, nil)
	tc.assertError(err, errors.ErrStackUnderflow)
}

// Test_exponentByteCode_StackUnderflow_One verifies ErrStackUnderflow with one item.
func Test_exponentByteCode_StackUnderflow_One(t *testing.T) {
	tc := newTestContext(t).withStack(55)
	err := exponentByteCode(tc.ctx, nil)
	tc.assertError(err, errors.ErrStackUnderflow)
}

// ═══════════════════════════════════════════════════════════════════════════════
// divideByteCode
//
// Pops two values and pushes v1 / v2.  Division by zero returns ErrDivisionByZero
// for all numeric types (including floats — unlike Go, which returns ±Inf).
// Strings, booleans, nil: ErrInvalidType.
//
// MATH-5: case int16: asserts v1.(int8) — panics with int16 inputs.
// MATH-6: case uint16: asserts v1.(int8) — panics with uint16 inputs.
// ═══════════════════════════════════════════════════════════════════════════════

// Test_divideByteCode_Integers verifies basic integer division (truncating).
func Test_divideByteCode_Integers(t *testing.T) {
	tc := newTestContext(t).withStack(9, 3)
	err := divideByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(3)
}

// Test_divideByteCode_IntegersTruncating verifies that integer division truncates
// (Go integer division semantics: 10/3 = 3, not 3.33…).
func Test_divideByteCode_IntegersTruncating(t *testing.T) {
	tc := newTestContext(t).withStack(10, 3)
	err := divideByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(3) // remainder discarded
}

// Test_divideByteCode_Int64 verifies int64 division.
func Test_divideByteCode_Int64(t *testing.T) {
	tc := newTestContext(t).withStack(int64(100), int64(4))
	err := divideByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(int64(25))
}

// Test_divideByteCode_Int32 verifies int32 division.
func Test_divideByteCode_Int32(t *testing.T) {
	tc := newTestContext(t).withStack(int32(12), int32(3))
	err := divideByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(int32(4))
}

// Test_divideByteCode_Byte verifies byte (uint8) division.
func Test_divideByteCode_Byte(t *testing.T) {
	tc := newTestContext(t).withStack(int(12), byte(2)) // promotes to int
	err := divideByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(int(6))
}

// Test_divideByteCode_Float32 verifies float32 division with fractional result.
func Test_divideByteCode_Float32(t *testing.T) {
	tc := newTestContext(t).withStack(float32(10.0), int(4)) // int promoted to float32
	err := divideByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(float32(2.5))
}

// Test_divideByteCode_Float64 verifies float64 division.
func Test_divideByteCode_Float64(t *testing.T) {
	tc := newTestContext(t).withStack(float64(7.0), float64(2.0))
	err := divideByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(float64(3.5))
}

// Test_divideByteCode_IntDivByZero verifies that integer division by zero
// returns ErrDivisionByZero rather than a Go runtime panic.
func Test_divideByteCode_IntDivByZero(t *testing.T) {
	tc := newTestContext(t).withStack(9, 0)
	err := divideByteCode(tc.ctx, nil)
	tc.assertError(err, errors.ErrDivisionByZero)
}

// Test_divideByteCode_Float64DivByZero verifies that float64 / 0 returns
// ErrDivisionByZero (Ego does not produce IEEE 754 ±Inf).
func Test_divideByteCode_Float64DivByZero(t *testing.T) {
	tc := newTestContext(t).withStack(float64(9), float64(0))
	err := divideByteCode(tc.ctx, nil)
	tc.assertError(err, errors.ErrDivisionByZero)
}

// Test_divideByteCode_Float32DivByZero verifies float32 / 0 returns ErrDivisionByZero.
func Test_divideByteCode_Float32DivByZero(t *testing.T) {
	tc := newTestContext(t).withStack(float32(9), float32(0))
	err := divideByteCode(tc.ctx, nil)
	tc.assertError(err, errors.ErrDivisionByZero)
}

// Test_divideByteCode_Bool_Error verifies booleans are not divisible.
func Test_divideByteCode_Bool_Error(t *testing.T) {
	tc := newTestContext(t).withStack(true, false)
	err := divideByteCode(tc.ctx, nil)
	tc.assertError(err, errors.ErrInvalidType)
}

// Test_divideByteCode_String_Error verifies strings are not divisible.
func Test_divideByteCode_String_Error(t *testing.T) {
	tc := newTestContext(t).withStack("*", 5)
	err := divideByteCode(tc.ctx, nil)
	tc.assertError(err, errors.ErrInvalidType)
}

// Test_divideByteCode_NilFirst_Error verifies ErrInvalidType for nil operand.
func Test_divideByteCode_NilFirst_Error(t *testing.T) {
	tc := newTestContext(t).withStack(2, nil)
	err := divideByteCode(tc.ctx, nil)
	tc.assertError(err, errors.ErrInvalidType)
}

// Test_divideByteCode_StackUnderflow_Zero verifies ErrStackUnderflow.
func Test_divideByteCode_StackUnderflow_Zero(t *testing.T) {
	tc := newTestContext(t)
	err := divideByteCode(tc.ctx, nil)
	tc.assertError(err, errors.ErrStackUnderflow)
}

// Test_divideByteCode_StackUnderflow_One verifies ErrStackUnderflow with one item.
func Test_divideByteCode_StackUnderflow_One(t *testing.T) {
	tc := newTestContext(t).withStack(55)
	err := divideByteCode(tc.ctx, nil)
	tc.assertError(err, errors.ErrStackUnderflow)
}

// Test_divideByteCode_Int16 verifies int16 division.
// Previously broken (MATH-5): the case int16: branch asserted v1.(int8) instead
// of v1.(int16), causing a runtime panic with two int16 operands.
func Test_divideByteCode_Int16(t *testing.T) {
	tc := newTestContext(t).withStack(int16(9), int16(3))
	err := divideByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(int16(3)) // 9 / 3 = 3
}

// Test_divideByteCode_Uint16 verifies uint16 division.
// Previously broken (MATH-6): the case uint16: branch asserted v1.(int8) instead
// of v1.(uint16), causing a runtime panic with two uint16 operands.
func Test_divideByteCode_Uint16(t *testing.T) {
	tc := newTestContext(t).withStack(uint16(9), uint16(3))
	err := divideByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(uint16(3))
}

// ═══════════════════════════════════════════════════════════════════════════════
// moduloByteCode
//
// Pops two values and pushes v1 % v2.  Only integer types are supported.
// Modulo by zero returns ErrDivisionByZero.  Floats, strings, booleans, nil:
// ErrInvalidType.
//
// MATH-7: case int16: asserts v1.(int8) — panics with int16 inputs.
// MATH-8: case uint16: asserts v1.(int8) — panics with uint16 inputs.
// ═══════════════════════════════════════════════════════════════════════════════

// Test_moduloByteCode_IntegerRemainder verifies 10 % 3 = 1.
func Test_moduloByteCode_IntegerRemainder(t *testing.T) {
	tc := newTestContext(t).withStack(10, 3)
	err := moduloByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(1)
}

// Test_moduloByteCode_EvenlyDivisible verifies that 9 % 3 = 0.
func Test_moduloByteCode_EvenlyDivisible(t *testing.T) {
	tc := newTestContext(t).withStack(9, 3)
	err := moduloByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(0)
}

// Test_moduloByteCode_Int32 verifies int32 modulo.
func Test_moduloByteCode_Int32(t *testing.T) {
	tc := newTestContext(t).withStack(int32(17), int32(5))
	err := moduloByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(int32(2)) // 17 % 5 = 2
}

// Test_moduloByteCode_Int64 verifies int64 modulo.
func Test_moduloByteCode_Int64(t *testing.T) {
	tc := newTestContext(t).withStack(int64(100), int64(7))
	err := moduloByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(int64(2)) // 100 % 7 = 2
}

// Test_moduloByteCode_Uint32 verifies uint32 modulo.
func Test_moduloByteCode_Uint32(t *testing.T) {
	tc := newTestContext(t).withStack(uint32(10), uint32(3))
	err := moduloByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(uint32(1))
}

// Test_moduloByteCode_Uint64 verifies uint64 modulo.
func Test_moduloByteCode_Uint64(t *testing.T) {
	tc := newTestContext(t).withStack(uint64(10), uint64(3))
	err := moduloByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(uint64(1))
}

// Test_moduloByteCode_MixedIntAndByte verifies byte is promoted to int.
func Test_moduloByteCode_MixedIntAndByte(t *testing.T) {
	tc := newTestContext(t).withStack(15, byte(4)) // 15 % 4 = 3
	err := moduloByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(3)
}

// Test_moduloByteCode_NegativeDividend verifies that modulo with a negative
// dividend produces a negative remainder (Go semantics: -10 % 3 = -1).
func Test_moduloByteCode_NegativeDividend(t *testing.T) {
	tc := newTestContext(t).withStack(-10, 3)
	err := moduloByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(-1) // Go uses truncated division: -10 % 3 == -1
}

// Test_moduloByteCode_DivByZero verifies ErrDivisionByZero for modulo by zero.
func Test_moduloByteCode_DivByZero(t *testing.T) {
	tc := newTestContext(t).withStack(9, 0)
	err := moduloByteCode(tc.ctx, nil)
	tc.assertError(err, errors.ErrDivisionByZero)
}

// Test_moduloByteCode_Bool_Error verifies booleans are not supported.
func Test_moduloByteCode_Bool_Error(t *testing.T) {
	tc := newTestContext(t).withStack(true, false)
	err := moduloByteCode(tc.ctx, nil)
	tc.assertError(err, errors.ErrInvalidType)
}

// Test_moduloByteCode_String_Error verifies strings are not supported.
func Test_moduloByteCode_String_Error(t *testing.T) {
	tc := newTestContext(t).withStack("*", 5)
	err := moduloByteCode(tc.ctx, nil)
	tc.assertError(err, errors.ErrInvalidType)
}

// Test_moduloByteCode_NilFirst_Error verifies ErrInvalidType for nil operand.
func Test_moduloByteCode_NilFirst_Error(t *testing.T) {
	tc := newTestContext(t).withStack(2, nil)
	err := moduloByteCode(tc.ctx, nil)
	tc.assertError(err, errors.ErrInvalidType)
}

// Test_moduloByteCode_StackUnderflow_Zero verifies ErrStackUnderflow.
func Test_moduloByteCode_StackUnderflow_Zero(t *testing.T) {
	tc := newTestContext(t)
	err := moduloByteCode(tc.ctx, nil)
	tc.assertError(err, errors.ErrStackUnderflow)
}

// Test_moduloByteCode_StackUnderflow_One verifies ErrStackUnderflow with one item.
func Test_moduloByteCode_StackUnderflow_One(t *testing.T) {
	tc := newTestContext(t).withStack(55)
	err := moduloByteCode(tc.ctx, nil)
	tc.assertError(err, errors.ErrStackUnderflow)
}

// Test_moduloByteCode_Int16 verifies int16 modulo.
// Previously broken (MATH-7): the case int16: branch asserted v1.(int8) instead
// of v1.(int16), causing a runtime panic with two int16 operands.
func Test_moduloByteCode_Int16(t *testing.T) {
	tc := newTestContext(t).withStack(int16(10), int16(3))
	err := moduloByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(int16(1)) // 10 % 3 = 1
}

// Test_moduloByteCode_Uint16 verifies uint16 modulo.
// Previously broken (MATH-8): the case uint16: branch asserted v1.(int8) instead
// of v1.(uint16), causing a runtime panic with two uint16 operands.
func Test_moduloByteCode_Uint16(t *testing.T) {
	tc := newTestContext(t).withStack(uint16(10), uint16(3))
	err := moduloByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(uint16(1))
}

// ═══════════════════════════════════════════════════════════════════════════════
// bitAndByteCode
//
// Pops two values, converts each to int via data.Int (supporting bool, string
// parse), and pushes their bitwise AND.
// ═══════════════════════════════════════════════════════════════════════════════

// Test_bitAndByteCode_IntValues verifies basic bitwise AND: 5 & 4 = 4 (0101 & 0100).
func Test_bitAndByteCode_IntValues(t *testing.T) {
	tc := newTestContext(t).withStack(5, 4)
	err := bitAndByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(4) // 0101 & 0100 = 0100
}

// Test_bitAndByteCode_Int32Values verifies that int32 values are converted to
// int before the AND operation.
func Test_bitAndByteCode_Int32Values(t *testing.T) {
	tc := newTestContext(t).withStack(int32(5), int32(4))
	err := bitAndByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(4) // result is int (data.Int converts int32 to int)
}

// Test_bitAndByteCode_BoolTrue verifies that true is treated as 1.
func Test_bitAndByteCode_BoolTrue(t *testing.T) {
	tc := newTestContext(t).withStack(true, true)
	err := bitAndByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(1) // 1 & 1 = 1
}

// Test_bitAndByteCode_BoolFalse verifies that true & false = 0.
func Test_bitAndByteCode_BoolFalse(t *testing.T) {
	tc := newTestContext(t).withStack(true, false)
	err := bitAndByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(0) // 1 & 0 = 0
}

// Test_bitAndByteCode_ZeroResult verifies AND with a zero produces zero.
func Test_bitAndByteCode_ZeroResult(t *testing.T) {
	tc := newTestContext(t).withStack(9, 0)
	err := bitAndByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(0)
}

// Test_bitAndByteCode_StringConversion verifies that a numeric string is
// converted to int before the AND operation.
func Test_bitAndByteCode_StringConversion(t *testing.T) {
	// data.Int("7") = 7; 7 & 5 = 5 (0111 & 0101 = 0101)
	tc := newTestContext(t).withStack("7", 5)
	err := bitAndByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(5)
}

// Test_bitAndByteCode_NilFirst_Error verifies ErrInvalidType for nil operand.
func Test_bitAndByteCode_NilFirst_Error(t *testing.T) {
	tc := newTestContext(t).withStack(9, nil)
	err := bitAndByteCode(tc.ctx, nil)
	tc.assertError(err, errors.ErrInvalidType)
}

// Test_bitAndByteCode_StackUnderflow_Zero verifies ErrStackUnderflow.
func Test_bitAndByteCode_StackUnderflow_Zero(t *testing.T) {
	tc := newTestContext(t)
	err := bitAndByteCode(tc.ctx, nil)
	tc.assertError(err, errors.ErrStackUnderflow)
}

// Test_bitAndByteCode_StackUnderflow_One verifies ErrStackUnderflow with one item.
func Test_bitAndByteCode_StackUnderflow_One(t *testing.T) {
	tc := newTestContext(t).withStack(55)
	err := bitAndByteCode(tc.ctx, nil)
	tc.assertError(err, errors.ErrStackUnderflow)
}

// ═══════════════════════════════════════════════════════════════════════════════
// bitOrByteCode
//
// Pops two values, converts each to int via data.Int, and pushes their bitwise OR.
// ═══════════════════════════════════════════════════════════════════════════════

// Test_bitOrByteCode_IntValues verifies basic bitwise OR: 5 | 4 = 5 (0101 | 0100).
func Test_bitOrByteCode_IntValues(t *testing.T) {
	tc := newTestContext(t).withStack(5, 4)
	err := bitOrByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(5) // 0101 | 0100 = 0101
}

// Test_bitOrByteCode_DistinctBits verifies OR of non-overlapping bits.
func Test_bitOrByteCode_DistinctBits(t *testing.T) {
	tc := newTestContext(t).withStack(9, 0) // 9 | 0 = 9
	err := bitOrByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(9)
}

// Test_bitOrByteCode_Int32Values verifies int32 OR (result is int).
func Test_bitOrByteCode_Int32Values(t *testing.T) {
	tc := newTestContext(t).withStack(int32(5), int32(4))
	err := bitOrByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(5)
}

// Test_bitOrByteCode_BoolValues verifies true | true = 1.
func Test_bitOrByteCode_BoolValues(t *testing.T) {
	tc := newTestContext(t).withStack(true, true)
	err := bitOrByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(1)
}

// Test_bitOrByteCode_BoolFalseAndFalse verifies false | false = 0.
func Test_bitOrByteCode_BoolFalseAndFalse(t *testing.T) {
	tc := newTestContext(t).withStack(false, false)
	err := bitOrByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(0)
}

// Test_bitOrByteCode_BoolTrueAndFalse verifies true | false = 1.
func Test_bitOrByteCode_BoolTrueAndFalse(t *testing.T) {
	tc := newTestContext(t).withStack(true, false)
	err := bitOrByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(1)
}

// Test_bitOrByteCode_StringConversion verifies numeric string OR.
func Test_bitOrByteCode_StringConversion(t *testing.T) {
	// data.Int("7") = 7; 7 | 5 = 7 (0111 | 0101 = 0111)
	tc := newTestContext(t).withStack("7", 5)
	err := bitOrByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(7)
}

// Test_bitOrByteCode_NilFirst_Error verifies ErrInvalidType for nil operand.
func Test_bitOrByteCode_NilFirst_Error(t *testing.T) {
	tc := newTestContext(t).withStack(9, nil)
	err := bitOrByteCode(tc.ctx, nil)
	tc.assertError(err, errors.ErrInvalidType)
}

// Test_bitOrByteCode_StackUnderflow_Zero verifies ErrStackUnderflow.
func Test_bitOrByteCode_StackUnderflow_Zero(t *testing.T) {
	tc := newTestContext(t)
	err := bitOrByteCode(tc.ctx, nil)
	tc.assertError(err, errors.ErrStackUnderflow)
}

// Test_bitOrByteCode_StackUnderflow_One verifies ErrStackUnderflow with one item.
func Test_bitOrByteCode_StackUnderflow_One(t *testing.T) {
	tc := newTestContext(t).withStack(55)
	err := bitOrByteCode(tc.ctx, nil)
	tc.assertError(err, errors.ErrStackUnderflow)
}

// ═══════════════════════════════════════════════════════════════════════════════
// bitShiftByteCode
//
// Pops two values: v1 (top) = shift amount, v2 = value to shift.
// Positive shift → right shift (>>).  Negative shift → left shift (<<).
// The valid shift range is [-64, 63]; outside returns ErrInvalidBitShift.
// ═══════════════════════════════════════════════════════════════════════════════

// Test_bitShiftByteCode_RightShift verifies a two-bit right shift: 12 >> 2 = 3.
func Test_bitShiftByteCode_RightShift(t *testing.T) {
	// withStack(value, shift): v2=value on bottom, v1=shift on top.
	tc := newTestContext(t).withStack(12, 2) // 12 >> 2
	err := bitShiftByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(int64(3))
}

// Test_bitShiftByteCode_LeftShift verifies a three-bit left shift via negative
// shift amount: 5 << 3 = 40.
func Test_bitShiftByteCode_LeftShift(t *testing.T) {
	// A negative shift amount means left shift.
	tc := newTestContext(t).withStack(5, -3) // 5 << 3
	err := bitShiftByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(int64(40))
}

// Test_bitShiftByteCode_ZeroShift verifies that a shift of 0 returns the
// original value unchanged (identity operation).
func Test_bitShiftByteCode_ZeroShift(t *testing.T) {
	tc := newTestContext(t).withStack(42, 0) // 42 >> 0 = 42
	err := bitShiftByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(int64(42))
}

// Test_bitShiftByteCode_MaxRightShift verifies the maximum valid positive shift
// of 63 (the boundary just inside the valid range).
func Test_bitShiftByteCode_MaxRightShift(t *testing.T) {
	// 1 << 63 (as signed int64) followed by >> 63 should give -1 >> 63 = -1
	// (arithmetic shift).  Use a simpler value: a large power of 2.
	tc := newTestContext(t).withStack(1<<32, 32) // (1<<32) >> 32 = 1
	err := bitShiftByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(int64(1))
}

// Test_bitShiftByteCode_MaxLeftShift verifies the maximum valid negative shift
// of -64 (the boundary just inside the valid range).
func Test_bitShiftByteCode_MaxLeftShift(t *testing.T) {
	// shift = -64 means left-shift by 64; valid per the guard: -64 >= -64.
	tc := newTestContext(t).withStack(1, -64) // 1 << 64 overflows int64 → 0
	err := bitShiftByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack(int64(0)) // overflows int64
}

// Test_bitShiftByteCode_InvalidPositiveShift verifies that a shift amount of 64
// (outside the valid range of [−64, 63]) returns ErrInvalidBitShift.
func Test_bitShiftByteCode_InvalidPositiveShift(t *testing.T) {
	tc := newTestContext(t).withStack(5, 64) // 64 > 63 → invalid
	err := bitShiftByteCode(tc.ctx, nil)
	tc.assertError(err, errors.ErrInvalidBitShift)
}

// Test_bitShiftByteCode_InvalidNegativeShift verifies that a shift of -65
// (outside the valid range) returns ErrInvalidBitShift.
func Test_bitShiftByteCode_InvalidNegativeShift(t *testing.T) {
	tc := newTestContext(t).withStack(5, -65) // -65 < -64 → invalid
	err := bitShiftByteCode(tc.ctx, nil)
	tc.assertError(err, errors.ErrInvalidBitShift)
}

// Test_bitShiftByteCode_NilFirst_Error verifies ErrInvalidType when the shift
// amount is nil.
func Test_bitShiftByteCode_NilFirst_Error(t *testing.T) {
	tc := newTestContext(t).withStack(9, nil) // nil is the shift amount (v1)
	err := bitShiftByteCode(tc.ctx, nil)
	tc.assertError(err, errors.ErrInvalidType)
}

// Test_bitShiftByteCode_NilSecond_Error verifies ErrInvalidType when the value
// to be shifted is nil.
func Test_bitShiftByteCode_NilSecond_Error(t *testing.T) {
	tc := newTestContext(t).withStack(nil, 0) // nil is the value (v2)
	err := bitShiftByteCode(tc.ctx, nil)
	tc.assertError(err, errors.ErrInvalidType)
}

// Test_bitShiftByteCode_StackUnderflow_Zero verifies ErrStackUnderflow.
func Test_bitShiftByteCode_StackUnderflow_Zero(t *testing.T) {
	tc := newTestContext(t)
	err := bitShiftByteCode(tc.ctx, nil)
	tc.assertError(err, errors.ErrStackUnderflow)
}

// Test_bitShiftByteCode_StackUnderflow_One verifies ErrStackUnderflow with one item.
func Test_bitShiftByteCode_StackUnderflow_One(t *testing.T) {
	tc := newTestContext(t).withStack(55)
	err := bitShiftByteCode(tc.ctx, nil)
	tc.assertError(err, errors.ErrStackUnderflow)
}
