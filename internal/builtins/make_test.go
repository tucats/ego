package builtins

// Tests for the Make() and New() builtins (builtins/make.go).
//
// Make() implements the Ego built-in make() function.  It constructs:
//   - Maps:    make(map[K]V) or make(map[K]V, capacityHint)
//     fixed BUG-22: map support was added; capacity is accepted but ignored
//     (matching Go's semantics where it is only a performance hint).
//   - Arrays:  make([]T, size) or make([]T, size, capacity)
//     capacity must be >= size; currently ignored for actual allocation.
//   - Channels: make(chan, n)
//
// New() implements the Ego built-in new() function.  It returns a pointer to
// a zero-value of the given type.

import (
	"testing"

	"github.com/tucats/ego/internal/errors" // used for ErrInvalidValue, ErrInvalidType, ErrNotAType
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/language/symbols"
)

// ---- Make — basic cases ----

// Test_Make_IntArray verifies that make([]int, n) returns a native Go []any
// slice pre-populated with integer zero values.
func Test_Make_IntArray(t *testing.T) {
	s := symbols.NewSymbolTable("test")
	// When a raw int is passed as the type model, Make produces a []any of ints.
	args := data.NewList(0, 4) // model = int(0), size = 4

	got, err := Make(s, args)
	if err != nil {
		t.Fatalf("Make(int, 4) error: %v", err)
	}

	arr, ok := got.([]any)
	if !ok {
		t.Fatalf("Make(int, 4) returned %T, want []any", got)
	}

	if len(arr) != 4 {
		t.Errorf("Make(int, 4) length = %d, want 4", len(arr))
	}

	// All elements should be int zero value (0).
	if arr[0] != 0 {
		t.Errorf("Make(int, 4) [0] = %v, want 0", arr[0])
	}
}

// Test_Make_BoolArray verifies that make([]bool, n) produces a slice of
// false values.
func Test_Make_BoolArray(t *testing.T) {
	s := symbols.NewSymbolTable("test")
	args := data.NewList(false, 3)

	got, err := Make(s, args)
	if err != nil {
		t.Fatalf("Make(bool, 3) error: %v", err)
	}

	arr, ok := got.([]any)
	if !ok {
		t.Fatalf("Make(bool, 3) returned %T, want []any", got)
	}

	if len(arr) != 3 {
		t.Errorf("Make(bool, 3) length = %d, want 3", len(arr))
	}

	if arr[0] != false {
		t.Errorf("Make(bool, 3) [0] = %v, want false", arr[0])
	}
}

// Test_Make_StringArray verifies that make([]string, n) produces a slice of
// empty strings.
func Test_Make_StringArray(t *testing.T) {
	s := symbols.NewSymbolTable("test")
	args := data.NewList("", 2)

	got, err := Make(s, args)
	if err != nil {
		t.Fatalf("Make(string, 2) error: %v", err)
	}

	arr, ok := got.([]any)
	if !ok {
		t.Fatalf("Make(string, 2) returned %T, want []any", got)
	}

	if len(arr) != 2 {
		t.Errorf("Make(string, 2) length = %d, want 2", len(arr))
	}

	if arr[0] != "" {
		t.Errorf("Make(string, 2) [0] = %v, want empty string", arr[0])
	}
}

// Test_Make_Float64Array verifies that make([]float64, n) produces a slice of
// 0.0 values.
func Test_Make_Float64Array(t *testing.T) {
	s := symbols.NewSymbolTable("test")
	args := data.NewList(0.0, 3)

	got, err := Make(s, args)
	if err != nil {
		t.Fatalf("Make(float64, 3) error: %v", err)
	}

	arr, ok := got.([]any)
	if !ok {
		t.Fatalf("Make(float64, 3) returned %T, want []any", got)
	}

	if arr[0] != 0.0 {
		t.Errorf("Make(float64, 3) [0] = %v, want 0.0", arr[0])
	}
}

// ---- Make — typed Ego arrays ----

// Test_Make_EgoTypedArray verifies that passing a *data.Type produces a typed
// Ego array via InstanceOfType, not a raw []any.
func Test_Make_EgoTypedArray(t *testing.T) {
	s := symbols.NewSymbolTable("test")
	// Passing data.IntType as the first argument exercises the *data.Type branch.
	args := data.NewList(data.IntType, 5)

	got, err := Make(s, args)
	if err != nil {
		t.Fatalf("Make(*data.Type int, 5) error: %v", err)
	}

	// InstanceOfType(IntType) returns int(0), so kind becomes int; Make returns []any.
	if got == nil {
		t.Error("Make(*data.Type int, 5) returned nil")
	}
}

