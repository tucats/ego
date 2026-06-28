package builtins

// Tests for the Delete() builtin function (builtins/delete.go).
//
// Delete() handles three forms:
//   1. delete(map, key)       — removes a key from a map
//   2. delete(array, index)   — removes an element from an array by index
//   3. delete(varname)        — deletes a symbol from the symbol table
//                               (only available when extensions are enabled)

import (
	"testing"

	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/symbols"
)

// Test_Delete_MapByKey verifies that a key is removed from a map.
func Test_Delete_MapByKey(t *testing.T) {
	m := data.NewMapFromMap(map[string]any{
		"name": "Alice",
		"age":  30,
	})

	s := symbols.NewSymbolTable("test")
	args := data.NewList(m, "age")

	got, err := Delete(s, args)
	if err != nil {
		t.Fatalf("Delete(map, key) unexpected error: %v", err)
	}

	// The returned value should be the map itself (with the key removed).
	result, ok := got.(*data.Map)
	if !ok {
		t.Fatalf("Delete(map, key) returned %T, want *data.Map", got)
	}

	// The "age" key should no longer be present.
	_, found, _ := result.Get("age")
	if found {
		t.Error("Delete(map, key): key \"age\" still present after delete")
	}

	// The "name" key should still be present.
	_, found, _ = result.Get("name")
	if !found {
		t.Error("Delete(map, key): key \"name\" was unexpectedly removed")
	}
}

// Test_Delete_ArrayByIndex verifies that an element is removed from an array
// at the specified zero-based index.
func Test_Delete_ArrayByIndex(t *testing.T) {
	arr := data.NewArrayFromInterfaces(data.IntType, 10, 20, 30)
	s := symbols.NewSymbolTable("test")
	// Remove element at index 1 (value 20).
	args := data.NewList(arr, 1)

	got, err := Delete(s, args)
	if err != nil {
		t.Fatalf("Delete(array, index) unexpected error: %v", err)
	}

	result, ok := got.(*data.Array)
	if !ok {
		t.Fatalf("Delete(array, index) returned %T, want *data.Array", got)
	}

	// After deletion, the array should have 2 elements: [10, 30].
	if result.Len() != 2 {
		t.Errorf("Delete(array, index) length = %d, want 2", result.Len())
	}

	v0, _ := result.Get(0)
	if v0 != 10 {
		t.Errorf("Delete(array, index) [0] = %v, want 10", v0)
	}

	v1, _ := result.Get(1)
	if v1 != 30 {
		t.Errorf("Delete(array, index) [1] = %v, want 30", v1)
	}
}

// Test_Delete_ArrayOutOfRangeReturnsError verifies that an out-of-range index
// returns an error.
func Test_Delete_ArrayOutOfRangeReturnsError(t *testing.T) {
	arr := data.NewArrayFromInterfaces(data.IntType, 1, 2, 3)
	s := symbols.NewSymbolTable("test")
	// Index 10 is beyond the array length of 3.
	args := data.NewList(arr, 10)

	_, err := Delete(s, args)
	if err == nil {
		t.Fatal("Delete(array, out-of-range index) expected error, got nil")
	}
}

// Test_Delete_SymbolWithExtensions verifies that delete(varname) removes a
// symbol from the symbol table when extensions are enabled.
//
// Note: extensions() reads from the global symbols.RootSymbolTable, so we
// must explicitly set and clean up that global flag.
func Test_Delete_SymbolWithExtensions(t *testing.T) {
	s := symbols.NewSymbolTable("test")

	// Set extensions=true in the global root table; restore it after the test.
	s.Root().SetAlways(defs.ExtensionsVariable, true)
	t.Cleanup(func() { s.Root().SetAlways(defs.ExtensionsVariable, false) })

	// Create the variable to be deleted.
	s.SetAlways("myVar", 42)

	args := data.NewList("myVar")

	_, err := Delete(s, args)
	if err != nil {
		t.Fatalf("Delete(symbol) with extensions error: %v", err)
	}

	// The symbol should no longer be in the table.
	_, found := s.Get("myVar")
	if found {
		t.Error("Delete(symbol): variable still accessible after delete")
	}
}

// Test_Delete_SymbolWithoutExtensionsReturnsError verifies that delete(varname)
// is rejected when extensions are not enabled.
//
// The string form of delete is an Ego extension.  In standard mode it should
// return ErrArgumentType.
//
// Note: extensions() reads from the global symbols.RootSymbolTable, so we
// must explicitly disable the flag for this test.
func Test_Delete_SymbolWithoutExtensionsReturnsError(t *testing.T) {
	s := symbols.NewSymbolTable("test")

	// Explicitly disable extensions in the global root table; restore after.
	s.Root().SetAlways(defs.ExtensionsVariable, false)
	t.Cleanup(func() { s.Root().SetAlways(defs.ExtensionsVariable, false) })

	args := data.NewList("myVar")

	_, err := Delete(s, args)
	if err == nil {
		t.Fatal("Delete(symbol) without extensions expected error, got nil")
	}

	if !errors.Equals(errors.New(err), errors.ErrArgumentType) {
		t.Errorf("Delete(symbol) without extensions error = %v, want ErrArgumentType", err)
	}
}

// Test_Delete_WrongArgCountForMap verifies that passing only one argument to
// a map delete returns ErrArgumentCount.
func Test_Delete_WrongArgCountForMap(t *testing.T) {
	m := data.NewMapFromMap(map[string]any{"a": 1})
	s := symbols.NewSymbolTable("test")
	// Map delete requires 2 args; passing only 1 should fail.
	args := data.NewList(m)

	_, err := Delete(s, args)
	if err == nil {
		t.Fatal("Delete(map) with 1 arg expected error, got nil")
	}

	if !errors.Equals(errors.New(err), errors.ErrArgumentCount) {
		t.Errorf("Delete(map, 1-arg) error = %v, want ErrArgumentCount", err)
	}
}

// Test_Delete_InvalidTypeReturnsError verifies that calling delete with a type
// that is neither a string, map, nor array returns ErrInvalidType.
func Test_Delete_InvalidTypeReturnsError(t *testing.T) {
	s := symbols.NewSymbolTable("test")
	// An int is not a deleteable type.
	args := data.NewList(42, 0)

	_, err := Delete(s, args)
	if err == nil {
		t.Fatal("Delete(int, int) expected error, got nil")
	}

	if !errors.Equals(errors.New(err), errors.ErrInvalidType) {
		t.Errorf("Delete(int, int) error = %v, want ErrInvalidType", err)
	}
}
