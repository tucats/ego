package bytecode

// argcheck_test.go contains unit tests for argCheckByteCode, the bytecode
// handler for the ArgCheck instruction.
//
// # What ArgCheck does
//
// ArgCheck validates that the number of arguments in the function's argument
// list (stored as "__args" in the symbol table) satisfies a minimum and
// maximum count.  It is emitted by the compiler at the top of every Ego
// function body.
//
// # Operand forms
//
// The instruction accepts four different operand types:
//
//	[]any{min, max}               – explicit range
//	[]any{min, max, name}         – range + function name (string or Token)
//	int n (n >= 0)                – exactly n arguments required
//	int n (n < 0)                 – 0..-n arguments allowed (variable length)
//	[]int{min, max}               – explicit integer range
//
// Anything else returns ErrArgumentTypeCheck.
//
// # How to read these tests
//
// Each test builds a testContext (see testhelpers_test.go), populates the
// argument list to simulate a function call, then calls argCheckByteCode
// directly and asserts the result.  Tests are grouped into sections that
// mirror the distinct code paths in argcheck.go.

import (
	"testing"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/tokenizer"
)

// ─── Section 1: []any operand, 2 elements ────────────────────────────────────

// Test_argCheckByteCode_SliceOp2_Exact verifies that exactly the required
// number of arguments passes when min == max.
func Test_argCheckByteCode_SliceOp2_Exact(t *testing.T) {
	tc := newTestContext(t).
		withArgList(1, 2)

	err := argCheckByteCode(tc.ctx, []any{2, 2})

	tc.assertNoError(err)
}

// Test_argCheckByteCode_SliceOp2_InRange verifies that an argument count
// within the min/max window passes.
func Test_argCheckByteCode_SliceOp2_InRange(t *testing.T) {
	tc := newTestContext(t).
		withArgList("a", "b", "c")

	err := argCheckByteCode(tc.ctx, []any{1, 5})

	tc.assertNoError(err)
}

// Test_argCheckByteCode_SliceOp2_Zero verifies that zero arguments is accepted
// when both min and max are 0.
func Test_argCheckByteCode_SliceOp2_Zero(t *testing.T) {
	tc := newTestContext(t).
		withArgList() // empty arg list

	err := argCheckByteCode(tc.ctx, []any{0, 0})

	tc.assertNoError(err)
}

// Test_argCheckByteCode_SliceOp2_TooFew verifies that having fewer arguments
// than the minimum produces ErrArgumentCount.
func Test_argCheckByteCode_SliceOp2_TooFew(t *testing.T) {
	tc := newTestContext(t).
		withArgList("only-one")

	err := argCheckByteCode(tc.ctx, []any{3, 5})

	tc.assertError(err, errors.ErrArgumentCount)
}

// Test_argCheckByteCode_SliceOp2_TooMany verifies that having more arguments
// than the maximum produces ErrArgumentCount.
func Test_argCheckByteCode_SliceOp2_TooMany(t *testing.T) {
	tc := newTestContext(t).
		withArgList(1, 2, 3, 4, 5)

	err := argCheckByteCode(tc.ctx, []any{1, 3})

	tc.assertError(err, errors.ErrArgumentCount)
}

// Test_argCheckByteCode_SliceOp2_MinEqualsMax verifies the boundary condition
// where the arg count equals both min and max (lower bound).
func Test_argCheckByteCode_SliceOp2_MinEqualsMax(t *testing.T) {
	tc := newTestContext(t).
		withArgList(42)

	err := argCheckByteCode(tc.ctx, []any{1, 1})

	tc.assertNoError(err)
}

// ─── Section 2: []any operand, 3 elements – string function name ─────────────

// Test_argCheckByteCode_SliceOp3_StringName verifies that the 3-element form
// sets c.module to the supplied function name string.
func Test_argCheckByteCode_SliceOp3_StringName(t *testing.T) {
	const funcName = "myFunc"

	tc := newTestContext(t).
		withArgList("x", "y")

	err := argCheckByteCode(tc.ctx, []any{2, 2, funcName})

	tc.assertNoError(err)

	// The side effect: c.module must be set to the function name.
	if tc.ctx.module != funcName {
		t.Errorf("c.module: got %q, want %q", tc.ctx.module, funcName)
	}
}