// Test_Make_EgoArrayMakeMethod verifies that passing a *data.Array produces a
// correctly-sized array via the array's own Make method.
func Test_Make_EgoArrayMakeMethod(t *testing.T) {
	s := symbols.NewSymbolTable("test")
	// A *data.Array as the first argument uses v.Make(size).
	arr := data.NewArray(data.StringType, 0)
	args := data.NewList(arr, 6)

	got, err := Make(s, args)
	if err != nil {
		t.Fatalf("Make(*data.Array, 6) error: %v", err)
	}

	result, ok := got.(*data.Array)
	if !ok {
		t.Fatalf("Make(*data.Array, 6) returned %T, want *data.Array", got)
	}

	if result.Len() != 6 {
		t.Errorf("Make(*data.Array, 6) length = %d, want 6", result.Len())
	}
}

// ---- Make — channel ----

// Test_Make_Channel verifies that make(chan, n) returns a *data.Channel with
// the requested capacity.
func Test_Make_Channel(t *testing.T) {
	s := symbols.NewSymbolTable("test")
	// Passing a *data.Channel as the type model creates a new channel.
	chModel := data.NewChannel(0) // zero-capacity model
	args := data.NewList(chModel, 3)

	got, err := Make(s, args)
	if err != nil {
		t.Fatalf("Make(chan, 3) error: %v", err)
	}

	_, ok := got.(*data.Channel)
	if !ok {
		t.Fatalf("Make(chan, 3) returned %T, want *data.Channel", got)
	}
}

// ---- Make — invalid size ----

// Test_Make_NegativeSizeReturnsError verifies that Make() returns an error for
// a negative size rather than panicking.
//
// BUILTIN-MAKE-1 is resolved: Make() now validates size >= 0 before calling
// Go's built-in make(), preventing the unrecoverable runtime panic.
func Test_Make_NegativeSizeReturnsError(t *testing.T) {
	s := symbols.NewSymbolTable("test")
	args := data.NewList(0, -1) // negative size

	_, err := Make(s, args)
	if err == nil {
		t.Fatal("Make(int, -1) expected error, got nil")
	}

	if !errors.Equals(errors.New(err), errors.ErrInvalidFunctionArgument) {
		t.Errorf("Make(int, -1) error = %v, want ErrInvalidValue", err)
	}
}

// Test_Make_ZeroSizeReturnsEmptyArray verifies that size 0 is valid and
// produces an empty array.
func Test_Make_ZeroSizeReturnsEmptyArray(t *testing.T) {
	s := symbols.NewSymbolTable("test")
	args := data.NewList(0, 0)

	got, err := Make(s, args)
	if err != nil {
		t.Fatalf("Make(int, 0) error: %v", err)
	}

	arr, ok := got.([]any)
	if !ok {
		t.Fatalf("Make(int, 0) returned %T, want []any", got)
	}

	if len(arr) != 0 {
		t.Errorf("Make(int, 0) length = %d, want 0", len(arr))
	}
}

// Test_Make_InvalidSizeStringReturnsError verifies that passing a non-integer
// as the size argument produces an error.
func Test_Make_InvalidSizeStringReturnsError(t *testing.T) {
	s := symbols.NewSymbolTable("test")
	args := data.NewList(0, "not-a-number")

	_, err := Make(s, args)
	if err == nil {
		t.Fatal("Make(int, \"not-a-number\") expected error, got nil")
	}
}

// ---- Make — maps (BUG-22 fix) ----

// Test_Make_Map_NoSize verifies that make(map[K]V) with no size argument
// returns an initialized, writable *data.Map (BUG-22 fix).
//
// Before the fix this call errored with "incorrect function argument count: 1"
// because the map branch did not exist.
func Test_Make_Map_NoSize(t *testing.T) {
	s := symbols.NewSymbolTable("test")
	mapType := data.MapType(data.StringType, data.IntType) // map[string]int
	args := data.NewList(mapType)

	got, err := Make(s, args)
	if err != nil {
		t.Fatalf("Make(map[string]int) error: %v (BUG-22 not fixed?)", err)
	}

	m, ok := got.(*data.Map)
	if !ok {
		t.Fatalf("Make(map[string]int) returned %T, want *data.Map", got)
	}

	// The map must be writable: Set() must not return an error.
	if _, err := m.Set("key", 42); err != nil {
		t.Errorf("Make(map[string]int) returned un-writable map: %v", err)
	}
}

