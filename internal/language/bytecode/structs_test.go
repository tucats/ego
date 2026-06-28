package bytecode

// structs_test.go tests the bytecode instructions in structs.go:
//
//   loadIndexByteCode   — LoadIndex opcode (index into map/array/struct/channel)
//   loadSliceByteCode   — LoadSlice opcode (sub-string or sub-array slicing)
//   storeIndexByteCode  — StoreIndex opcode (write to map/array/struct/package)
//   storeInMap          — helper: map element write
//   storeInArray        — helper: array element write with bounds/type checks
//   storeMethodInType   — helper: method registration on a *data.Type
//   storeInPackage      — helper: package symbol write with visibility checks
//   storeIntoByteCode   — StoreInto opcode (map write, reversed operand order)
//   flattenByteCode     — Flatten opcode (variadic spread, e.g. f(arr...))
//
// # Known issues documented here
//
//   STRUCT-1  storeInPackage: the DiscardedVariable check is dead code (documented).
//   STRUCT-2  storeIndexByteCode: struct field is written before the package
//             visibility check, so the write is not rolled back on error.
//   STRUCT-3  flattenByteCode: argCountDelta is 0, not -1, for empty arrays.
//
// # Test organization
//
// Section 1:  loadIndexByteCode
// Section 2:  loadSliceByteCode
// Section 3:  storeIndexByteCode — package, type, map, struct, array, error cases
// Section 4:  storeInArray       — bounds and strict-mode type checks
// Section 5:  storeMethodInType  — *ByteCode, *Declaration, invalid value
// Section 6:  storeInPackage     — visibility, read-only, constant, normal store
// Section 7:  storeIntoByteCode  — map store, error cases
// Section 8:  flattenByteCode    — *data.Array, []any, scalar, empty, marker
//
// All tests use the newTestContext / withXxx / assertXxx helpers from
// testhelpers_test.go as required by the project testing standards.

import (
	"testing"

	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/symbols"
)

// ─── Section 1: loadIndexByteCode ────────────────────────────────────────────

// Test_loadIndexByteCode_ArrayValidIndex verifies that reading an in-bounds
// array element pushes the correct value.
func Test_loadIndexByteCode_ArrayValidIndex(t *testing.T) {
	arr := data.NewArrayFromInterfaces(data.IntType, 10, 20, 30)
	tc := newTestContext(t).withStack(arr, 1) // stack: [arr, 1]

	err := loadIndexByteCode(tc.ctx, nil)

	tc.assertNoError(err)
	tc.assertTopStack(20)
}

// Test_loadIndexByteCode_ArrayIndexFromOperand verifies that when a non-nil
// operand is provided it is used as the index instead of popping the stack.
func Test_loadIndexByteCode_ArrayIndexFromOperand(t *testing.T) {
	arr := data.NewArrayFromInterfaces(data.IntType, 10, 20, 30)
	tc := newTestContext(t).withStack(arr) // index comes from operand, not stack

	err := loadIndexByteCode(tc.ctx, 2)

	tc.assertNoError(err)
	tc.assertTopStack(30)
}

// Test_loadIndexByteCode_ArrayNegativeIndex verifies that a negative subscript
// returns ErrArrayIndex without modifying the stack.
func Test_loadIndexByteCode_ArrayNegativeIndex(t *testing.T) {
	arr := data.NewArrayFromInterfaces(data.IntType, 10, 20, 30)
	tc := newTestContext(t).withStack(arr, -1)

	err := loadIndexByteCode(tc.ctx, nil)

	tc.assertError(err, errors.ErrArrayIndex)
}

// Test_loadIndexByteCode_ArrayOutOfBoundsIndex verifies that a subscript
// equal to or beyond the array length returns ErrArrayIndex.
func Test_loadIndexByteCode_ArrayOutOfBoundsIndex(t *testing.T) {
	arr := data.NewArrayFromInterfaces(data.IntType, 10, 20, 30)
	tc := newTestContext(t).withStack(arr, 3) // length is 3, valid range 0..2

	err := loadIndexByteCode(tc.ctx, nil)

	tc.assertError(err, errors.ErrArrayIndex)
}

// Test_loadIndexByteCode_MapKnownKey verifies that a map lookup for an
// existing key pushes the stored value.
func Test_loadIndexByteCode_MapKnownKey(t *testing.T) {
	m := data.NewMap(data.StringType, data.IntType)
	_, _ = m.Set("answer", 42)

	tc := newTestContext(t).withStack(m, "answer")

	err := loadIndexByteCode(tc.ctx, nil)

	tc.assertNoError(err)
	tc.assertTopStack(42)
}

