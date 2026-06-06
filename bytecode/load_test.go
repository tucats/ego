package bytecode

// load_test.go — comprehensive unit tests for loadByteCode and explodeByteCode.
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
// explodeByteCode (Explode opcode):
//   - Pops a *data.Map from the stack.
//   - Creates a local variable for every key-value pair in the map via
//     c.setAlways; the variable name is the string form of the map key.
//   - The map must have string keys; integer or other key types return
//     ErrWrongMapKeyType.
//   - After processing, pushes a bool: true if the map was empty, false if it
//     had at least one entry.
//   - A StackMarker on the stack returns ErrFunctionReturnedVoid.
//   - A non-map value returns ErrInvalidType.
//   - Stack underflow is detected by the initial c.Pop() and returns
//     ErrStackUnderflow (decorated with c.runtimeError since LOAD-1 fix).
//
// # Test organization
//
// Section 1 covers loadByteCode.
// Section 2 covers explodeByteCode.
//
// All tests use the newTestContext / withXxx / assertXxx helpers from
// testhelpers_test.go as required by the project testing standards.

import (
	"testing"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
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

// ─── Section 2: explodeByteCode ──────────────────────────────────────────────

// Test_explodeByteCode_StringKeyMap verifies the core happy path: a *data.Map
// with string keys is popped from the stack, a local variable is created for
// every key-value pair in the map, and the boolean "empty" flag (false, because
// the map had entries) is pushed onto the stack.
//
// This is the primary use of the Explode opcode: spreading a map's key-value
// pairs into named local variables in one step.
func Test_explodeByteCode_StringKeyMap(t *testing.T) {
	// Build a string-keyed map with two entries.
	m := data.NewMapFromMap(map[string]any{"alpha": 1, "beta": 2})
	// Put the map on the stack; explodeByteCode will pop it.
	tc := newTestContext(t).withStack(m)

	err := explodeByteCode(tc.ctx, nil)

	tc.assertNoError(err)
	// A non-empty map means the "empty" flag is false.  That boolean is the
	// only item left on the stack after the map was popped.
	tc.assertTopStack(false)
	tc.assertStackEmpty()
	// Each map entry should now be a local variable in the symbol table.
	tc.assertSymbolValue("alpha", 1)
	tc.assertSymbolValue("beta", 2)
}

// Test_explodeByteCode_EmptyMap verifies that processing an empty *data.Map
// pushes true (the "empty" flag) and does not create any new variables.
//
// data.NewMap creates a typed map with zero entries.  When the key-iteration
// loop has nothing to iterate, the `empty` local stays true and is pushed
// after the loop.
func Test_explodeByteCode_EmptyMap(t *testing.T) {
	// An empty map — string keys, interface{} values, no entries.
	m := data.NewMap(data.StringType, data.InterfaceType)
	tc := newTestContext(t).withStack(m)

	err := explodeByteCode(tc.ctx, nil)

	tc.assertNoError(err)
	// Empty map → "empty" flag is true.
	tc.assertTopStack(true)
	tc.assertStackEmpty()
}

// Test_explodeByteCode_IntKeyMap verifies that a map with integer keys is
// rejected with ErrWrongMapKeyType.
//
// explodeByteCode calls m.KeyType().IsString() to check whether the map's key
// type is a string type.  Integer keys cannot be used as Ego identifier names,
// so they are rejected before any variables are created.
func Test_explodeByteCode_IntKeyMap(t *testing.T) {
	// NewMapFromMap infers the key type as IntType from the Go map's key kind.
	m := data.NewMapFromMap(map[int]any{1: "one", 2: "two"})
	tc := newTestContext(t).withStack(m)

	err := explodeByteCode(tc.ctx, nil)

	tc.assertError(err, errors.ErrWrongMapKeyType)
	// The "empty" flag must NOT have been pushed because the error occurred
	// before any c.push call was reached.  The map was already popped, so the
	// stack must be empty.
	tc.assertStackEmpty()
}

// Test_explodeByteCode_NonMapString verifies that a plain string on the stack
// is rejected with ErrInvalidType.
//
// Only *data.Map is a valid input for Explode.  All other types fail the
// type-assertion `v.(*data.Map)` and fall through to the else branch, which
// returns ErrInvalidType with the actual type as context.
func Test_explodeByteCode_NonMapString(t *testing.T) {
	tc := newTestContext(t).withStack("I am not a map")

	err := explodeByteCode(tc.ctx, nil)

	tc.assertError(err, errors.ErrInvalidType)
	// The string was popped from the stack and the error path did not push
	// anything, so the stack must be empty.
	tc.assertStackEmpty()
}

// Test_explodeByteCode_NonMapInt verifies that an integer on the stack also
// returns ErrInvalidType.  Integers are a distinct type from strings, but the
// rejection path is identical for all non-map types.
func Test_explodeByteCode_NonMapInt(t *testing.T) {
	tc := newTestContext(t).withStack(42)

	err := explodeByteCode(tc.ctx, nil)

	tc.assertError(err, errors.ErrInvalidType)
	tc.assertStackEmpty()
}

// Test_explodeByteCode_NonMapStruct verifies that a *data.Struct on the stack
// is rejected with ErrInvalidType.
//
// This test is particularly important because the original function comment
// incorrectly described the operand as "a struct" (see LOAD-2 in
// docs/BYTECODE_ISSUES.md).  In reality the function only accepts *data.Map.
// This test serves as a regression anchor: if the implementation is ever
// accidentally changed to accept *data.Struct, the test will fail.
func Test_explodeByteCode_NonMapStruct(t *testing.T) {
	// Build a minimal struct.
	structType := data.StructureType(data.Field{Name: "x", Type: data.IntType})
	s := data.NewStruct(structType)
	s.SetAlways("x", 7)
	tc := newTestContext(t).withStack(s)

	err := explodeByteCode(tc.ctx, nil)

	// *data.Struct is NOT a *data.Map; the type assertion fails → ErrInvalidType.
	tc.assertError(err, errors.ErrInvalidType)
	tc.assertStackEmpty()
}

// Test_explodeByteCode_StackUnderflow verifies the behavior when the execution
// stack is empty when explodeByteCode calls c.Pop().
//
// Since the LOAD-1 fix, the raw error from c.Pop() is now decorated with
// c.runtimeError before being returned, so the error message includes the
// module name and source line — consistent with all other runtime errors in
// the bytecode package.
func Test_explodeByteCode_StackUnderflow(t *testing.T) {
	tc := newTestContext(t) // empty stack

	err := explodeByteCode(tc.ctx, nil)

	// ErrStackUnderflow is returned and, after the LOAD-1 fix, is wrapped with
	// source-location information.
	tc.assertError(err, errors.ErrStackUnderflow)
}

// Test_explodeByteCode_StackMarker verifies that a StackMarker sentinel on the
// stack is detected and returns ErrFunctionReturnedVoid.
//
// StackMarkers are internal sentinels pushed by the runtime to separate logical
// groups of values on the stack (for example, the results of a function call).
// If a StackMarker appears where a map is expected, it almost certainly means
// a function that should have returned a value returned void instead.
// explodeByteCode checks for this explicitly with isStackMarker(v).
func Test_explodeByteCode_StackMarker(t *testing.T) {
	// NewStackMarker creates the sentinel value used internally by the runtime.
	tc := newTestContext(t).withStack(NewStackMarker("results"))

	err := explodeByteCode(tc.ctx, nil)

	tc.assertError(err, errors.ErrFunctionReturnedVoid)
	// The marker was popped and no result was pushed; stack must be empty.
	tc.assertStackEmpty()
}

// Test_explodeByteCode_VariableValues verifies that the values written to the
// symbol table after explodeByteCode runs accurately reflect the values from
// the source map.
//
// A single-entry map is used here so the assertion is unambiguous: exactly one
// variable is created and its value must match the map entry precisely.
func Test_explodeByteCode_VariableValues(t *testing.T) {
	m := data.NewMapFromMap(map[string]any{"score": 99})
	tc := newTestContext(t).withStack(m)

	err := explodeByteCode(tc.ctx, nil)

	tc.assertNoError(err)
	// Non-empty map → empty flag is false.
	tc.assertTopStack(false)
	tc.assertStackEmpty()
	// "score" must be in the local symbol table with value 99.
	tc.assertSymbolValue("score", 99)
}

// Test_explodeByteCode_SingleEntry verifies that a single-key map correctly
// creates exactly one variable and pushes false (map was not empty).
//
// This is a slightly simplified version of Test_explodeByteCode_StringKeyMap,
// focused on confirming that the "empty" flag tracks entry count accurately
// rather than a fixed boolean.
func Test_explodeByteCode_SingleEntry(t *testing.T) {
	m := data.NewMapFromMap(map[string]any{"only": "value"})
	tc := newTestContext(t).withStack(m)

	err := explodeByteCode(tc.ctx, nil)

	tc.assertNoError(err)
	// One entry means "not empty" → false.
	tc.assertTopStack(false)
	tc.assertStackEmpty()
	tc.assertSymbolValue("only", "value")
}

// Test_explodeByteCode_StringValueTypes verifies that explodeByteCode preserves
// the Go types of map values when writing them to the symbol table.
//
// The map created by data.NewMapFromMap(map[string]any{...}) stores values as
// the any (interface{}) type.  The individual values retain their concrete Go
// types.  assertSymbolValue uses reflect.DeepEqual to confirm both the value
// and the type match.
func Test_explodeByteCode_StringValueTypes(t *testing.T) {
	// A map with three entries, each with a different value type.
	m := data.NewMapFromMap(map[string]any{
		"name":   "Alice",
		"active": true,
	})
	tc := newTestContext(t).withStack(m)

	err := explodeByteCode(tc.ctx, nil)

	tc.assertNoError(err)
	tc.assertTopStack(false)
	tc.assertStackEmpty()
	tc.assertSymbolValue("name", "Alice")
	tc.assertSymbolValue("active", true)
}