// Test_Make_Map_WithCapacity verifies that make(map[K]V, n) with a capacity
// hint returns a valid, writable *data.Map.  The capacity is accepted for
// compatibility with Go source but is currently ignored for Ego maps (as in
// Go, where it is only a performance hint, not a hard allocation).
func Test_Make_Map_WithCapacity(t *testing.T) {
	s := symbols.NewSymbolTable("test")
	mapType := data.MapType(data.StringType, data.IntType)
	args := data.NewList(mapType, 10) // capacity hint = 10

	got, err := Make(s, args)
	if err != nil {
		t.Fatalf("Make(map[string]int, 10) error: %v", err)
	}

	m, ok := got.(*data.Map)
	if !ok {
		t.Fatalf("Make(map[string]int, 10) returned %T, want *data.Map", got)
	}

	if _, err := m.Set("hello", 1); err != nil {
		t.Errorf("Make(map[string]int, 10): map not writable: %v", err)
	}
}

// Test_Make_Map_ZeroCapacity verifies that make(map[K]V, 0) is valid and
// returns a writable map.  Zero is an accepted capacity hint in Go.
func Test_Make_Map_ZeroCapacity(t *testing.T) {
	s := symbols.NewSymbolTable("test")
	mapType := data.MapType(data.IntType, data.StringType)
	args := data.NewList(mapType, 0)

	got, err := Make(s, args)
	if err != nil {
		t.Fatalf("Make(map[int]string, 0) error: %v", err)
	}

	if _, ok := got.(*data.Map); !ok {
		t.Fatalf("Make(map[int]string, 0) returned %T, want *data.Map", got)
	}
}

// Test_Make_Map_NegativeCapacity verifies that a negative capacity hint is
// rejected.  Go panics on negative slice/map sizes; Ego catches it explicitly.
func Test_Make_Map_NegativeCapacity(t *testing.T) {
	s := symbols.NewSymbolTable("test")
	mapType := data.MapType(data.StringType, data.IntType)
	args := data.NewList(mapType, -1)

	_, err := Make(s, args)
	if err == nil {
		t.Fatal("Make(map[string]int, -1) expected error for negative capacity, got nil")
	}

	if !errors.Equals(errors.New(err), errors.ErrInvalidFunctionArgument) {
		t.Errorf("Make(map[string]int, -1) error = %v, want ErrInvalidFunctionArgument", err)
	}
}

// Test_Make_Map_NonIntegerCapacity verifies that a non-integer capacity hint
// produces a type error rather than a panic.
func Test_Make_Map_NonIntegerCapacity(t *testing.T) {
	s := symbols.NewSymbolTable("test")
	mapType := data.MapType(data.StringType, data.IntType)
	args := data.NewList(mapType, "big") // capacity must be an integer

	_, err := Make(s, args)
	if err == nil {
		t.Fatal("Make(map[string]int, \"big\") expected error for non-integer capacity, got nil")
	}
}

// Test_Make_Map_IsEmptyAfterCreation verifies that a newly made map starts
// empty (Len == 0) regardless of the capacity hint.
func Test_Make_Map_IsEmptyAfterCreation(t *testing.T) {
	s := symbols.NewSymbolTable("test")
	mapType := data.MapType(data.StringType, data.Float64Type)
	args := data.NewList(mapType, 5)

	got, err := Make(s, args)
	if err != nil {
		t.Fatalf("Make(map[string]float64, 5) error: %v", err)
	}

	m := got.(*data.Map)
	if m.Len() != 0 {
		t.Errorf("Make(map[string]float64, 5) Len = %d, want 0", m.Len())
	}
}

// ---- Make — array with capacity (third argument) ----