// Test_loadIndexByteCode_MapMissingKey verifies that reading a key that does
// not exist in a map pushes nil (no error, matching Go's zero-value map read).
func Test_loadIndexByteCode_MapMissingKey(t *testing.T) {
	m := data.NewMap(data.StringType, data.IntType)
	tc := newTestContext(t).withStack(m, "missing")

	err := loadIndexByteCode(tc.ctx, nil)

	tc.assertNoError(err)
	tc.assertTopStack(nil)
}

// Test_loadIndexByteCode_StructKnownField verifies that a struct field read
// pushes the field's value.
func Test_loadIndexByteCode_StructKnownField(t *testing.T) {
	s := data.NewStructFromMap(map[string]any{"x": 99})
	tc := newTestContext(t).withStack(s, "x")

	err := loadIndexByteCode(tc.ctx, nil)

	tc.assertNoError(err)
	tc.assertTopStack(99)
}

// Test_loadIndexByteCode_StructMissingField verifies the actual behavior when
// reading a non-existent struct field.
//
// data.Struct.Get first checks s.fields and then falls through to
// s.typeDef.functions.  When neither map contains the key, the map
// lookup for typeDef.functions returns the zero value of data.Function
// (a non-nil struct) and ok=false.  The bytecode ignores the ok flag
// and pushes the zero-value data.Function onto the stack.
//
// This behavior differs from Go map access (which returns nil for a missing
// key in a map[K]*T).  A future fix in data/structs.go could check ok before
// returning the function-map result, ensuring nil is returned instead.
func Test_loadIndexByteCode_StructMissingField(t *testing.T) {
	s := data.NewStructFromMap(map[string]any{"x": 1})
	tc := newTestContext(t).withStack(s, "y")

	err := loadIndexByteCode(tc.ctx, nil)

	tc.assertNoError(err)

	// The value pushed is data.Function{} (zero value), not nil, due to the
	// typeDef.functions map fallback in data.Struct.Get.
	v, popErr := tc.ctx.Pop()
	if popErr != nil {
		t.Fatalf("Pop failed: %v", popErr)
	}

	if v == nil {
		// If this ever passes as nil, the data.Struct.Get fix was applied.
		return
	}

	if _, ok := v.(data.Function); !ok {
		t.Errorf("expected data.Function{} or nil for missing field, got %T: %v", v, v)
	}
}

// Test_loadIndexByteCode_PackageReturnsError verifies that attempting to read
// from a *data.Package via LoadIndex is always rejected.  Package members must
// be accessed with the Member opcode, not LoadIndex.
func Test_loadIndexByteCode_PackageReturnsError(t *testing.T) {
	pkg := data.NewPackage("math", "math")
	tc := newTestContext(t).withStack(pkg, "Pi")

	err := loadIndexByteCode(tc.ctx, nil)

	tc.assertError(err, errors.ErrReadOnlyValue)
}

// Test_loadIndexByteCode_ChannelReceive verifies that indexing a channel
// receives a value from it (the index value is ignored for channels).
func Test_loadIndexByteCode_ChannelReceive(t *testing.T) {
	ch := data.NewChannel(1)
	_ = ch.Send(55)

	tc := newTestContext(t).withStack(ch, 0) // index 0 is ignored for channels

	err := loadIndexByteCode(tc.ctx, nil)

	tc.assertNoError(err)
	tc.assertTopStack(55)
}

// Test_loadIndexByteCode_InvalidType verifies that an unsupported composite
// type returns ErrInvalidType.
func Test_loadIndexByteCode_InvalidType(t *testing.T) {
	tc := newTestContext(t).withStack(42, "key") // int is not indexable

	err := loadIndexByteCode(tc.ctx, nil)

	tc.assertError(err, errors.ErrInvalidType)
}

// Test_loadIndexByteCode_StackMarkerAsCollection verifies that a StackMarker
// in the collection position returns ErrFunctionReturnedVoid.
func Test_loadIndexByteCode_StackMarkerAsCollection(t *testing.T) {
	tc := newTestContext(t).withStack(NewStackMarker("results"), "key")

	err := loadIndexByteCode(tc.ctx, nil)

	tc.assertError(err, errors.ErrFunctionReturnedVoid)
}

// Test_loadIndexByteCode_StackMarkerAsIndex verifies that a StackMarker in
// the index position returns ErrFunctionReturnedVoid.
func Test_loadIndexByteCode_StackMarkerAsIndex(t *testing.T) {
	arr := data.NewArrayFromInterfaces(data.IntType, 1, 2, 3)
	tc := newTestContext(t).withStack(arr, NewStackMarker("results"))

	err := loadIndexByteCode(tc.ctx, nil)

	tc.assertError(err, errors.ErrFunctionReturnedVoid)
}