// Test_argCheckByteCode_SliceOp3_StringName_TooFew verifies that the 3-element
// form still enforces the count even when a function name is provided.
func Test_argCheckByteCode_SliceOp3_StringName_TooFew(t *testing.T) {
	tc := newTestContext(t).
		withArgList() // no args

	err := argCheckByteCode(tc.ctx, []any{1, 2, "needsArgs"})

	tc.assertError(err, errors.ErrArgumentCount)
}

// Test_argCheckByteCode_SliceOp3_StringName_TooMany verifies ErrArgumentCount
// when too many args are supplied in the 3-element form.
func Test_argCheckByteCode_SliceOp3_StringName_TooMany(t *testing.T) {
	tc := newTestContext(t).
		withArgList(1, 2, 3)

	err := argCheckByteCode(tc.ctx, []any{0, 2, "twoArgMax"})

	tc.assertError(err, errors.ErrArgumentCount)
}

// ─── Section 3: []any operand, 3 elements – tokenizer.Token function name ────

// Test_argCheckByteCode_SliceOp3_TokenName verifies that when the third
// element is a tokenizer.Token its Spelling() is used as the function name
// and stored in c.module.
func Test_argCheckByteCode_SliceOp3_TokenName(t *testing.T) {
	tc := newTestContext(t).
		withArgList(100)

	tok := tokenizer.NewIdentifierToken("parseNum")

	err := argCheckByteCode(tc.ctx, []any{1, 1, tok})

	tc.assertNoError(err)

	if tc.ctx.module != "parseNum" {
		t.Errorf("c.module: got %q, want %q", tc.ctx.module, "parseNum")
	}
}

// Test_argCheckByteCode_SliceOp3_NonStringName verifies that a third element
// that is neither a string nor a Token is still accepted (data.String()
// converts it), setting c.module to whatever data.String produces.
func Test_argCheckByteCode_SliceOp3_NonStringName(t *testing.T) {
	tc := newTestContext(t).
		withArgList(1)

	// An integer as the name element: data.String(42) == "42".
	err := argCheckByteCode(tc.ctx, []any{1, 1, 42})

	tc.assertNoError(err)

	if tc.ctx.module != "42" {
		t.Errorf("c.module: got %q, want %q", tc.ctx.module, "42")
	}
}

// ─── Section 4: []any operand – invalid element counts ───────────────────────

// Test_argCheckByteCode_SliceOp1_TooShort verifies that a 1-element []any
// operand is rejected with ErrArgumentTypeCheck.
func Test_argCheckByteCode_SliceOp1_TooShort(t *testing.T) {
	tc := newTestContext(t).
		withArgList(1)

	err := argCheckByteCode(tc.ctx, []any{2})

	tc.assertError(err, errors.ErrArgumentTypeCheck)
}

// Test_argCheckByteCode_SliceOp4_TooLong verifies that a 4-element []any
// operand is rejected with ErrArgumentTypeCheck.
func Test_argCheckByteCode_SliceOp4_TooLong(t *testing.T) {
	tc := newTestContext(t).
		withArgList(1)

	err := argCheckByteCode(tc.ctx, []any{0, 2, "fn", "extra"})

	tc.assertError(err, errors.ErrArgumentTypeCheck)
}

// ─── Section 5: int operand ───────────────────────────────────────────────────

// Test_argCheckByteCode_IntOp_ExactMatch verifies that a positive int operand
// accepts exactly that many arguments.
func Test_argCheckByteCode_IntOp_ExactMatch(t *testing.T) {
	tc := newTestContext(t).
		withArgList("a", "b", "c")

	err := argCheckByteCode(tc.ctx, 3)

	tc.assertNoError(err)
}

// Test_argCheckByteCode_IntOp_Zero verifies that an operand of 0 accepts zero
// arguments and rejects any non-empty argument list.
func Test_argCheckByteCode_IntOp_Zero(t *testing.T) {
	tc := newTestContext(t).
		withArgList() // empty

	err := argCheckByteCode(tc.ctx, 0)

	tc.assertNoError(err)
}

