package data

// Tests for data.Copy and data.CopyDepth — the type-preserving deep-copy functions.
//
// Coverage plan
// ─────────────
//   1.  nil input
//   2.  Simple scalars — type and value are both preserved
//   3.  Immutable wrapper — inner value is copied, wrapper is renewed
//   4.  Interface wrapper — inner value is copied, wrapper is renewed
//   5.  *Array (general) — elements are deep-copied; mutation of original
//         does not affect the copy
//   6.  *Array (byte) — the []byte path is tested separately
//   7.  *Map — key/value pairs are deep-copied; mutation is independent
//   8.  *Struct — fields are deep-copied; mutation is independent
//   9.  Nested structures — an array whose elements are structs
//  10.  Ego scalar pointers — original and copy share the same target
//         (pointer semantics)
//  11.  *Channel — returned as-is (shared communication object)
//  12.  Function / Package / *Type — returned as-is (immutable singletons)
//  13.  Unknown native Go type — ErrNotCopyable is returned
//  14.  CopyDepth depth=0 — ErrCopyDepthExceeded at the very first call
//  15.  CopyDepth with depth just sufficient for the nesting level — success
//  16.  CopyDepth with depth one less than needed — ErrCopyDepthExceeded
//  17.  Copy (default depth 100) handles deeply-nested structures fine

import (
	"testing"

	"github.com/tucats/ego/internal/errors"
)

// ─────────────────────────────────────────────────────────────────────────────
// 1. nil
// ─────────────────────────────────────────────────────────────────────────────