// Test_loadIndexByteCode_MapTwoValuePath verifies the "comma-ok" two-value
// return path.  When the next bytecode instruction is StackCheck(2), a found
// bool is also pushed alongside the value.
func Test_loadIndexByteCode_MapTwoValuePath(t *testing.T) {
	m := data.NewMap(data.StringType, data.IntType)
	_, _ = m.Set("k", 7)

	tc := newTestContext(t)
	// Emit StackCheck(2) as the next instruction so the lookahead fires.
	tc.ctx.bc.Emit(StackCheck, 2)

	_ = tc.ctx.push(m)
	_ = tc.ctx.push("k")

	err := loadIndexByteCode(tc.ctx, nil)

	tc.assertNoError(err)
	// Stack should be (bottom → top): StackMarker("results"), 7, true
	tc.assertTopStack(7)   // value is on top after found bool
	tc.assertTopStack(true) // found bool
	// The StackMarker is still below; pop and discard it to keep the stack clean.
	_, _ = tc.ctx.Pop()
}

// ─── Section 2: loadSliceByteCode ────────────────────────────────────────────

// Test_loadSliceByteCode_StringValidSlice verifies that slicing a string with
// valid bounds pushes the expected sub-string.
func Test_loadSliceByteCode_StringValidSlice(t *testing.T) {
	tc := newTestContext(t).withStack("hello", 1, 4)

	err := loadSliceByteCode(tc.ctx, nil)

	tc.assertNoError(err)
	tc.assertTopStack("ell")
}

// Test_loadSliceByteCode_StringFullSlice verifies a slice covering the entire
// string returns the original string.
func Test_loadSliceByteCode_StringFullSlice(t *testing.T) {
	tc := newTestContext(t).withStack("abc", 0, 3)

	err := loadSliceByteCode(tc.ctx, nil)

	tc.assertNoError(err)
	tc.assertTopStack("abc")
}

// Test_loadSliceByteCode_StringUpperBoundTooLarge verifies that a high index
// beyond the string length returns ErrInvalidSliceIndex.
func Test_loadSliceByteCode_StringUpperBoundTooLarge(t *testing.T) {
	tc := newTestContext(t).withStack("hi", 0, 5)

	err := loadSliceByteCode(tc.ctx, nil)

	tc.assertError(err, errors.ErrInvalidSliceIndex)
}

// Test_loadSliceByteCode_StringNegativeUpperBound verifies that a negative
// high index returns ErrInvalidSliceIndex.
func Test_loadSliceByteCode_StringNegativeUpperBound(t *testing.T) {
	tc := newTestContext(t).withStack("hi", 0, -1)

	err := loadSliceByteCode(tc.ctx, nil)

	tc.assertError(err, errors.ErrInvalidSliceIndex)
}

// Test_loadSliceByteCode_StringNegativeLowerBound verifies that a negative
// low index returns ErrInvalidSliceIndex.
func Test_loadSliceByteCode_StringNegativeLowerBound(t *testing.T) {
	tc := newTestContext(t).withStack("hi", -1, 2)

	err := loadSliceByteCode(tc.ctx, nil)

	tc.assertError(err, errors.ErrInvalidSliceIndex)
}

// Test_loadSliceByteCode_StringLowGtHigh verifies that low > high returns
// ErrInvalidSliceIndex.
func Test_loadSliceByteCode_StringLowGtHigh(t *testing.T) {
	tc := newTestContext(t).withStack("hello", 3, 1)

	err := loadSliceByteCode(tc.ctx, nil)

	tc.assertError(err, errors.ErrInvalidSliceIndex)
}

// Test_loadSliceByteCode_ArrayValidSlice verifies that slicing an array with
// valid bounds pushes a new array containing only the selected elements.
func Test_loadSliceByteCode_ArrayValidSlice(t *testing.T) {
	arr := data.NewArrayFromInterfaces(data.IntType, 10, 20, 30, 40)
	tc := newTestContext(t).withStack(arr, 1, 3)

	err := loadSliceByteCode(tc.ctx, nil)

	tc.assertNoError(err)

	got, popErr := tc.ctx.Pop()
	if popErr != nil {
		t.Fatalf("Pop failed: %v", popErr)
	}

	result, ok := got.(*data.Array)
	if !ok {
		t.Fatalf("expected *data.Array, got %T", got)
	}

	if result.Len() != 2 {
		t.Fatalf("expected length 2, got %d", result.Len())
	}

	v0, _ := result.Get(0)
	v1, _ := result.Get(1)

	if v0 != 20 || v1 != 30 {
		t.Errorf("slice contents: got [%v %v], want [20 30]", v0, v1)
	}
}

