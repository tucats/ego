package builtins

// Tests for the DeepCopy() builtin function (builtins/copy.go).
//
// DeepCopy() is a recursive deep-copy utility used by $new() and other parts
// of the Ego runtime to create independent copies of complex data structures.
// The depth parameter caps recursion to avoid infinite loops in cyclic data.

import (
	"testing"

	"github.com/tucats/ego/data"
)

// ---- Scalar types ----

// Test_DeepCopy_Bool verifies that a bool value is returned as-is.
// Scalars are immutable values in Go, so a direct copy is correct.
func Test_DeepCopy_Bool(t *testing.T) {
	got := DeepCopy(true, MaxDeepCopyDepth)
	if got != true {
		t.Errorf("DeepCopy(bool) = %v, want true", got)
	}
}

// Test_DeepCopy_Int verifies that an int value is returned unchanged.
func Test_DeepCopy_Int(t *testing.T) {
	got := DeepCopy(42, MaxDeepCopyDepth)
	if got != 42 {
		t.Errorf("DeepCopy(int) = %v, want 42", got)
	}
}

// Test_DeepCopy_String verifies that a string value is returned unchanged.
// Strings are immutable in Go so a direct copy is safe.
func Test_DeepCopy_String(t *testing.T) {
	got := DeepCopy("hello", MaxDeepCopyDepth)
	if got != "hello" {
		t.Errorf("DeepCopy(string) = %v, want \"hello\"", got)
	}
}

// Test_DeepCopy_Float64 verifies that a float64 is returned unchanged.
func Test_DeepCopy_Float64(t *testing.T) {
	got := DeepCopy(3.14, MaxDeepCopyDepth)
	if got != 3.14 {
		t.Errorf("DeepCopy(float64) = %v, want 3.14", got)
	}
}

// Test_DeepCopy_NilDepthExceeded verifies that when depth is exhausted (< 0),
// DeepCopy returns nil rather than panicking.
func Test_DeepCopy_NilDepthExceeded(t *testing.T) {
	got := DeepCopy("should be nil", -1)
	if got != nil {
		t.Errorf("DeepCopy with depth=-1 returned %v, want nil", got)
	}
}

// ---- Map ----

// Test_DeepCopy_Map verifies that a *data.Map is deep-copied: the result is an
// independent map with the same keys and values.
func Test_DeepCopy_Map(t *testing.T) {
	original := data.NewMap(data.StringType, data.IntType)
	_, _ = original.Set("x", 10)
	_, _ = original.Set("y", 20)

	got := DeepCopy(original, MaxDeepCopyDepth)

	copyMapValue, ok := got.(*data.Map)
	if !ok {
		t.Fatalf("DeepCopy(*data.Map) returned %T, want *data.Map", got)
	}

	// Verify that values were copied.
	v, _, _ := copyMapValue.Get("x")
	if v != 10 {
		t.Errorf("DeepCopy map: copy[\"x\"] = %v, want 10", v)
	}

	// Verify independence: mutating the copy must not affect the original.
	_, _ = copyMapValue.Set("x", 999)
	origX, _, _ := original.Get("x")

	if origX != 10 {
		t.Errorf("DeepCopy map: modifying copy changed original (got %v, want 10)", origX)
	}
}

// ---- Array (documents BUILTIN-COPY-1) ----

// Test_DeepCopy_ArrayCopiesElements verifies that DeepCopy returns a populated
// copy of an array with the same elements as the source.
//
// BUILTIN-COPY-1 is resolved: the fix changes v.Set(i, vv) to r.Set(i, vv)
// in the copy loop so elements are written into the result array, not back
// into the source.
func Test_DeepCopy_ArrayCopiesElements(t *testing.T) {
	source := data.NewArrayFromInterfaces(data.IntType, 1, 2, 3)

	got := DeepCopy(source, MaxDeepCopyDepth)

	result, ok := got.(*data.Array)
	if !ok {
		t.Fatalf("DeepCopy(*data.Array) returned %T, want *data.Array", got)
	}

	if result.Len() != 3 {
		t.Fatalf("DeepCopy array: result length = %d, want 3", result.Len())
	}

	// All three elements must be present in the result, not zero-valued.
	for i, want := range []any{1, 2, 3} {
		v, err := result.Get(i)
		if err != nil {
			t.Errorf("DeepCopy array: result.Get(%d) error: %v", i, err)

			continue
		}

		if v != want {
			t.Errorf("DeepCopy array: result[%d] = %v, want %v", i, v, want)
		}
	}
}

// Test_DeepCopy_ArrayIsIndependentFromSource verifies that mutating the copy
// does not affect the original, and vice versa (true independence).
func Test_DeepCopy_ArrayIsIndependentFromSource(t *testing.T) {
	source := data.NewArrayFromInterfaces(data.IntType, 10, 20, 30)

	got := DeepCopy(source, MaxDeepCopyDepth)
	result := got.(*data.Array)

	// Mutate the copy.
	_ = result.Set(0, 999)

	// The source must be unchanged.
	v0, _ := source.Get(0)
	if v0 != 10 {
		t.Errorf("DeepCopy array: mutating copy changed source[0] to %v, want 10", v0)
	}
}

// ---- Package ----

// Test_DeepCopy_Package verifies that a *data.Package is deep-copied: the
// result is a separate package with the same key-value pairs.
func Test_DeepCopy_Package(t *testing.T) {
	pkg := &data.Package{}
	pkg.Set("pi", 3.14159)
	pkg.Set("e", 2.71828)

	got := DeepCopy(pkg, MaxDeepCopyDepth)

	result, ok := got.(*data.Package)
	if !ok {
		t.Fatalf("DeepCopy(*data.Package) returned %T, want *data.Package", got)
	}

	pi, found := result.Get("pi")
	if !found {
		t.Fatal("DeepCopy package: key \"pi\" missing from copy")
	}

	if pi != 3.14159 {
		t.Errorf("DeepCopy package: copy[\"pi\"] = %v, want 3.14159", pi)
	}
}

// ---- Unknown types ----

// Test_DeepCopy_UnknownTypeReturnsValue verifies that an unrecognized type
// falls through to the default case which returns the value as-is.
// This mirrors the behavior for basic Go types not in the switch.
func Test_DeepCopy_UnknownTypeReturnsValue(t *testing.T) {
	// A struct not in the switch falls through to the default return.
	type myStruct struct{ X int }

	src := myStruct{X: 7}

	got := DeepCopy(src, MaxDeepCopyDepth)

	v, ok := got.(myStruct)
	if !ok {
		t.Fatalf("DeepCopy(unknown struct) returned %T, want myStruct", got)
	}

	if v.X != 7 {
		t.Errorf("DeepCopy(unknown struct) X = %d, want 7", v.X)
	}
}
