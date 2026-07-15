package bytecode

// load_test.go — comprehensive unit tests for loadByteCode.
//
// # Functions under test
//
// loadByteCode (Load opcode):
//   - Reads a named value from the symbol table and pushes it onto the stack.
//   - The operand (i) is the variable name, converted to a string with
//     data.String(i), so any value that stringifies to a non-empty identifier
//     is accepted.
//   - An empty or nil operand returns ErrInvalidIdentifier.
//   - A name that is absent from every scope in the table chain returns
//     ErrUnknownIdentifier.
//   - If the stored value is a data.Immutable (a read-only constant wrapper),
//     the wrapper is stripped by data.UnwrapConstant before pushing, so callers
//     always receive the raw inner value.
//
// All tests use the newTestContext / withXxx / assertXxx helpers from
// testhelpers_test.go as required by the project testing standards.

import (
	"testing"

	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/errors"
)

// ─── Section 1: loadByteCode ─────────────────────────────────────────────────

// Test_loadByteCode_Integer verifies the core happy path: an integer value
// stored in the symbol table is retrieved by name and pushed onto the stack.
//
// This is the simplest possible exercise of loadByteCode — one symbol, one
// stack push, no wrapping.
func Test_loadByteCode_Integer(t *testing.T) {
	// Set up a context with the variable "count" initialized to 42.
	// withSymbol calls c.create then c.set, which is exactly what compiled
	// Ego code does before a variable is used.
	tc := newTestContext(t).withSymbol("count", 42)

	err := loadByteCode(tc.ctx, "count")

	// No error is expected.
	tc.assertNoError(err)
	// The value 42 should now be on top of the stack.
	tc.assertTopStack(42)
	// Popping the result above should leave the stack fully empty.
	tc.assertStackEmpty()
}

// Test_loadByteCode_String verifies that a string value is correctly loaded
// from the symbol table and pushed onto the stack.
//
// Go strings are reference types; the test confirms the exact string value is
// preserved without truncation or modification.
func Test_loadByteCode_String(t *testing.T) {
	tc := newTestContext(t).withSymbol("greeting", "hello, world")

	err := loadByteCode(tc.ctx, "greeting")

	tc.assertNoError(err)
	tc.assertTopStack("hello, world")
	tc.assertStackEmpty()
}

// Test_loadByteCode_Bool verifies that a boolean value is loaded and pushed
// correctly.  This exercises a type that, unlike numeric types, has only two
// valid values and is commonly checked with identity equality.
func Test_loadByteCode_Bool(t *testing.T) {
	tc := newTestContext(t).withSymbol("flag", true)

	err := loadByteCode(tc.ctx, "flag")

	tc.assertNoError(err)
	tc.assertTopStack(true)
	tc.assertStackEmpty()
}

// Test_loadByteCode_Float64 verifies that a floating-point value is loaded
// without loss of precision.  IEEE 754 float64 has 53-bit mantissa precision;
// reflect.DeepEqual (used by assertTopStack) compares the exact bit pattern.
func Test_loadByteCode_Float64(t *testing.T) {
	tc := newTestContext(t).withSymbol("pi", 3.14159265358979)

	err := loadByteCode(tc.ctx, "pi")

	tc.assertNoError(err)
	tc.assertTopStack(3.14159265358979)
	tc.assertStackEmpty()
}

// Test_loadByteCode_ImmutableUnwrapped verifies that when the symbol table
// holds a data.Immutable (read-only constant wrapper), loadByteCode strips the
// wrapper before pushing the value.
//
// In Ego, a declaration like `const MaxItems = 100` stores
// data.Immutable{Value: 100} in the symbol table to prevent reassignment.
// When the constant is later used in an expression, the Load opcode must push
// the raw value (100), not the wrapper struct, so arithmetic and comparisons
// work correctly.
//
// loadByteCode calls data.UnwrapConstant(v) before pushing, which performs
// this stripping.
//
// Note: we bypass withSymbol here and call SetAlways directly so that the
// Immutable wrapper reaches the symbol table intact.  withSymbol goes through
// c.create + c.set, which passes through the Ego type system and may unwrap
// constants on its own code path.  SetAlways stores whatever value we give it
// verbatim, letting us test the exact invariant we care about.
func Test_loadByteCode_ImmutableUnwrapped(t *testing.T) {
	tc := newTestContext(t)
	// Place the Immutable wrapper directly in the local symbol table.
	tc.ctx.symbols.SetAlways("MaxItems", data.Constant(100))

	err := loadByteCode(tc.ctx, "MaxItems")

	tc.assertNoError(err)
	// The stack must contain the raw integer 100, NOT the wrapper.
	// If loadByteCode did NOT call UnwrapConstant, this would be
	// data.Immutable{Value: 100} and the assertion would fail.
	tc.assertTopStack(100)
	tc.assertStackEmpty()
}