// Test_loadSliceByteCode_InvalidType verifies that an unsupported type
// returns ErrInvalidType.
func Test_loadSliceByteCode_InvalidType(t *testing.T) {
	tc := newTestContext(t).withStack(99, 0, 1) // int is not sliceable

	err := loadSliceByteCode(tc.ctx, nil)

	tc.assertError(err, errors.ErrInvalidType)
}

// Test_loadSliceByteCode_StackMarker verifies that a StackMarker in any
// operand position returns ErrFunctionReturnedVoid.
func Test_loadSliceByteCode_StackMarker(t *testing.T) {
	tc := newTestContext(t).withStack(NewStackMarker("x"), 0, 1)

	err := loadSliceByteCode(tc.ctx, nil)

	tc.assertError(err, errors.ErrFunctionReturnedVoid)
}

// ─── Section 3: storeIndexByteCode ───────────────────────────────────────────

// Test_storeIndexByteCode_MapStore verifies that storing into a map key works
// and pushes the updated map back onto the stack.
func Test_storeIndexByteCode_MapStore(t *testing.T) {
	m := data.NewMap(data.StringType, data.IntType)
	tc := newTestContext(t).withStack(99, m, "key") // value=99, dest=m, index="key"

	err := storeIndexByteCode(tc.ctx, nil)

	tc.assertNoError(err)

	// The updated map should be on top.
	got, popErr := tc.ctx.Pop()
	if popErr != nil {
		t.Fatalf("Pop failed: %v", popErr)
	}

	result, ok := got.(*data.Map)
	if !ok {
		t.Fatalf("expected *data.Map, got %T", got)
	}

	v, _, getErr := result.Get("key")
	if getErr != nil || v != 99 {
		t.Errorf("map[\"key\"]: got %v (err %v), want 99", v, getErr)
	}
}

// Test_storeIndexByteCode_MapStoreWithOperand verifies that when the index is
// supplied as the instruction operand it is used in place of the stack top.
func Test_storeIndexByteCode_MapStoreWithOperand(t *testing.T) {
	m := data.NewMap(data.StringType, data.IntType)
	tc := newTestContext(t).withStack(7, m) // value=7, dest=m; index from operand

	err := storeIndexByteCode(tc.ctx, "operandKey")

	tc.assertNoError(err)

	top, popErr := tc.ctx.Pop()
	if popErr != nil {
		t.Fatalf("Pop failed: %v", popErr)
	}

	result := top.(*data.Map)
	v, _, _ := result.Get("operandKey")

	if v != 7 {
		t.Errorf("expected 7, got %v", v)
	}
}

// Test_storeIndexByteCode_StructStore verifies that writing to a struct field
// succeeds and the updated struct is pushed back.
func Test_storeIndexByteCode_StructStore(t *testing.T) {
	s := data.NewStructFromMap(map[string]any{"n": 0})
	tc := newTestContext(t).withStack(42, s, "n")

	err := storeIndexByteCode(tc.ctx, nil)

	tc.assertNoError(err)

	top, _ := tc.ctx.Pop()
	result := top.(*data.Struct)
	v, _ := result.Get("n")

	if v != 42 {
		t.Errorf("expected 42, got %v", v)
	}
}

// Test_storeIndexByteCode_ArrayStore verifies that writing to an in-bounds
// array element succeeds.
func Test_storeIndexByteCode_ArrayStore(t *testing.T) {
	arr := data.NewArray(data.IntType, 3)
	tc := newTestContext(t).withStack(77, arr, 1)

	err := storeIndexByteCode(tc.ctx, nil)

	tc.assertNoError(err)

	top, _ := tc.ctx.Pop()
	result := top.(*data.Array)
	v, _ := result.Get(1)

	if v != 77 {
		t.Errorf("expected 77, got %v", v)
	}
}

// Test_storeIndexByteCode_ArrayOutOfBounds verifies that writing to an array
// index that is beyond the end returns ErrArrayIndex.
func Test_storeIndexByteCode_ArrayOutOfBounds(t *testing.T) {
	arr := data.NewArray(data.IntType, 3)
	tc := newTestContext(t).withStack(1, arr, 10)

	err := storeIndexByteCode(tc.ctx, nil)

	tc.assertError(err, errors.ErrArrayIndex)
}

// Test_storeIndexByteCode_PackageUnexportedName verifies that writing to a
// lowercase (unexported) package member is rejected with ErrSymbolNotExported.
func Test_storeIndexByteCode_PackageUnexportedName(t *testing.T) {
	pkg := data.NewPackage("mypkg", "mypkg")
	tc := newTestContext(t).withStack(1, pkg, "lowercase")

	err := storeIndexByteCode(tc.ctx, nil)

	tc.assertError(err, errors.ErrSymbolNotExported)
}