// Test_Copy_Nil verifies that Copy(nil) returns (nil, nil) with no error.
func Test_Copy_Nil(t *testing.T) {
	got, err := Copy(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got != nil {
		t.Errorf("Copy(nil) = %v, want nil", got)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// 2. Scalar values
// ─────────────────────────────────────────────────────────────────────────────

// Test_Copy_Scalars verifies that every scalar type is copied with both its
// type and value preserved.  Because Go copies value types on assignment,
// the result is always independent of the original.
func Test_Copy_Scalars(t *testing.T) {
	// Table of input values.  Each entry is "a value of type T; the copy must
	// equal v and have the same Go type."
	cases := []struct {
		name  string
		input any
		want  any
	}{
		{"bool true", true, true},
		{"bool false", false, false},
		{"byte", byte(200), byte(200)},
		{"int8", int8(-7), int8(-7)},
		{"int16", int16(300), int16(300)},
		{"uint16", uint16(600), uint16(600)},
		{"int32", int32(-1000), int32(-1000)},
		{"uint32", uint32(99999), uint32(99999)},
		{"int", int(42), int(42)},
		{"uint", uint(42), uint(42)},
		{"int64", int64(-9e18), int64(-9e18)},
		{"uint64", uint64(18e18), uint64(18e18)},
		{"float32", float32(3.14), float32(3.14)},
		{"float64", float64(2.718281828), float64(2.718281828)},
		{"string", "hello", "hello"},
		{"empty string", "", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Copy(tc.input)
			if err != nil {
				t.Fatalf("Copy(%v) unexpected error: %v", tc.input, err)
			}

			if got != tc.want {
				t.Errorf("Copy(%v) = %v (%T), want %v (%T)",
					tc.input, got, got, tc.want, tc.want)
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// 3. Immutable wrapper
// ─────────────────────────────────────────────────────────────────────────────

// Test_Copy_Immutable verifies that copying an Immutable produces a new
// Immutable whose inner value is an independent copy.
func Test_Copy_Immutable(t *testing.T) {
	original := Immutable{Value: int(55)}

	got, err := Copy(original)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	imm, ok := got.(Immutable)
	if !ok {
		t.Fatalf("Copy(Immutable) returned %T, want Immutable", got)
	}

	if imm.Value != int(55) {
		t.Errorf("inner value = %v (%T), want int(55)", imm.Value, imm.Value)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// 4. Interface wrapper
// ─────────────────────────────────────────────────────────────────────────────

// Test_Copy_Interface verifies that copying an Interface (Ego's "any" type
// wrapper) produces a new Interface with an independently copied inner value.
func Test_Copy_Interface(t *testing.T) {
	original := Interface{Value: int(77), BaseType: IntType}

	got, err := Copy(original)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	iface, ok := got.(Interface)
	if !ok {
		t.Fatalf("Copy(Interface) returned %T, want Interface", got)
	}

	if iface.Value != int(77) {
		t.Errorf("inner value = %v (%T), want int(77)", iface.Value, iface.Value)
	}

	// BaseType is a shared singleton (*Type); the pointer should be identical.
	if iface.BaseType != IntType {
		t.Errorf("BaseType = %v, want IntType", iface.BaseType)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// 5. *Array (general element type)
// ─────────────────────────────────────────────────────────────────────────────

// Test_Copy_Array_Independence verifies that modifying an element of the
// original *Array after Copy does not change the copy.
func Test_Copy_Array_Independence(t *testing.T) {
	original := NewArray(IntType, 3)
	_ = original.Set(0, int(10))
	_ = original.Set(1, int(20))
	_ = original.Set(2, int(30))

	got, err := Copy(original)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	copy_, ok := got.(*Array)
	if !ok {
		t.Fatalf("Copy(*Array) returned %T, want *Array", got)
	}

	// Mutate the original.
	_ = original.Set(1, int(999))

	// The copy must still hold the original value at index 1.
	elem, _ := copy_.Get(1)
	if elem != int(20) {
		t.Errorf("copy[1] = %v after mutating original, want int(20)", elem)
	}
}

// Test_Copy_Array_TypePreserved verifies that the element type of the copied
// array matches the original.
func Test_Copy_Array_TypePreserved(t *testing.T) {
	original := NewArray(Float64Type, 2)
	_ = original.Set(0, float64(1.1))
	_ = original.Set(1, float64(2.2))

	got, err := Copy(original)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	copy_, ok := got.(*Array)
	if !ok {
		t.Fatalf("Copy(*Array) returned %T, want *Array", got)
	}

	if copy_.Type() != Float64Type {
		t.Errorf("copy element type = %v, want Float64Type", copy_.Type())
	}

	if copy_.Len() != 2 {
		t.Errorf("copy length = %d, want 2", copy_.Len())
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// 6. *Array (byte array)
// ─────────────────────────────────────────────────────────────────────────────

// Test_Copy_ByteArray verifies that the []byte path produces an independent
// copy: mutating the original bytes does not affect the copy.
func Test_Copy_ByteArray(t *testing.T) {
	original := NewArrayFromBytes(10, 20, 30)

	got, err := Copy(original)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	copy_, ok := got.(*Array)
	if !ok {
		t.Fatalf("Copy(*Array/byte) returned %T, want *Array", got)
	}

	// Mutate the original byte.
	original.bytes[0] = 99

	// The copy must still hold the original value.
	elem, _ := copy_.Get(0)
	if elem != byte(10) {
		t.Errorf("copy[0] = %v after mutating original, want byte(10)", elem)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// 7. *Map
// ─────────────────────────────────────────────────────────────────────────────

// Test_Copy_Map_Independence verifies that modifying a map value in the
// original does not affect the copy.
func Test_Copy_Map_Independence(t *testing.T) {
	original := NewMap(StringType, IntType)
	_, _ = original.Set("alpha", int(1))
	_, _ = original.Set("beta", int(2))

	got, err := Copy(original)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	copy_, ok := got.(*Map)
	if !ok {
		t.Fatalf("Copy(*Map) returned %T, want *Map", got)
	}

	// Mutate the original.
	_, _ = original.Set("alpha", int(999))

	// The copy must still hold the original value.
	val, found, _ := copy_.Get("alpha")
	if !found {
		t.Fatal("key 'alpha' not found in copy")
	}

	if val != int(1) {
		t.Errorf("copy['alpha'] = %v after mutating original, want int(1)", val)
	}
}

// Test_Copy_Map_TypePreserved verifies that key and element types are
// carried over from the original to the copy.
func Test_Copy_Map_TypePreserved(t *testing.T) {
	original := NewMap(StringType, Float64Type)
	_, _ = original.Set("pi", float64(3.14))

	got, err := Copy(original)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	copy_, ok := got.(*Map)
	if !ok {
		t.Fatalf("Copy(*Map) returned %T, want *Map", got)
	}

	if copy_.KeyType() != StringType {
		t.Errorf("copy key type = %v, want StringType", copy_.KeyType())
	}

	if copy_.ElementType() != Float64Type {
		t.Errorf("copy element type = %v, want Float64Type", copy_.ElementType())
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// 8. *Struct
// ─────────────────────────────────────────────────────────────────────────────

// Test_Copy_Struct_Independence verifies that modifying a field in the
// original struct after Copy does not affect the copy.
func Test_Copy_Struct_Independence(t *testing.T) {
	// Build a simple anonymous struct with two fields.
	t_ := StructureType()
	t_.DefineField("x", IntType)
	t_.DefineField("y", StringType)

	original := NewStruct(t_)
	_ = original.Set("x", int(42))
	_ = original.Set("y", "hello")

	got, err := Copy(original)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	copy_, ok := got.(*Struct)
	if !ok {
		t.Fatalf("Copy(*Struct) returned %T, want *Struct", got)
	}

	// Mutate the original.
	_ = original.Set("x", int(999))

	// The copy must retain the pre-mutation value.
	val, found := copy_.Get("x")
	if !found {
		t.Fatal("field 'x' not found in copy")
	}

	if val != int(42) {
		t.Errorf("copy.x = %v after mutating original, want int(42)", val)
	}
}

// Test_Copy_Struct_MetadataPreserved verifies that the type definition,
// static flag, and readonly flag are preserved in the copy.
func Test_Copy_Struct_MetadataPreserved(t *testing.T) {
	t_ := StructureType()
	t_.DefineField("n", IntType)

	original := NewStruct(t_)
	_ = original.SetReadonly(true)

	got, err := Copy(original)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	copy_, ok := got.(*Struct)
	if !ok {
		t.Fatalf("Copy(*Struct) returned %T, want *Struct", got)
	}

	// readonly flag must be copied.
	if !copy_.readonly {
		t.Error("copy.readonly = false, want true")
	}

	// Type pointer is a shared singleton, so it must be the same object.
	if copy_.Type() != original.Type() {
		t.Error("copy type definition differs from original")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// 9. Nested structures
// ─────────────────────────────────────────────────────────────────────────────

// Test_Copy_NestedArrayInStruct verifies that an array stored as a struct
// field is deep-copied: mutating the original array does not affect the copy.
func Test_Copy_NestedArrayInStruct(t *testing.T) {
	inner := NewArray(IntType, 2)
	_ = inner.Set(0, int(11))
	_ = inner.Set(1, int(22))

	// Build a struct that holds the array in a field.
	// Use an anonymous struct (no pre-defined field types) so we can store
	// any value in the field without type-check failures.
	original := NewStructFromMap(map[string]any{"data": inner})

	got, err := Copy(original)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	copy_, ok := got.(*Struct)
	if !ok {
		t.Fatalf("Copy(*Struct) returned %T, want *Struct", got)
	}

	// Mutate the inner array of the original.
	_ = inner.Set(0, int(999))

	// The copy's inner array must still hold the original value.
	field, found := copy_.Get("data")
	if !found {
		t.Fatal("field 'data' not found in copy")
	}

	arr, ok := field.(*Array)
	if !ok {
		t.Fatalf("field 'data' is %T, want *Array", field)
	}

	elem, _ := arr.Get(0)
	if elem != int(11) {
		t.Errorf("copy.data[0] = %v after mutating original, want int(11)", elem)
	}
}

// Test_Copy_NestedStructInArray verifies that a struct stored as an array
// element is deep-copied.
func Test_Copy_NestedStructInArray(t *testing.T) {
	inner := NewStructFromMap(map[string]any{"v": int(5)})
	original := NewArray(InterfaceType, 1)
	_ = original.Set(0, inner)

	got, err := Copy(original)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	copy_, ok := got.(*Array)
	if !ok {
		t.Fatalf("Copy(*Array) returned %T, want *Array", got)
	}

	// Mutate the inner struct of the original.
	_ = inner.Set("v", int(999))

	// The copy's element must still hold the pre-mutation value.
	elem, _ := copy_.Get(0)

	s, ok := elem.(*Struct)
	if !ok {
		t.Fatalf("copy element is %T, want *Struct", elem)
	}

	val, found := s.Get("v")
	if !found {
		t.Fatal("field 'v' not found in copy element")
	}

	if val != int(5) {
		t.Errorf("copy element 'v' = %v after mutating original, want int(5)", val)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// 10. Ego scalar pointer types — shared target
// ─────────────────────────────────────────────────────────────────────────────

// Test_Copy_ScalarPointer_SharedTarget verifies that copying an Ego scalar
// pointer (*int) returns the SAME pointer, not a copy of the pointed-to value.
// Both the original and the copy must see a write made through the pointer,
// which is the correct Ego pointer semantic.
func Test_Copy_ScalarPointer_SharedTarget(t *testing.T) {
	n := int(7)
	ptr := &n // simulate what AddressOf returns for an int

	got, err := Copy(ptr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	copyPtr, ok := got.(*int)
	if !ok {
		t.Fatalf("Copy(*int) returned %T, want *int", got)
	}

	// The pointer itself must be identical (same target address).
	if copyPtr != ptr {
		t.Error("Copy(*int) returned a different pointer; expected shared target")
	}

	// A write through the original pointer must be visible via the copy pointer.
	n = 99

	if *copyPtr != 99 {
		t.Errorf("*copyPtr = %d after write to original, want 99", *copyPtr)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// 11. *Channel — returned as-is
// ─────────────────────────────────────────────────────────────────────────────

// Test_Copy_Channel_ReturnedAsIs verifies that copying a *Channel returns the
// exact same pointer (channels are inherently shared, not copyable).
func Test_Copy_Channel_ReturnedAsIs(t *testing.T) {
	ch := NewChannel(4) // buffered channel with capacity 4

	got, err := Copy(ch)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	chCopy, ok := got.(*Channel)
	if !ok {
		t.Fatalf("Copy(*Channel) returned %T, want *Channel", got)
	}

	if chCopy != ch {
		t.Error("Copy(*Channel) returned a different pointer; expected same channel")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// 12. Immutable singletons — Function, Package, *Type
// ─────────────────────────────────────────────────────────────────────────────

// Test_Copy_Type_ReturnedAsIs verifies that a *Type singleton is returned
// unchanged (type objects must not be duplicated).
func Test_Copy_Type_ReturnedAsIs(t *testing.T) {
	got, err := Copy(IntType)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tp, ok := got.(*Type)
	if !ok {
		t.Fatalf("Copy(*Type) returned %T, want *Type", got)
	}

	// Must be the exact same singleton pointer.
	if tp != IntType {
		t.Error("Copy(*Type) returned a different *Type; expected the same singleton")
	}
}

// Test_Copy_Function_ReturnedAsIs verifies that a Function descriptor is
// returned unchanged (functions are immutable after compilation).
func Test_Copy_Function_ReturnedAsIs(t *testing.T) {
	fn := Function{IsNative: true}

	got, err := Copy(fn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := got.(Function); !ok {
		t.Fatalf("Copy(Function) returned %T, want Function", got)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// 13. Unknown native Go type — error path
// ─────────────────────────────────────────────────────────────────────────────

// Test_Copy_UnknownType_ReturnsError verifies that Copy returns ErrNotCopyable
// when given a native Go type that is not part of the Ego data model.
//
// We use an anonymous struct literal as the "unknown" type; it has no case in
// copyValue's type switch and therefore falls through to the default error path.
func Test_Copy_UnknownType_ReturnsError(t *testing.T) {
	// A plain anonymous Go struct — not an Ego data type.
	unknown := struct{ X int }{X: 42}

	_, err := Copy(unknown)
	if err == nil {
		t.Fatal("Copy(unknown struct) expected ErrNotCopyable, got nil")
	}

	if !errors.Equal(errors.New(err), errors.ErrNotCopyable) {
		t.Errorf("Copy(unknown struct) error = %v, want ErrNotCopyable", err)
	}
}

// Test_Copy_UnknownTypeInsideArray_ReturnsError verifies that ErrNotCopyable
// propagates correctly when an uncopyable type is nested inside a *Array.
func Test_Copy_UnknownTypeInsideArray_ReturnsError(t *testing.T) {
	original := NewArray(InterfaceType, 1)
	// Store a raw Go struct (uncopyable) as an array element.
	original.data[0] = struct{ X int }{X: 1}

	_, err := Copy(original)
	if err == nil {
		t.Fatal("expected ErrNotCopyable for array containing uncopyable element, got nil")
	}

	if !errors.Equal(errors.New(err), errors.ErrNotCopyable) {
		t.Errorf("error = %v, want ErrNotCopyable", err)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// 14–17. Recursion depth guard
// ─────────────────────────────────────────────────────────────────────────────

// buildNestedArray constructs a chain of single-element arrays n levels deep
// with an int(42) at the innermost position.
//
//	n=0  → int(42)           (not an array)
//	n=1  → [42]
//	n=2  → [[42]]
//	n=3  → [[[42]]]
//	…
//
// Copying a result of buildNestedArray(n) requires a depth of at least n+1:
//   - one unit to process each array container
//   - one unit to process the final scalar
func buildNestedArray(n int) any {
	if n == 0 {
		return int(42)
	}

	// Build the n-1 level inner value first, then wrap it in a one-element
	// InterfaceType array so any value can be stored inside.
	inner := buildNestedArray(n - 1)
	arr := NewArray(InterfaceType, 1)
	_ = arr.Set(0, inner)

	return arr
}

// Test_CopyDepth_ZeroDepth verifies that CopyDepth returns ErrCopyDepthExceeded
// immediately when maxDepth is 0, even for a trivial scalar input.
// This confirms the guard fires before any work is attempted.
func Test_CopyDepth_ZeroDepth(t *testing.T) {
	_, err := CopyDepth(int(42), 0)
	if err == nil {
		t.Fatal("CopyDepth(42, 0) expected ErrCopyDepthExceeded, got nil")
	}

	if !errors.Equal(errors.New(err), errors.ErrCopyDepthExceeded) {
		t.Errorf("CopyDepth(42, 0) error = %v, want ErrCopyDepthExceeded", err)
	}
}

// Test_CopyDepth_ScalarAtDepthOne verifies that a scalar value succeeds at
// depth=1.  Scalars do not recurse, so one unit of depth is always enough.
func Test_CopyDepth_ScalarAtDepthOne(t *testing.T) {
	got, err := CopyDepth(int(7), 1)
	if err != nil {
		t.Fatalf("CopyDepth(7, 1) unexpected error: %v", err)
	}

	if got != int(7) {
		t.Errorf("CopyDepth(7, 1) = %v (%T), want int(7)", got, got)
	}
}

// Test_CopyDepth_ExactDepthSucceeds verifies that CopyDepth succeeds when
// maxDepth is exactly the minimum value required for the given structure.
//
// buildNestedArray(3) creates [[[42]]] — three levels of arrays plus a scalar.
// Copying it requires depth 4:
//   - depth 4 → process outer *Array, recurse at depth 3
//   - depth 3 → process middle *Array, recurse at depth 2
//   - depth 2 → process inner *Array, recurse at depth 1
//   - depth 1 → process int(42) scalar → success
func Test_CopyDepth_ExactDepthSucceeds(t *testing.T) {
	v := buildNestedArray(3) // [[[42]]]

	got, err := CopyDepth(v, 4)
	if err != nil {
		t.Fatalf("CopyDepth([[[42]]], 4) unexpected error: %v", err)
	}

	// Unwrap three levels of arrays to verify the innermost value survived.
	arr1, ok := got.(*Array)
	if !ok {
		t.Fatalf("level-1 result is %T, want *Array", got)
	}

	elem1, _ := arr1.Get(0)

	arr2, ok := elem1.(*Array)
	if !ok {
		t.Fatalf("level-2 result is %T, want *Array", elem1)
	}

	elem2, _ := arr2.Get(0)
	
	arr3, ok := elem2.(*Array)
	if !ok {
		t.Fatalf("level-3 result is %T, want *Array", elem2)
	}

	innermost, _ := arr3.Get(0)
	if innermost != int(42) {
		t.Errorf("innermost value = %v (%T), want int(42)", innermost, innermost)
	}
}

// Test_CopyDepth_InsufficientDepthFails verifies that CopyDepth returns
// ErrCopyDepthExceeded when maxDepth is one less than the minimum required.
//
// Using the same buildNestedArray(3) = [[[42]]] structure as the previous
// test, a depth of 3 is one too shallow:
//   - depth 3 → process outer *Array, recurse at depth 2
//   - depth 2 → process middle *Array, recurse at depth 1
//   - depth 1 → process inner *Array, recurse at depth 0
//   - depth 0 → ErrCopyDepthExceeded
func Test_CopyDepth_InsufficientDepthFails(t *testing.T) {
	v := buildNestedArray(3) // [[[42]]]

	_, err := CopyDepth(v, 3)
	if err == nil {
		t.Fatal("CopyDepth([[[42]]], 3) expected ErrCopyDepthExceeded, got nil")
	}

	if !errors.Equal(errors.New(err), errors.ErrCopyDepthExceeded) {
		t.Errorf("CopyDepth([[[42]]], 3) error = %v, want ErrCopyDepthExceeded", err)
	}
}

// Test_Copy_DefaultDepthHandlesReasonableNesting verifies that the one-argument
// Copy function (which uses maxCopyDepth=100) handles structures that are
// many levels deep without error.
//
// A 50-level nested array is well within the default limit of 100.
func Test_Copy_DefaultDepthHandlesReasonableNesting(t *testing.T) {
	v := buildNestedArray(50) // 50 levels of single-element arrays

	_, err := Copy(v)
	if err != nil {
		t.Fatalf("Copy on 50-level nested array unexpected error: %v", err)
	}
}