// Test_argCheckByteCode_IntOp_TooFew verifies ErrArgumentCount when fewer
// arguments than the positive int operand are present.
func Test_argCheckByteCode_IntOp_TooFew(t *testing.T) {
	tc := newTestContext(t).
		withArgList("only-one")

	err := argCheckByteCode(tc.ctx, 3)

	tc.assertError(err, errors.ErrArgumentCount)
}

// Test_argCheckByteCode_IntOp_TooMany verifies ErrArgumentCount when more
// arguments than the positive int operand are present.
func Test_argCheckByteCode_IntOp_TooMany(t *testing.T) {
	tc := newTestContext(t).
		withArgList(1, 2, 3, 4)

	err := argCheckByteCode(tc.ctx, 2)

	tc.assertError(err, errors.ErrArgumentCount)
}

// Test_argCheckByteCode_IntOp_ZeroWithArgs verifies ErrArgumentCount when
// the operand is 0 (no args expected) but some args are present.
func Test_argCheckByteCode_IntOp_ZeroWithArgs(t *testing.T) {
	tc := newTestContext(t).
		withArgList("unexpected")

	err := argCheckByteCode(tc.ctx, 0)

	tc.assertError(err, errors.ErrArgumentCount)
}

// Test_argCheckByteCode_IntOp_Negative_WithinRange verifies that a negative
// int operand implements "variable args" semantics: min=0, max=-operand.
// Passing 2 arguments when the operand is -5 (max 5) should succeed.
func Test_argCheckByteCode_IntOp_Negative_WithinRange(t *testing.T) {
	tc := newTestContext(t).
		withArgList(1, 2)

	// -5 means accept 0..5 arguments.
	err := argCheckByteCode(tc.ctx, -5)

	tc.assertNoError(err)
}

// Test_argCheckByteCode_IntOp_Negative_Zero verifies that zero arguments is
// accepted when the operand is negative (min is always 0 for variadic).
func Test_argCheckByteCode_IntOp_Negative_Zero(t *testing.T) {
	tc := newTestContext(t).
		withArgList()

	err := argCheckByteCode(tc.ctx, -3)

	tc.assertNoError(err)
}

// Test_argCheckByteCode_IntOp_Negative_TooMany verifies ErrArgumentCount when
// a negative int operand's absolute value is exceeded.
func Test_argCheckByteCode_IntOp_Negative_TooMany(t *testing.T) {
	tc := newTestContext(t).
		withArgList(1, 2, 3, 4)

	// -2 means accept 0..2; passing 4 exceeds the max.
	err := argCheckByteCode(tc.ctx, -2)

	tc.assertError(err, errors.ErrArgumentCount)
}

// ─── Section 6: []int operand ─────────────────────────────────────────────────

// Test_argCheckByteCode_IntSlice_InRange verifies the basic []int{min, max}
// happy path.
func Test_argCheckByteCode_IntSlice_InRange(t *testing.T) {
	tc := newTestContext(t).
		withArgList("a", "b")

	err := argCheckByteCode(tc.ctx, []int{1, 3})

	tc.assertNoError(err)
}

// Test_argCheckByteCode_IntSlice_ExactMin verifies that min == max == count
// passes.
func Test_argCheckByteCode_IntSlice_ExactMin(t *testing.T) {
	tc := newTestContext(t).
		withArgList("only")

	err := argCheckByteCode(tc.ctx, []int{1, 1})

	tc.assertNoError(err)
}

// Test_argCheckByteCode_IntSlice_TooFew verifies ErrArgumentCount when count
// < min.
func Test_argCheckByteCode_IntSlice_TooFew(t *testing.T) {
	tc := newTestContext(t).
		withArgList()

	err := argCheckByteCode(tc.ctx, []int{2, 4})

	tc.assertError(err, errors.ErrArgumentCount)
}