// Test_storeIndexByteCode_InvalidType verifies that an unsupported destination
// type returns ErrInvalidType.
func Test_storeIndexByteCode_InvalidType(t *testing.T) {
	tc := newTestContext(t).withStack(1, "string-cant-be-indexed", "key")

	err := storeIndexByteCode(tc.ctx, nil)

	tc.assertError(err, errors.ErrInvalidType)
}

// Test_storeIndexByteCode_StackMarkerAsValue verifies that a StackMarker in
// the value position returns ErrFunctionReturnedVoid.
func Test_storeIndexByteCode_StackMarkerAsValue(t *testing.T) {
	m := data.NewMap(data.StringType, data.IntType)
	tc := newTestContext(t).withStack(NewStackMarker("x"), m, "key")

	err := storeIndexByteCode(tc.ctx, nil)

	tc.assertError(err, errors.ErrFunctionReturnedVoid)
}

// Test_storeIndexByteCode_TypeStoreMethod verifies that storing a *ByteCode
// into a *data.Type registers it as a named method.
func Test_storeIndexByteCode_TypeStoreMethod(t *testing.T) {
	// Create a type that can hold methods.
	t1 := data.NewTypeInstance(data.StructKind)

	// Create a minimal ByteCode with a declaration.
	bc := &ByteCode{
		declaration: &data.Declaration{Name: "Greet"},
	}

	tc := newTestContext(t).withStack(bc, t1, "Greet")

	err := storeIndexByteCode(tc.ctx, nil)

	tc.assertNoError(err)
}

// ─── Section 4: storeInArray ─────────────────────────────────────────────────

// Test_storeInArray_ValidStore verifies that writing a value at a valid index
// succeeds, the value is stored, and the array is pushed onto the stack.
func Test_storeInArray_ValidStore(t *testing.T) {
	arr := data.NewArray(data.IntType, 3)
	tc := newTestContext(t)

	err := storeInArray(tc.ctx, arr, 1, 55)

	tc.assertNoError(err)

	top, _ := tc.ctx.Pop()
	result := top.(*data.Array)
	v, _ := result.Get(1)

	if v != 55 {
		t.Errorf("expected 55, got %v", v)
	}
}

// Test_storeInArray_NegativeSubscript verifies that a negative subscript
// returns ErrArrayIndex.
func Test_storeInArray_NegativeSubscript(t *testing.T) {
	arr := data.NewArray(data.IntType, 3)
	tc := newTestContext(t)

	err := storeInArray(tc.ctx, arr, -1, 1)

	tc.assertError(err, errors.ErrArrayIndex)
}

// Test_storeInArray_SubscriptBeyondEnd verifies that a subscript equal to
// the array length (off-by-one) returns ErrArrayIndex.
func Test_storeInArray_SubscriptBeyondEnd(t *testing.T) {
	arr := data.NewArray(data.IntType, 3)
	tc := newTestContext(t)

	err := storeInArray(tc.ctx, arr, 3, 1) // valid range is 0..2

	tc.assertError(err, errors.ErrArrayIndex)
}

// Test_storeInArray_StrictTypeMismatch verifies that in strict mode writing a
// value whose Go type differs from the existing element type is rejected.
func Test_storeInArray_StrictTypeMismatch(t *testing.T) {
	arr := data.NewArray(data.IntType, 3)
	// Pre-populate index 0 so the type checker has something to compare against.
	_ = arr.Set(0, int(1))

	tc := newTestContext(t).withTypeStrictness(defs.StrictTypeEnforcement)

	err := storeInArray(tc.ctx, arr, 0, "not an int") // int vs string

	tc.assertError(err, errors.ErrInvalidVarType)
}

// Test_storeInArray_StrictModeNilExistingAllowsWrite verifies that in strict
// mode, writing to an element that currently holds nil (freshly allocated
// array) succeeds — the nil guard allows initial population.
func Test_storeInArray_StrictModeNilExistingAllowsWrite(t *testing.T) {
	arr := data.NewArray(data.IntType, 3) // all elements are nil initially
	tc := newTestContext(t).withTypeStrictness(defs.StrictTypeEnforcement)

	err := storeInArray(tc.ctx, arr, 0, 42) // first write to nil slot is OK

	tc.assertNoError(err)
}

// ─── Section 5: storeMethodInType ────────────────────────────────────────────