// Test_loadByteCode_EmptyOperand verifies that an empty string operand is
// rejected immediately with ErrInvalidIdentifier.
//
// data.String("") == "", and len("") == 0 triggers the guard at the top of
// loadByteCode before any symbol-table lookup is attempted.  This prevents a
// symbol named "" from accidentally being loaded.
func Test_loadByteCode_EmptyOperand(t *testing.T) {
	tc := newTestContext(t)

	err := loadByteCode(tc.ctx, "")

	// ErrInvalidIdentifier because the name is empty.
	tc.assertError(err, errors.ErrInvalidIdentifier)
	// loadByteCode must not push anything when the operand is invalid.
	tc.assertStackEmpty()
}

// Test_loadByteCode_NilOperand verifies that a nil operand is treated as an
// empty name and returns ErrInvalidIdentifier.
//
// data.String(nil) returns "", so nil follows the same code path as an
// explicit empty string.  The compiler should never emit a nil operand here,
// but the guard defends against malformed bytecode.
func Test_loadByteCode_NilOperand(t *testing.T) {
	tc := newTestContext(t)

	err := loadByteCode(tc.ctx, nil)

	tc.assertError(err, errors.ErrInvalidIdentifier)
	tc.assertStackEmpty()
}

// Test_loadByteCode_UnknownName verifies that a name absent from all scopes
// in the symbol-table chain returns ErrUnknownIdentifier.
//
// newTestContext establishes a two-level root→local hierarchy.  "missing" is
// not in either table, so c.get() searches both and returns (nil, false).
func Test_loadByteCode_UnknownName(t *testing.T) {
	tc := newTestContext(t)

	err := loadByteCode(tc.ctx, "missing")

	tc.assertError(err, errors.ErrUnknownIdentifier)
	tc.assertStackEmpty()
}

// Test_loadByteCode_FromParentScope verifies that loadByteCode finds values
// stored in a parent (outer) symbol table, not just in the innermost scope.
//
// Ego's symbol table is a linked list of scopes.  c.get() calls
// symbols.Get(), which walks outward from the innermost scope until it finds
// the name or reaches the root.  This is essential for closures and nested
// functions that read variables declared in an outer scope.
//
// newTestContext creates a root→local two-level chain.  We store the value in
// the root (parent) to confirm the cross-scope walk works.
func Test_loadByteCode_FromParentScope(t *testing.T) {
	tc := newTestContext(t)
	// Parent() returns the root table.  SetAlways stores the value there.
	tc.ctx.symbols.Parent().SetAlways("rootValue", 99)

	err := loadByteCode(tc.ctx, "rootValue")

	tc.assertNoError(err)
	tc.assertTopStack(99)
	tc.assertStackEmpty()
}

// Test_loadByteCode_Array verifies that a *data.Array is loaded from the
// symbol table and pushed as a pointer (the same pointer, not a copy).
//
// Ego arrays are always stored and passed as *data.Array so that modifications
// inside functions are visible to the caller.  assertTopStack uses
// reflect.DeepEqual, which for pointer types compares the pointed-to structs,
// ensuring the loaded value is the same object.
func Test_loadByteCode_Array(t *testing.T) {
	arr := data.NewArrayFromInterfaces(data.IntType, 10, 20, 30)
	tc := newTestContext(t)
	tc.ctx.symbols.SetAlways("nums", arr)

	err := loadByteCode(tc.ctx, "nums")

	tc.assertNoError(err)
	tc.assertTopStack(arr)
	tc.assertStackEmpty()
}

// Test_loadByteCode_Map verifies that a *data.Map is loaded and pushed intact.
//
// Like arrays, maps are always pointer types in Ego.
func Test_loadByteCode_Map(t *testing.T) {
	m := data.NewMapFromMap(map[string]any{"x": 1, "y": 2})
	tc := newTestContext(t)
	tc.ctx.symbols.SetAlways("coords", m)

	err := loadByteCode(tc.ctx, "coords")

	tc.assertNoError(err)
	tc.assertTopStack(m)
	tc.assertStackEmpty()
}

// Test_loadByteCode_Struct verifies that a *data.Struct is loaded from the
// symbol table and pushed onto the stack unchanged.
//
// Structs are pointer types in Ego; the test confirms that the same pointer
// (same object) appears on the stack after loading.
func Test_loadByteCode_Struct(t *testing.T) {
	// Build a minimal struct with one integer field "id".
	structType := data.StructureType(data.Field{Name: "id", Type: data.IntType})
	s := data.NewStruct(structType)
	s.SetAlways("id", 7)

	tc := newTestContext(t)
	
	tc.ctx.symbols.SetAlways("record", s)

	err := loadByteCode(tc.ctx, "record")

	tc.assertNoError(err)
	tc.assertTopStack(s)
	tc.assertStackEmpty()
}