// Test_argCheckByteCode_IntSlice_TooMany verifies ErrArgumentCount when count
// > max.
func Test_argCheckByteCode_IntSlice_TooMany(t *testing.T) {
	tc := newTestContext(t).
		withArgList(1, 2, 3, 4, 5)

	err := argCheckByteCode(tc.ctx, []int{0, 2})

	tc.assertError(err, errors.ErrArgumentCount)
}

// Test_argCheckByteCode_IntSlice_WrongLen1 verifies that a 1-element []int
// operand is rejected.
func Test_argCheckByteCode_IntSlice_WrongLen1(t *testing.T) {
	tc := newTestContext(t).
		withArgList(1)

	err := argCheckByteCode(tc.ctx, []int{2})

	tc.assertError(err, errors.ErrArgumentTypeCheck)
}

// Test_argCheckByteCode_IntSlice_WrongLen3 verifies that a 3-element []int
// operand is rejected.
func Test_argCheckByteCode_IntSlice_WrongLen3(t *testing.T) {
	tc := newTestContext(t).
		withArgList(1)

	err := argCheckByteCode(tc.ctx, []int{1, 2, 3})

	tc.assertError(err, errors.ErrArgumentTypeCheck)
}

// ─── Section 7: Invalid operand types ────────────────────────────────────────

// Test_argCheckByteCode_NilOperand verifies that nil is rejected.
func Test_argCheckByteCode_NilOperand(t *testing.T) {
	tc := newTestContext(t).
		withArgList(1)

	err := argCheckByteCode(tc.ctx, nil)

	tc.assertError(err, errors.ErrArgumentTypeCheck)
}

// Test_argCheckByteCode_StringOperand verifies that a plain string operand
// (not wrapped in a slice) is rejected.
func Test_argCheckByteCode_StringOperand(t *testing.T) {
	tc := newTestContext(t).
		withArgList(1)

	err := argCheckByteCode(tc.ctx, "bad-operand")

	tc.assertError(err, errors.ErrArgumentTypeCheck)
}

// Test_argCheckByteCode_Float64Operand verifies that a float64 operand
// is rejected.
func Test_argCheckByteCode_Float64Operand(t *testing.T) {
	tc := newTestContext(t).
		withArgList(1)

	err := argCheckByteCode(tc.ctx, 2.0)

	tc.assertError(err, errors.ErrArgumentTypeCheck)
}

// Test_argCheckByteCode_DataListOperand verifies that a data.List operand
// is rejected; ArgCheck uses []any, not data.List.
func Test_argCheckByteCode_DataListOperand(t *testing.T) {
	tc := newTestContext(t).
		withArgList(1)

	err := argCheckByteCode(tc.ctx, data.NewList(1, 2))

	tc.assertError(err, errors.ErrArgumentTypeCheck)
}

// ─── Section 8: Missing or invalid __args ────────────────────────────────────

// Test_argCheckByteCode_NoArgListSymbol verifies that ErrArgumentTypeCheck is
// returned when __args is absent from the symbol table.  This would occur if
// ArgCheck is executed outside a function-call context.
func Test_argCheckByteCode_NoArgListSymbol(t *testing.T) {
	// withArgList is intentionally NOT called.
	tc := newTestContext(t)

	err := argCheckByteCode(tc.ctx, 0)

	tc.assertError(err, errors.ErrArgumentTypeCheck)
}

// Test_argCheckByteCode_ArgListNotArray verifies that ErrArgumentTypeCheck is
// returned when __args exists but is not a *data.Array.
func Test_argCheckByteCode_ArgListNotArray(t *testing.T) {
	tc := newTestContext(t)

	// Manually corrupt __args with a non-array value.
	tc.ctx.symbols.SetAlways("__args", "not-an-array")

	err := argCheckByteCode(tc.ctx, 0)

	tc.assertError(err, errors.ErrArgumentTypeCheck)
}

// ─── Section 9: Variable-length args via negative maxArgCount ────────────────