// Test_storeMethodInType_WithBytecode verifies that a *ByteCode value with a
// non-nil declaration is registered as a method on the type.
func Test_storeMethodInType_WithBytecode(t *testing.T) {
	t1 := data.NewTypeInstance(data.StructKind)
	bc := &ByteCode{
		declaration: &data.Declaration{Name: "Run"},
	}

	tc := newTestContext(t)

	err := storeMethodInType(tc.ctx, t1, "Run", bc)

	tc.assertNoError(err)
}

// Test_storeMethodInType_WithDeclaration verifies that a *data.Declaration is
// accepted directly as a method registration value.
func Test_storeMethodInType_WithDeclaration(t *testing.T) {
	t1 := data.NewTypeInstance(data.StructKind)
	decl := &data.Declaration{Name: "Init"}

	tc := newTestContext(t)

	err := storeMethodInType(tc.ctx, t1, "Init", decl)

	tc.assertNoError(err)
}

// Test_storeMethodInType_InvalidValue verifies that passing a value that is
// neither *ByteCode nor *data.Declaration returns ErrInvalidValue.
func Test_storeMethodInType_InvalidValue(t *testing.T) {
	t1 := data.NewTypeInstance(data.StructKind)
	tc := newTestContext(t)

	err := storeMethodInType(tc.ctx, t1, "BadMethod", "not-a-function")

	tc.assertError(err, errors.ErrInvalidValue)
}

// Test_storeMethodInType_BytecodeWithNilDeclaration verifies that a *ByteCode
// with a nil declaration returns ErrInvalidValue (the declaration must be set
// before registering the bytecode as a method).
func Test_storeMethodInType_BytecodeWithNilDeclaration(t *testing.T) {
	t1 := data.NewTypeInstance(data.StructKind)
	bc := &ByteCode{} // declaration is nil

	tc := newTestContext(t)

	err := storeMethodInType(tc.ctx, t1, "NoDecl", bc)

	tc.assertError(err, errors.ErrInvalidValue)
}

// ─── Section 6: storeInPackage ───────────────────────────────────────────────

// Test_storeInPackage_UnexportedName verifies that writing to a lowercase
// (unexported) package symbol returns ErrSymbolNotExported.
func Test_storeInPackage_UnexportedName(t *testing.T) {
	pkg := data.NewPackage("testpkg", "testpkg")
	tc := newTestContext(t)

	err := storeInPackage(tc.ctx, pkg, "lowercase", 1)

	tc.assertError(err, errors.ErrSymbolNotExported)
}

// Test_storeInPackage_BytecodeIsReadOnly verifies that overwriting a *ByteCode
// value already stored in the package returns ErrReadOnlyValue.
func Test_storeInPackage_BytecodeIsReadOnly(t *testing.T) {
	bc := &ByteCode{}
	pkg := data.NewPackageFromMap("testpkg", map[string]any{
		"Func": bc,
	})

	tc := newTestContext(t)

	err := storeInPackage(tc.ctx, pkg, "Func", "new value")

	tc.assertError(err, errors.ErrReadOnlyValue)
}

// Test_storeInPackage_ImmutablePackageValueIsReadOnly verifies that a
// data.Immutable stored directly in the package object cannot be overwritten.
func Test_storeInPackage_ImmutablePackageValueIsReadOnly(t *testing.T) {
	pkg := data.NewPackageFromMap("testpkg", map[string]any{
		"Const": data.Immutable{Value: 3},
	})

	tc := newTestContext(t)

	err := storeInPackage(tc.ctx, pkg, "Const", 99)

	tc.assertError(err, errors.ErrReadOnlyValue)
}

// Test_storeInPackage_ConstantInSymbolTable verifies that a symbol stored as
// an Immutable in the package symbol table cannot be reassigned.
func Test_storeInPackage_ConstantInSymbolTable(t *testing.T) {
	pkg := data.NewPackage("testpkg", "testpkg")

	// Seed the package's symbol table with an Immutable constant.
	syms := symbols.GetPackageSymbolTable(pkg)
	syms.SetAlways("Pi", data.Immutable{Value: 3.14159})

	tc := newTestContext(t)

	err := storeInPackage(tc.ctx, pkg, "Pi", 0)

	tc.assertError(err, errors.ErrInvalidConstant)
}

// Test_storeInPackage_NormalExportedNameSucceeds verifies that writing to an
// exported name that already exists in the package's symbol table succeeds.
//
// storeInPackage uses syms.Set (not syms.SetAlways), which requires the symbol
// to already exist in the table.  This is intentional: storeInPackage is used
// to UPDATE existing package symbols, not to create new ones.  New symbols are
// seeded at package-load time before any Ego code can assign to them.
func Test_storeInPackage_NormalExportedNameSucceeds(t *testing.T) {
	pkg := data.NewPackage("testpkg", "testpkg")

	// Pre-create the symbol so that syms.Set can find and update it.
	syms := symbols.GetPackageSymbolTable(pkg)
	syms.SetAlways("Value", 0) // seed with initial value

	tc := newTestContext(t)

	err := storeInPackage(tc.ctx, pkg, "Value", 42)

	tc.assertNoError(err)

	v, found := syms.Get("Value")

	if !found {
		t.Errorf("symbol 'Value' not found after store")
	}

	if v != 42 {
		t.Errorf("symbol 'Value': got %v, want 42", v)
	}
}

