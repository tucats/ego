package builtins

// Tests for the Append() builtin function (builtins/append.go).
//
// Append() implements the Ego built-in append() function.  It takes an array
// as its first argument and appends zero or more additional values, returning a
// new array.  Type-checking behaviour depends on the type-enforcement level
// stored in the symbol table.

import (
	"testing"

	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/symbols"
)

// helper: build a symbol table with the given type-enforcement level set.
// The level should be one of defs.StrictTypeEnforcement (0),
// defs.RelaxedTypeEnforcement (1), or defs.NoTypeEnforcement (2).
func newSymbolsWithTypeLevel(level int) *symbols.SymbolTable {
	s := symbols.NewSymbolTable("test")
	s.Root().SetAlways(defs.TypeCheckingVariable, level)

	return s
}

// ---- Basic append tests ----

// Test_Append_SingleItemToIntArray verifies that a single integer can be
// appended to an existing integer array.
func Test_Append_SingleItemToIntArray(t *testing.T) {
	arr := data.NewArrayFromInterfaces(data.IntType, 1, 2, 3)
	args := data.NewList(arr, 4)
	s := newSymbolsWithTypeLevel(defs.StrictTypeEnforcement)

	got, err := Append(s, args)
	if err != nil {
		t.Fatalf("Append() unexpected error: %v", err)
	}

	result, ok := got.(*data.Array)
	if !ok {
		t.Fatalf("Append() returned %T, want *data.Array", got)
	}

	// The result should have 4 elements.
	if result.Len() != 4 {
		t.Errorf("Append() returned array length %d, want 4", result.Len())
	}
}

// Test_Append_MultipleItemsToArray verifies that several items can be
// appended in a single call.
func Test_Append_MultipleItemsToArray(t *testing.T) {
	arr := data.NewArrayFromInterfaces(data.IntType, 10)
	args := data.NewList(arr, 20, 30)
	s := newSymbolsWithTypeLevel(defs.StrictTypeEnforcement)

	got, err := Append(s, args)
	if err != nil {
		t.Fatalf("Append() unexpected error: %v", err)
	}

	result, ok := got.(*data.Array)
	if !ok {
		t.Fatalf("Append() returned %T, want *data.Array", got)
	}

	if result.Len() != 3 {
		t.Errorf("Append() returned array length %d, want 3", result.Len())
	}
}

// Test_Append_PreservesElementType verifies that the returned array preserves
// the element type of the original array.
func Test_Append_PreservesElementType(t *testing.T) {
	arr := data.NewArrayFromInterfaces(data.StringType, "a", "b")
	args := data.NewList(arr, "c")
	s := newSymbolsWithTypeLevel(defs.StrictTypeEnforcement)

	got, err := Append(s, args)
	if err != nil {
		t.Fatalf("Append() unexpected error: %v", err)
	}

	result, ok := got.(*data.Array)
	if !ok {
		t.Fatalf("Append() returned %T, want *data.Array", got)
	}

	// The result array must still be typed as string.
	if !result.Type().IsType(data.StringType) {
		t.Errorf("Append() result type = %v, want string", result.Type())
	}
}

// ---- Strict-mode type checking ----

// Test_Append_StrictModeRejectsWrongType verifies that appending a value of
// the wrong type in strict mode returns an error.
//
// In strict mode (StrictTypeEnforcement = 0), elements added to a typed array
// must match the declared element type.  Appending a string to an int array
// must return ErrWrongArrayValueType.
func Test_Append_StrictModeRejectsWrongType(t *testing.T) {
	arr := data.NewArrayFromInterfaces(data.IntType, 1, 2, 3)
	// "hello" cannot be appended to an int array in strict mode.
	args := data.NewList(arr, "hello")
	s := newSymbolsWithTypeLevel(defs.StrictTypeEnforcement)

	_, err := Append(s, args)
	if err == nil {
		t.Fatal("Append() expected type error in strict mode, got nil")
	}

	if !errors.Equals(errors.New(err), errors.ErrWrongArrayValueType) {
		t.Errorf("Append() error = %v, want ErrWrongArrayValueType", err)
	}
}

// Test_Append_RelaxedModeCoercesCompatibleType verifies that in relaxed mode
// a compatible but mismatched type is coerced rather than rejected.
//
// Relaxed mode (RelaxedTypeEnforcement = 1) allows coercible values.
// Appending an int to a float64 array should succeed by converting the int.
func Test_Append_RelaxedModeCoercesCompatibleType(t *testing.T) {
	arr := data.NewArrayFromInterfaces(data.Float64Type, 1.1, 2.2)
	// 3 is an int, not a float64 — should be coerced in relaxed mode.
	args := data.NewList(arr, 3)
	s := newSymbolsWithTypeLevel(defs.RelaxedTypeEnforcement)

	got, err := Append(s, args)
	if err != nil {
		t.Fatalf("Append() relaxed mode coerce error: %v", err)
	}

	result, ok := got.(*data.Array)
	if !ok {
		t.Fatalf("Append() returned %T, want *data.Array", got)
	}

	if result.Len() != 3 {
		t.Errorf("Append() result length = %d, want 3", result.Len())
	}
}