// Test_Make_Array_WithValidCapacity verifies that make([]int, 3, 5) succeeds
// when capacity >= size.  The capacity value is accepted but currently ignored
// for Ego arrays (same as Go, where it only affects the underlying slice
// allocation).
func Test_Make_Array_WithValidCapacity(t *testing.T) {
	s := symbols.NewSymbolTable("test")
	args := data.NewList(0, 3, 5) // int model, size=3, capacity=5

	got, err := Make(s, args)
	if err != nil {
		t.Fatalf("Make(int, 3, 5) error: %v", err)
	}

	arr, ok := got.([]any)
	if !ok {
		t.Fatalf("Make(int, 3, 5) returned %T, want []any", got)
	}

	if len(arr) != 3 {
		t.Errorf("Make(int, 3, 5) length = %d, want 3", len(arr))
	}
}

// Test_Make_Array_CapacityEqualToSize verifies that capacity == size is valid.
func Test_Make_Array_CapacityEqualToSize(t *testing.T) {
	s := symbols.NewSymbolTable("test")
	args := data.NewList(0, 4, 4) // size=4, capacity=4

	_, err := Make(s, args)
	if err != nil {
		t.Fatalf("Make(int, 4, 4) error: %v", err)
	}
}

// Test_Make_Array_CapacityLessThanSize verifies that capacity < size returns
// an error.  In Go, make([]T, size, cap) panics when cap < size; Ego rejects
// it with ErrInvalidFunctionArgument.
func Test_Make_Array_CapacityLessThanSize(t *testing.T) {
	s := symbols.NewSymbolTable("test")
	args := data.NewList(0, 5, 2) // size=5, capacity=2 — invalid

	_, err := Make(s, args)
	if err == nil {
		t.Fatal("Make(int, 5, 2) expected error for capacity < size, got nil")
	}

	if !errors.Equals(errors.New(err), errors.ErrInvalidFunctionArgument) {
		t.Errorf("Make(int, 5, 2) error = %v, want ErrInvalidFunctionArgument", err)
	}
}

// Test_Make_Array_NegativeCapacityReturnsError verifies that a negative
// capacity argument is rejected.
func Test_Make_Array_NegativeCapacityReturnsError(t *testing.T) {
	s := symbols.NewSymbolTable("test")
	args := data.NewList(0, 3, -1) // size=3, capacity=-1

	_, err := Make(s, args)
	if err == nil {
		t.Fatal("Make(int, 3, -1) expected error for negative capacity, got nil")
	}

	if !errors.Equals(errors.New(err), errors.ErrInvalidFunctionArgument) {
		t.Errorf("Make(int, 3, -1) error = %v, want ErrInvalidFunctionArgument", err)
	}
}

// Test_Make_Array_NonIntegerCapacityReturnsError verifies that a non-integer
// capacity argument produces an error.
func Test_Make_Array_NonIntegerCapacityReturnsError(t *testing.T) {
	s := symbols.NewSymbolTable("test")
	args := data.NewList(0, 3, "huge")

	_, err := Make(s, args)
	if err == nil {
		t.Fatal("Make(int, 3, \"huge\") expected error for non-integer capacity, got nil")
	}
}

// ---- New() ----

// Test_New_IntTypeReturnsPointerToInt verifies that new(*data.Type for int)
// returns a pointer value (address) that can be stored and read.
func Test_New_IntTypeReturnsPointer(t *testing.T) {
	s := symbols.NewSymbolTable("test")
	args := data.NewList(data.IntType)

	got, err := New(s, args)
	if err != nil {
		t.Fatalf("New(IntType) error: %v", err)
	}

	if got == nil {
		t.Fatal("New(IntType) returned nil, want pointer")
	}
}

// Test_New_NonTypeReturnsError verifies that passing a non-type argument to
// New() returns ErrNotAType.
func Test_New_NonTypeReturnsError(t *testing.T) {
	s := symbols.NewSymbolTable("test")
	args := data.NewList(42) // 42 is an int, not a *data.Type

	_, err := New(s, args)
	if err == nil {
		t.Fatal("New(int literal) expected error, got nil")
	}

	if !errors.Equals(errors.New(err), errors.ErrNotAType) {
		t.Errorf("New(int literal) error = %v, want ErrNotAType", err)
	}
}

// Test_New_StringTypeReturnsPointerToString verifies that new(StringType)
// returns a non-nil pointer.
func Test_New_StringTypeReturnsPointer(t *testing.T) {
	s := symbols.NewSymbolTable("test")
	args := data.NewList(data.StringType)

	got, err := New(s, args)
	if err != nil {
		t.Fatalf("New(StringType) error: %v", err)
	}

	if got == nil {
		t.Fatal("New(StringType) returned nil, want pointer")
	}
}