// Test_storeInPackage_NoDiscardedVariableCheck_STRUCT1 confirms the STRUCT-1
// fix: the dead DiscardedVariable ("_") guard was removed from storeInPackage.
//
// Any name starting with "_" fails the HasCapitalizedName check before
// reaching the removed guard.  The behavior is unchanged but the dead code is
// gone.  This test is kept as a regression anchor.
func Test_storeInPackage_NoDiscardedVariableCheck_STRUCT1(t *testing.T) {
	pkg := data.NewPackage("testpkg", "testpkg")
	tc := newTestContext(t)

	// "_Foo" starts with "_", which is not uppercase, so HasCapitalizedName
	// returns false and the function immediately returns ErrSymbolNotExported.
	err := storeInPackage(tc.ctx, pkg, "_Foo", 1)

	tc.assertError(err, errors.ErrSymbolNotExported)
}

// ─── Section 7: storeIntoByteCode ────────────────────────────────────────────

// Test_storeIntoByteCode_MapStore verifies that storing into a map via the
// StoreInto opcode succeeds and pushes the updated map.
// StoreInto pops in the order: index (top), value, destination (bottom).
func Test_storeIntoByteCode_MapStore(t *testing.T) {
	m := data.NewMap(data.StringType, data.IntType)
	// Stack order: destination (bottom), value, index (top)
	tc := newTestContext(t).withStack(m, 88, "k")

	err := storeIntoByteCode(tc.ctx, nil)

	tc.assertNoError(err)

	top, _ := tc.ctx.Pop()
	result := top.(*data.Map)
	v, _, _ := result.Get("k")

	if v != 88 {
		t.Errorf("expected 88, got %v", v)
	}
}

// Test_storeIntoByteCode_InvalidType verifies that an unsupported destination
// type returns ErrInvalidType.
func Test_storeIntoByteCode_InvalidType(t *testing.T) {
	tc := newTestContext(t).withStack("not-a-map", 1, "key")

	err := storeIntoByteCode(tc.ctx, nil)

	tc.assertError(err, errors.ErrInvalidType)
}

// Test_storeIntoByteCode_StackMarkerAsDestination verifies that a StackMarker
// in the destination position returns ErrFunctionReturnedVoid.
func Test_storeIntoByteCode_StackMarkerAsDestination(t *testing.T) {
	tc := newTestContext(t).withStack(NewStackMarker("x"), 1, "key")

	err := storeIntoByteCode(tc.ctx, nil)

	tc.assertError(err, errors.ErrFunctionReturnedVoid)
}

// Test_storeIntoByteCode_EmptyStack verifies that calling StoreInto with an
// empty stack returns an error (stack underflow from Pop).
func Test_storeIntoByteCode_EmptyStack(t *testing.T) {
	tc := newTestContext(t)

	err := storeIntoByteCode(tc.ctx, nil)

	if err == nil {
		t.Errorf("expected an error from empty stack, got nil")
	}
}

// ─── Section 8: flattenByteCode ──────────────────────────────────────────────

// Test_flattenByteCode_DataArray verifies that a *data.Array is expanded into
// individual stack values and argCountDelta reflects the net change.
func Test_flattenByteCode_DataArray(t *testing.T) {
	arr := data.NewArrayFromInterfaces(data.IntType, 10, 20, 30)
	tc := newTestContext(t).withStack(arr)

	err := flattenByteCode(tc.ctx, nil)

	tc.assertNoError(err)

	// Expect 3 elements pushed; argCountDelta should be 2 (3 elements - 1 original).
	if tc.ctx.argCountDelta != 2 {
		t.Errorf("argCountDelta: got %d, want 2", tc.ctx.argCountDelta)
	}

	tc.assertTopStack(30)
	tc.assertTopStack(20)
	tc.assertTopStack(10)
	tc.assertStackEmpty()
}

// Test_flattenByteCode_GoSlice verifies that a plain []any is also expanded
// correctly, with the same argCountDelta logic.
func Test_flattenByteCode_GoSlice(t *testing.T) {
	slice := []any{1, 2}
	tc := newTestContext(t).withStack(slice)

	err := flattenByteCode(tc.ctx, nil)

	tc.assertNoError(err)

	if tc.ctx.argCountDelta != 1 {
		t.Errorf("argCountDelta: got %d, want 1", tc.ctx.argCountDelta)
	}

	tc.assertTopStack(2)
	tc.assertTopStack(1)
	tc.assertStackEmpty()
}