// Test_Append_NoTypeEnforcementSkipsCheck verifies that with no type
// enforcement, any value may be appended to any array.
func Test_Append_NoTypeEnforcementSkipsCheck(t *testing.T) {
	arr := data.NewArrayFromInterfaces(data.IntType, 1, 2)
	// A bool value is incompatible with int, but should be allowed in
	// no-enforcement mode.
	args := data.NewList(arr, true)
	s := newSymbolsWithTypeLevel(defs.NoTypeEnforcement)

	_, err := Append(s, args)
	if err != nil {
		t.Fatalf("Append() no-enforcement error: %v", err)
	}
}

// ---- Interface / untyped arrays ----

// Test_Append_InterfaceArrayAcceptsAnyElement verifies that an interface
// (untyped) array accepts any element type without a type error.
func Test_Append_InterfaceArrayAcceptsAnyElement(t *testing.T) {
	arr := data.NewArrayFromInterfaces(data.InterfaceType, "x")
	args := data.NewList(arr, 42, true, 3.14)
	s := newSymbolsWithTypeLevel(defs.StrictTypeEnforcement)

	got, err := Append(s, args)
	if err != nil {
		t.Fatalf("Append() interface array error: %v", err)
	}

	result, ok := got.(*data.Array)
	if !ok {
		t.Fatalf("Append() returned %T, want *data.Array", got)
	}

	if result.Len() != 4 {
		t.Errorf("Append() result length = %d, want 4", result.Len())
	}
}

// Test_Append_RawGoSliceUniformTypeInferred verifies that when a raw []any is
// passed as the first argument and all elements share the same type, the
// returned array carries that concrete element type.
//
// BUILTIN-APPEND-1 is resolved: Append() now inspects the elements of a []any
// first argument and promotes `kind` to the uniform element type when all
// elements agree.
func Test_Append_RawGoSliceUniformTypeInferred(t *testing.T) {
	// All elements are strings, so kind should be inferred as StringType.
	rawSlice := []any{"alpha", "beta"}
	args := data.NewList(rawSlice, "gamma")
	s := newSymbolsWithTypeLevel(defs.StrictTypeEnforcement)

	got, err := Append(s, args)
	if err != nil {
		t.Fatalf("Append() raw slice error: %v", err)
	}

	result, ok := got.(*data.Array)
	if !ok {
		t.Fatalf("Append() returned %T, want *data.Array", got)
	}

	if result.Len() != 3 {
		t.Errorf("Append() raw slice length = %d, want 3", result.Len())
	}

	// After the fix the element type should be StringType, not InterfaceType.
	if result.Type().IsInterface() {
		t.Errorf("Append() raw slice: type is InterfaceType; want StringType (BUILTIN-APPEND-1 not fixed?)")
	}
}

// Test_Append_RawGoSliceMixedTypesStaysInterface verifies that a []any whose
// elements have different types results in an InterfaceType array (correct).
func Test_Append_RawGoSliceMixedTypesStaysInterface(t *testing.T) {
	rawSlice := []any{"text", 42} // mixed types
	args := data.NewList(rawSlice)
	s := newSymbolsWithTypeLevel(defs.StrictTypeEnforcement)

	got, err := Append(s, args)
	if err != nil {
		t.Fatalf("Append() mixed raw slice error: %v", err)
	}

	result, ok := got.(*data.Array)
	if !ok {
		t.Fatalf("Append() returned %T, want *data.Array", got)
	}

	// Mixed types cannot be promoted — result stays InterfaceType.
	if !result.Type().IsInterface() {
		t.Errorf("Append() mixed raw slice: type = %v, want InterfaceType", result.Type())
	}
}

// Test_Append_EmptyArrayGrowsCorrectly verifies that appending to an empty
// array creates a valid array with the expected element.
func Test_Append_EmptyArrayGrowsCorrectly(t *testing.T) {
	arr := data.NewArrayFromInterfaces(data.IntType)
	args := data.NewList(arr, 99)
	s := newSymbolsWithTypeLevel(defs.StrictTypeEnforcement)

	got, err := Append(s, args)
	if err != nil {
		t.Fatalf("Append() empty array error: %v", err)
	}

	result, ok := got.(*data.Array)
	if !ok {
		t.Fatalf("Append() returned %T, want *data.Array", got)
	}

	if result.Len() != 1 {
		t.Errorf("Append() result length = %d, want 1", result.Len())
	}

	v, _ := result.Get(0)
	if v != 99 {
		t.Errorf("Append() element[0] = %v, want 99", v)
	}
}
