package bytecode

// arg_test.go contains unit tests for argByteCode, the bytecode handler for
// the Arg instruction.
//
// # What Arg does
//
// The Arg instruction extracts a positional argument from the function's
// argument list (stored in the symbol table as "__args") and assigns it to a
// named local variable.  The operand is either a data.List or a []any slice
// with two or three elements:
//
//	[index, name]        – no type check; store args[index] as name
//	[index, name, type]  – validate type, coerce if needed, then store
//
// # How to read these tests
//
// Each test builds a testContext (see testhelpers_test.go), populates the
// argument list and, where relevant, a pre-existing symbol, then calls
// argByteCode directly.  After the call it asserts either that a specific
// error was returned OR that the named symbol has the expected value.
//
// The tests are grouped into sections that mirror the code paths in arg.go:
//  1. Happy-path cases (data.List operand, 2 elements)
//  2. Happy-path cases ([]any operand, 2 elements)
//  3. 3-element operand variants (with explicit type)
//  4. Type coercion via data.List operand
//  5. Error cases – bad operand shape or type
//  6. Error cases – missing or invalid argument list
//  7. Error cases – index out of range

import (
	"testing"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
)

// ─── Section 1: data.List operand, 2 elements (no type validation) ───────────

// Test_argByteCode_DataList2_Int verifies the fundamental happy path: extract
// argument 0 from the list and store it in a named symbol.
func Test_argByteCode_DataList2_Int(t *testing.T) {
	tc := newTestContext(t).
		withArgList(42)

	err := argByteCode(tc.ctx, data.NewList(0, "x"))

	tc.assertNoError(err)
	tc.assertSymbolValue("x", 42)
}

// Test_argByteCode_DataList2_String verifies that a string argument is stored
// without modification when no type is specified.
func Test_argByteCode_DataList2_String(t *testing.T) {
	tc := newTestContext(t).
		withArgList("hello", "world")

	// Extract the second argument (index 1) and store it as "greeting".
	err := argByteCode(tc.ctx, data.NewList(1, "greeting"))

	tc.assertNoError(err)
	tc.assertSymbolValue("greeting", "world")
}

// Test_argByteCode_DataList2_Bool verifies that a boolean argument is stored
// correctly via a data.List operand.
func Test_argByteCode_DataList2_Bool(t *testing.T) {
	tc := newTestContext(t).
		withArgList(true)

	err := argByteCode(tc.ctx, data.NewList(0, "flag"))

	tc.assertNoError(err)
	tc.assertSymbolValue("flag", true)
}

// Test_argByteCode_DataList2_Nil verifies that a nil argument is forwarded
// without error when no type is specified.
func Test_argByteCode_DataList2_Nil(t *testing.T) {
	tc := newTestContext(t).
		withArgList(nil)

	err := argByteCode(tc.ctx, data.NewList(0, "v"))

	tc.assertNoError(err)
	tc.assertSymbolValue("v", nil)
}

// Test_argByteCode_DataList2_Float64 verifies that a float64 argument is
// stored without modification when no type is specified.
func Test_argByteCode_DataList2_Float64(t *testing.T) {
	tc := newTestContext(t).
		withArgList(3.14)

	err := argByteCode(tc.ctx, data.NewList(0, "pi"))

	tc.assertNoError(err)
	tc.assertSymbolValue("pi", 3.14)
}

// Test_argByteCode_DataList2_SecondArg verifies that the correct argument is
// selected when multiple arguments are present and the index is non-zero.
func Test_argByteCode_DataList2_SecondArg(t *testing.T) {
	tc := newTestContext(t).
		withArgList("first", "second", "third")

	err := argByteCode(tc.ctx, data.NewList(2, "last"))

	tc.assertNoError(err)
	tc.assertSymbolValue("last", "third")
}

// ─── Section 2: []any operand, 2 elements (no type validation) ───────────────