// Test_argCheckByteCode_NegativeMax_ViaSlice verifies that a negative max
// supplied in the []any form causes ArgCheck to accept any number of arguments
// (because it sets maxArgCount = array.Len()).
func Test_argCheckByteCode_NegativeMax_ViaSlice(t *testing.T) {
	tc := newTestContext(t).
		withArgList(1, 2, 3, 4, 5, 6, 7)

	// min=0, max=-1 → after adjustment, max = len(args) = 7 → always passes.
	err := argCheckByteCode(tc.ctx, []any{0, -1})

	tc.assertNoError(err)
}

// Test_argCheckByteCode_NegativeMax_ViaIntSlice verifies the same negative-max
// behavior via the []int operand form.
func Test_argCheckByteCode_NegativeMax_ViaIntSlice(t *testing.T) {
	tc := newTestContext(t).
		withArgList(1, 2, 3)

	// min=1, max=-1 → max becomes len(args)=3 → passes (1 ≤ 3 ≤ 3).
	err := argCheckByteCode(tc.ctx, []int{1, -1})

	tc.assertNoError(err)
}

// ─── Section 10: Recursive function registration side effect ─────────────────
//
// When the 3-element []any form is used and a symbol with the function name
// already exists in a parent scope, argCheckByteCode copies it into the
// current local scope to ensure recursive calls can find the function.

// Test_argCheckByteCode_RecursiveRegistration_DataFunction verifies that a
// data.Function stored in a parent scope is registered in the local scope when
// its name matches the function name in the operand.
func Test_argCheckByteCode_RecursiveRegistration_DataFunction(t *testing.T) {
	tc := newTestContext(t).
		withArgList(1)

	// Store a data.Function in the root (parent) table under "recurse".
	fn := data.Function{
		Declaration: &data.Declaration{Name: "recurse"},
		Value:       &ByteCode{name: "recurse"},
	}
	tc.ctx.symbols.Root().SetAlways("recurse", fn)

	err := argCheckByteCode(tc.ctx, []any{1, 1, "recurse"})

	tc.assertNoError(err)

	// The function should now be accessible from the local scope directly.
	v, found := tc.ctx.symbols.Get("recurse")
	if !found {
		t.Fatal("recursive function not found in local symbol table after ArgCheck")
	}

	if _, ok := v.(data.Function); !ok {
		t.Errorf("expected data.Function in local table, got %T", v)
	}
}

// Test_argCheckByteCode_RecursiveRegistration_ByteCode verifies that a
// *ByteCode value with a non-nil declaration stored in a parent scope is
// wrapped in a data.Function and registered locally.
func Test_argCheckByteCode_RecursiveRegistration_ByteCode(t *testing.T) {
	tc := newTestContext(t).
		withArgList(1)

	decl := &data.Declaration{Name: "compute"}
	bc := &ByteCode{
		name:        "compute",
		declaration: decl,
	}
	tc.ctx.symbols.Root().SetAlways("compute", bc)

	err := argCheckByteCode(tc.ctx, []any{1, 1, "compute"})

	tc.assertNoError(err)

	v, found := tc.ctx.symbols.Get("compute")
	if !found {
		t.Fatal("recursive function not found in local symbol table after ArgCheck")
	}

	fn, ok := v.(data.Function)
	if !ok {
		t.Fatalf("expected data.Function in local table, got %T", v)
	}

	if fn.Declaration == nil || fn.Declaration.Name != "compute" {
		t.Errorf("function declaration mismatch: got %v", fn.Declaration)
	}
}

// Test_argCheckByteCode_RecursiveRegistration_NilDeclaration verifies that a
// *ByteCode with a nil declaration is NOT registered (fd.Value stays nil, so
// the Create/Set block is skipped).
func Test_argCheckByteCode_RecursiveRegistration_NilDeclaration(t *testing.T) {
	tc := newTestContext(t).
		withArgList(1)

	bc := &ByteCode{name: "noDecl"} // declaration is nil
	tc.ctx.symbols.Root().SetAlways("noDecl", bc)

	err := argCheckByteCode(tc.ctx, []any{1, 1, "noDecl"})

	tc.assertNoError(err)

	// Symbol should NOT have been copied into the local scope.
	_, found := tc.ctx.symbols.GetLocal("noDecl")
	if found {
		t.Errorf("expected *ByteCode with nil declaration NOT to be registered locally")
	}
}