// Test_flattenByteCode_ScalarPassthrough verifies that a non-array scalar
// value is pushed back unchanged and argCountDelta stays 0.
func Test_flattenByteCode_ScalarPassthrough(t *testing.T) {
	tc := newTestContext(t).withStack(99)

	err := flattenByteCode(tc.ctx, nil)

	tc.assertNoError(err)

	if tc.ctx.argCountDelta != 0 {
		t.Errorf("argCountDelta: got %d, want 0", tc.ctx.argCountDelta)
	}

	tc.assertTopStack(99)
}

// Test_flattenByteCode_EmptyArray_STRUCT3 confirms the STRUCT-3 fix: flattening
// an empty *data.Array now produces argCountDelta = -1, correctly signalling
// to the following Call opcode that the array argument slot should be removed
// from the argument count (0 expanded elements replace the original 1).
func Test_flattenByteCode_EmptyArray_STRUCT3(t *testing.T) {
	arr := data.NewArray(data.IntType, 0)
	tc := newTestContext(t).withStack(arr)

	err := flattenByteCode(tc.ctx, nil)

	tc.assertNoError(err)

	// After the STRUCT-3 fix, argCountDelta must be -1 for an empty array.
	if tc.ctx.argCountDelta != -1 {
		t.Errorf("argCountDelta: got %d, want -1 (empty array removes the original slot)", tc.ctx.argCountDelta)
	}

	tc.assertStackEmpty()
}

// Test_flattenByteCode_StackMarker verifies that a StackMarker returns
// ErrFunctionReturnedVoid.
func Test_flattenByteCode_StackMarker(t *testing.T) {
	tc := newTestContext(t).withStack(NewStackMarker("results"))

	err := flattenByteCode(tc.ctx, nil)

	tc.assertError(err, errors.ErrFunctionReturnedVoid)
}

// Test_flattenByteCode_EmptyStack verifies that Flatten on an empty stack
// returns an error (stack underflow).
func Test_flattenByteCode_EmptyStack(t *testing.T) {
	tc := newTestContext(t)

	err := flattenByteCode(tc.ctx, nil)

	if err == nil {
		t.Errorf("expected an error on empty stack, got nil")
	}
}

// Test_flattenByteCode_ResetsArgCountDelta verifies that argCountDelta is
// reset to 0 at the start of each Flatten invocation, overwriting any prior
// accumulated delta.
func Test_flattenByteCode_ResetsArgCountDelta(t *testing.T) {
	arr := data.NewArrayFromInterfaces(data.IntType, 5)
	tc := newTestContext(t).withStack(arr)
	tc.ctx.argCountDelta = 99 // stale delta from a previous operation

	err := flattenByteCode(tc.ctx, nil)

	tc.assertNoError(err)

	// argCountDelta should reflect only this flatten (1 element → delta 0).
	if tc.ctx.argCountDelta != 0 {
		t.Errorf("argCountDelta: got %d, want 0", tc.ctx.argCountDelta)
	}
}

// Test_storeIndexByteCode_StructPackageVisibility_STRUCT2 confirms the STRUCT-2
// fix: storeIndexByteCode now checks package visibility BEFORE calling a.Set,
// so an unexported field from a different package is rejected without modifying
// the struct.
func Test_storeIndexByteCode_StructPackageVisibility_STRUCT2(t *testing.T) {
	// Create a type that declares it belongs to "otherpkg".
	pkgType := data.NewTypeInstance(data.StructKind)
	pkgType.SetPackage("otherpkg")

	// Build a struct with that type and a field to observe.
	s := data.NewStruct(pkgType)
	s.SetAlways("unexported", 0)

	// Context is in "mypkg", which differs from the struct's package.
	tc := newTestContext(t)
	tc.ctx.pkg = "mypkg"

	// Push value=1, struct, field name "unexported".
	_ = tc.ctx.push(1)
	_ = tc.ctx.push(s)
	_ = tc.ctx.push("unexported")

	err := storeIndexByteCode(tc.ctx, nil)

	// The error must be ErrSymbolNotExported.
	tc.assertError(err, errors.ErrSymbolNotExported)

	// After the STRUCT-2 fix the field must still hold its original value (0),
	// confirming the visibility check ran before the write.
	v, _ := s.Get("unexported")
	if v != 0 {
		t.Errorf("STRUCT-2: field 'unexported' was modified to %v before error; fix did not work", v)
	}
}