// Test_argByteCode_SliceOp2_Int mirrors Test_argByteCode_DataList2_Int but
// uses the []any form of the operand.  Both forms must work identically
// because the compiler may emit either one depending on context.
func Test_argByteCode_SliceOp2_Int(t *testing.T) {
	tc := newTestContext(t).
		withArgList(99)

	err := argByteCode(tc.ctx, []any{0, "num"})

	tc.assertNoError(err)
	tc.assertSymbolValue("num", 99)
}

// Test_argByteCode_SliceOp2_String verifies string storage via the []any
// operand form.
func Test_argByteCode_SliceOp2_String(t *testing.T) {
	tc := newTestContext(t).
		withArgList("alpha", "beta")

	err := argByteCode(tc.ctx, []any{1, "letter"})

	tc.assertNoError(err)
	tc.assertSymbolValue("letter", "beta")
}

// ─── Section 3: 3-element operand with explicit type (data.List) ─────────────

// Test_argByteCode_DataList3_IntType verifies that an int argument passes
// an explicit data.IntType type check without needing coercion.
func Test_argByteCode_DataList3_IntType(t *testing.T) {
	tc := newTestContext(t).
		withArgList(7)

	err := argByteCode(tc.ctx, data.NewList(0, "n", data.IntType))

	tc.assertNoError(err)
	tc.assertSymbolValue("n", 7)
}

// Test_argByteCode_DataList3_StringType verifies that a string argument passes
// an explicit data.StringType type check.
func Test_argByteCode_DataList3_StringType(t *testing.T) {
	tc := newTestContext(t).
		withArgList("ego")

	err := argByteCode(tc.ctx, data.NewList(0, "lang", data.StringType))

	tc.assertNoError(err)
	tc.assertSymbolValue("lang", "ego")
}

// Test_argByteCode_DataList3_BoolType verifies that a bool argument passes an
// explicit data.BoolType type check.
func Test_argByteCode_DataList3_BoolType(t *testing.T) {
	tc := newTestContext(t).
		withArgList(true)

	err := argByteCode(tc.ctx, data.NewList(0, "ok", data.BoolType))

	tc.assertNoError(err)
	tc.assertSymbolValue("ok", true)
}

// Test_argByteCode_DataList3_InterfaceType verifies that any value passes an
// interface{} type check, since interface{} accepts all types.  The runtime
// wraps the concrete value in a data.Interface struct so that the Ego type
// system can track the concrete type alongside the value.
func Test_argByteCode_DataList3_InterfaceType(t *testing.T) {
	tc := newTestContext(t).
		withArgList(42)

	err := argByteCode(tc.ctx, data.NewList(0, "v", data.InterfaceType))

	tc.assertNoError(err)
	// The stored symbol holds a data.Interface wrapper, not the raw int.
	tc.assertSymbolValue("v", data.Interface{Value: 42, BaseType: data.IntType})
}

// Test_argByteCode_SliceOp3_IntType mirrors Test_argByteCode_DataList3_IntType
// using the []any form of the operand.
func Test_argByteCode_SliceOp3_IntType(t *testing.T) {
	tc := newTestContext(t).
		withArgList(55)

	err := argByteCode(tc.ctx, []any{0, "z", data.IntType})

	tc.assertNoError(err)
	tc.assertSymbolValue("z", 55)
}

// ─── Section 4: Type coercion (data.List, 3 elements) ────────────────────────
//
// When the target type is specified and data.IsCoercible returns true,
// argByteCode coerces the value after it passes the required-type check.
// These tests verify that coercion produces the expected Go types.

// Test_argByteCode_Coerce_IntToFloat64 verifies that an int argument is
// coerced to float64 when the operand specifies data.Float64Type.
func Test_argByteCode_Coerce_IntToFloat64(t *testing.T) {
	tc := newTestContext(t).
		withArgList(3)

	err := argByteCode(tc.ctx, data.NewList(0, "f", data.Float64Type))

	tc.assertNoError(err)
	tc.assertSymbolValue("f", float64(3))
}

// Test_argByteCode_Coerce_Float64ToInt verifies that a float64 argument is
// coerced to int when the operand specifies data.IntType.
func Test_argByteCode_Coerce_Float64ToInt(t *testing.T) {
	tc := newTestContext(t).
		withArgList(float64(7.0))

	err := argByteCode(tc.ctx, data.NewList(0, "n", data.IntType))

	tc.assertNoError(err)
	tc.assertSymbolValue("n", 7)
}

// Test_argByteCode_Coerce_IntToString verifies that an integer argument is
// coerced to a string representation when the operand specifies
// data.StringType.
func Test_argByteCode_Coerce_IntToString(t *testing.T) {
	tc := newTestContext(t).
		withArgList(123)

	err := argByteCode(tc.ctx, data.NewList(0, "s", data.StringType))

	tc.assertNoError(err)
	tc.assertSymbolValue("s", "123")
}

// Test_argByteCode_Coerce_StringToInt verifies that a numeric string argument
// is coerced to int when data.IntType is specified.
func Test_argByteCode_Coerce_StringToInt(t *testing.T) {
	tc := newTestContext(t).
		withArgList("42")

	err := argByteCode(tc.ctx, data.NewList(0, "n", data.IntType))

	tc.assertNoError(err)
	tc.assertSymbolValue("n", 42)
}

// ─── Section 5: Error cases – bad operand shape or type ──────────────────────

// Test_argByteCode_BadOperandType verifies that passing an operand that is
// neither a data.List nor a []any produces ErrInvalidOperand.
func Test_argByteCode_BadOperandType(t *testing.T) {
	tc := newTestContext(t).
		withArgList(1)

	// Passing a plain string is an invalid operand type.
	err := argByteCode(tc.ctx, "bad-operand")

	tc.assertError(err, errors.ErrInvalidOperand)
}

// Test_argByteCode_BadOperandType_Int verifies that an integer operand (not
// a list or slice) is rejected.
func Test_argByteCode_BadOperandType_Int(t *testing.T) {
	tc := newTestContext(t).
		withArgList(1)

	err := argByteCode(tc.ctx, 99)

	tc.assertError(err, errors.ErrInvalidOperand)
}

// Test_argByteCode_DataList1Element verifies that a 1-element data.List is
// rejected because argByteCode requires at least 2 elements (index + name).
func Test_argByteCode_DataList1Element(t *testing.T) {
	tc := newTestContext(t).
		withArgList(1)

	err := argByteCode(tc.ctx, data.NewList(0))

	tc.assertError(err, errors.ErrInvalidOperand)
}

// Test_argByteCode_DataList4Elements verifies that a 4-element data.List is
// rejected because argByteCode accepts at most 3 elements.
func Test_argByteCode_DataList4Elements(t *testing.T) {
	tc := newTestContext(t).
		withArgList(1)

	err := argByteCode(tc.ctx, data.NewList(0, "x", data.IntType, "extra"))

	tc.assertError(err, errors.ErrInvalidOperand)
}

// Test_argByteCode_Slice1Element mirrors Test_argByteCode_DataList1Element for
// the []any operand form.
func Test_argByteCode_Slice1Element(t *testing.T) {
	tc := newTestContext(t).
		withArgList(1)

	err := argByteCode(tc.ctx, []any{0})

	tc.assertError(err, errors.ErrInvalidOperand)
}

// Test_argByteCode_Slice4Elements mirrors Test_argByteCode_DataList4Elements
// for the []any operand form.
func Test_argByteCode_Slice4Elements(t *testing.T) {
	tc := newTestContext(t).
		withArgList(1)

	err := argByteCode(tc.ctx, []any{0, "x", data.IntType, "extra"})

	tc.assertError(err, errors.ErrInvalidOperand)
}

// ─── Section 6: Error cases – missing or invalid argument list ───────────────

// Test_argByteCode_NoArgListSymbol verifies that ErrInvalidArgumentList is
// returned when __args is not present in the symbol table.  This would happen
// if the Arg instruction is executed outside a function call context.
func Test_argByteCode_NoArgListSymbol(t *testing.T) {
	// withArgList is intentionally NOT called, so __args is absent.
	tc := newTestContext(t)

	err := argByteCode(tc.ctx, data.NewList(0, "x"))

	tc.assertError(err, errors.ErrInvalidArgumentList)
}

// Test_argByteCode_ArgListNotArray verifies that ErrInvalidArgumentList is
// returned when __args exists but holds a value that is not a *data.Array.
// This is a defensive check against corrupted symbol table state.
func Test_argByteCode_ArgListNotArray(t *testing.T) {
	tc := newTestContext(t)

	// Store a plain string instead of a *data.Array under __args.
	tc.ctx.symbols.SetAlways(defs.ArgumentListVariable, "not-an-array")

	err := argByteCode(tc.ctx, data.NewList(0, "x"))

	tc.assertError(err, errors.ErrInvalidArgumentList)
}

// ─── Section 7: Error cases – index out of range ─────────────────────────────

// Test_argByteCode_IndexOutOfRange verifies that requesting an argument at an
// index beyond the end of the list returns ErrInvalidArgumentList.  For
// example, asking for index 3 when only 2 arguments were passed.
func Test_argByteCode_IndexOutOfRange(t *testing.T) {
	// Provide 2 arguments (indices 0 and 1).
	tc := newTestContext(t).
		withArgList("a", "b")

	// Request index 5, which is beyond the end of the argument array.
	err := argByteCode(tc.ctx, data.NewList(5, "x"))

	tc.assertError(err, errors.ErrInvalidArgumentList)
}

// Test_argByteCode_IndexOutOfRange_Slice mirrors the out-of-range test using
// the []any operand form.
func Test_argByteCode_IndexOutOfRange_Slice(t *testing.T) {
	tc := newTestContext(t).
		withArgList("only-one")

	err := argByteCode(tc.ctx, []any{10, "x"})

	tc.assertError(err, errors.ErrInvalidArgumentList)
}

// ─── Section 8: Stack is left clean ──────────────────────────────────────────
//
// argByteCode pushes the value temporarily onto the stack while performing
// the required-type check, but it pops it back off before storing it in the
// symbol table.  After a successful call the stack must be empty.

// Test_argByteCode_StackIsClean_NoType verifies that the stack is empty after
// a successful 2-element operand (no type check path).
func Test_argByteCode_StackIsClean_NoType(t *testing.T) {
	tc := newTestContext(t).
		withArgList(100)

	err := argByteCode(tc.ctx, data.NewList(0, "n"))

	tc.assertNoError(err)
	tc.assertStackEmpty()
}

// Test_argByteCode_StackIsClean_WithType verifies that the stack is empty
// after a successful 3-element operand (type-check path).
func Test_argByteCode_StackIsClean_WithType(t *testing.T) {
	tc := newTestContext(t).
		withArgList(200)

	err := argByteCode(tc.ctx, data.NewList(0, "n", data.IntType))

	tc.assertNoError(err)
	tc.assertStackEmpty()
}

// Test_argByteCode_StackIsClean_WithCoercion verifies that the stack is empty
// after a successful call that required type coercion.
func Test_argByteCode_StackIsClean_WithCoercion(t *testing.T) {
	tc := newTestContext(t).
		withArgList(5)

	err := argByteCode(tc.ctx, data.NewList(0, "f", data.Float64Type))

	tc.assertNoError(err)
	tc.assertStackEmpty()
}

// ─── Section 9: Existing stack items are not disturbed ───────────────────────
//
// The Arg instruction must not consume items that were already on the stack
// before it ran; it uses only __args (the symbol table) as its source.

// Test_argByteCode_ExistingStackUntouched verifies that items already on the
// stack before Arg runs are still there afterwards, in the correct order.
func Test_argByteCode_ExistingStackUntouched(t *testing.T) {
	// Push two sentinel values onto the stack before calling Arg.
	tc := newTestContext(t).
		withStack("bottom", "top").
		withArgList(99)

	err := argByteCode(tc.ctx, data.NewList(0, "n"))

	tc.assertNoError(err)

	// The two original stack items must still be there, unmodified.
	tc.assertTopStack("top")
	tc.assertTopStack("bottom")
}

// ─── Section 10: Strict vs dynamic type-checking mode ────────────────────────
//
// argByteCode validates the argument type when a 3-element operand supplies a
// target type.  The validation uses requiredTypeByteCode, whose behavior is
// governed by the context's typeStrictness setting:
//
//   - StrictTypeEnforcement: the argument must already be the declared type;
//     a mismatch returns ErrTypeMismatch.
//   - RelaxedTypeEnforcement / NoTypeEnforcement (default): coercions are
//     permitted, so an int argument can satisfy a float64 parameter, etc.
//
// All tests in the previous sections run with the default NoTypeEnforcement
// (newTestContext does not set TypeCheckingVariable).  The tests below
// explicitly exercise both strict and dynamic modes to ensure each path
// through requiredTypeByteCode is covered.

// Test_argByteCode_StrictMode_MatchingType_Passes verifies that in strict mode
// an argument whose type exactly matches the declared parameter type is
// accepted without error and stored correctly.
func Test_argByteCode_StrictMode_MatchingType_Passes(t *testing.T) {
	tc := newTestContext(t).
		withTypeStrictness(defs.StrictTypeEnforcement).
		withArgList(42) // int arg for int parameter — exact match

	err := argByteCode(tc.ctx, data.NewList(0, "n", data.IntType))

	tc.assertNoError(err)
	tc.assertSymbolValue("n", 42)
}

// Test_argByteCode_StrictMode_MismatchedType_Fails verifies that in strict
// mode an argument of the wrong type returns ErrArgumentType.  Strict mode
// does not attempt coercion; the concrete types must match exactly.
//
// Implementation note: the error originates in strictConformanceCheck
// (bytecode/types.go) which returns ErrArgumentType — not ErrTypeMismatch —
// when the actual type does not satisfy the declared parameter type.
// argByteCode then wraps it with the argument position as context.
func Test_argByteCode_StrictMode_MismatchedType_Fails(t *testing.T) {
	tc := newTestContext(t).
		withTypeStrictness(defs.StrictTypeEnforcement).
		withArgList("hello") // string arg for int parameter — type mismatch

	err := argByteCode(tc.ctx, data.NewList(0, "n", data.IntType))

	tc.assertError(err, errors.ErrArgumentType)
}

// Test_argByteCode_StrictMode_StringMatchesStringType verifies that a string
// argument passes a StringType annotation in strict mode.
func Test_argByteCode_StrictMode_StringMatchesStringType(t *testing.T) {
	tc := newTestContext(t).
		withTypeStrictness(defs.StrictTypeEnforcement).
		withArgList("hello")

	err := argByteCode(tc.ctx, data.NewList(0, "s", data.StringType))

	tc.assertNoError(err)
	tc.assertSymbolValue("s", "hello")
}

// Test_argByteCode_DynamicMode_CoercesIntToFloat64 verifies that the default
// NoTypeEnforcement mode allows numeric coercion: an int argument is
// successfully coerced to float64 when that is the declared type.
func Test_argByteCode_DynamicMode_CoercesIntToFloat64(t *testing.T) {
	// newTestContext uses NoTypeEnforcement by default — no call to
	// withTypeStrictness is needed, but we include it explicitly here so
	// the test is self-documenting about which mode it is testing.
	tc := newTestContext(t).
		withTypeStrictness(defs.NoTypeEnforcement).
		withArgList(3)

	err := argByteCode(tc.ctx, data.NewList(0, "f", data.Float64Type))

	tc.assertNoError(err)
	tc.assertSymbolValue("f", float64(3))
}

// Test_argByteCode_RelaxedMode_CoercesIntToFloat64 verifies that
// RelaxedTypeEnforcement also allows numeric widening coercions, consistent
// with the documented behaviour that relaxed mode permits integer↔float
// conversions.
func Test_argByteCode_RelaxedMode_CoercesIntToFloat64(t *testing.T) {
	tc := newTestContext(t).
		withTypeStrictness(defs.RelaxedTypeEnforcement).
		withArgList(5)

	err := argByteCode(tc.ctx, data.NewList(0, "f", data.Float64Type))

	tc.assertNoError(err)
	tc.assertSymbolValue("f", float64(5))
}
