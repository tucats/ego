package data

import "testing"

// Tests for Struct.Copy(), specifically the fix found while working on
// BUG-26 (struct assignment/pass-by-value did not copy structs at all).
//
// Copy() used to build its result by calling NewStructFromMap(s.fields).
// NewStructFromMap() is meant to build a struct from a generic
// "map[string]any" that might have type-metadata keys mixed in with the
// real field values (as happens when reconstructing a struct from
// JSON-like data), so it deliberately skips any key starting with
// MetadataPrefix ("__"). That filtering is wrong when the input is already
// a *Struct's own field map: some of a struct's real fields legitimately
// start with "__", most importantly NativeFieldName ("__native"), which
// packages like time, uuid, and sync use to store the wrapped Go-native
// value (a *uuid.UUID, *time.Time, *sync.Mutex, ...). Routing through
// NewStructFromMap silently dropped that field on every copy.

// TestStructCopyPreservesNativeField is a regression test for the bug
// above: SetNative stores a value under the "__native" key, and Copy()
// must preserve it.
func TestStructCopyPreservesNativeField(t *testing.T) {
	original := NewStruct(StructureType())
	original.SetNative("wrapped-native-value")

	copied := original.Copy()

	value, found := copied.Get(NativeFieldName)
	if !found {
		t.Fatalf("Copy() lost the %q field entirely", NativeFieldName)
	}

	if value != "wrapped-native-value" {
		t.Errorf("Copy() native field = %v, want %v", value, "wrapped-native-value")
	}
}

// TestStructCopyPreservesOrdinaryFields is a basic sanity check that plain,
// non-metadata-looking fields still copy correctly (this always worked, but
// is worth pinning down alongside the native-field regression test above).
func TestStructCopyPreservesOrdinaryFields(t *testing.T) {
	original := NewStruct(StructureType())
	_ = original.Set("X", 42)
	_ = original.Set("Y", "hello")

	copied := original.Copy()

	x, _ := copied.Get("X")
	if x != 42 {
		t.Errorf("Copy() field X = %v, want 42", x)
	}

	y, _ := copied.Get("Y")
	if y != "hello" {
		t.Errorf("Copy() field Y = %v, want \"hello\"", y)
	}
}

// TestStructCopyIsIndependent verifies that the copy has its own fields map:
// writing a new value into the copy must not be visible through the
// original, and vice versa. This is the core property that makes Copy()
// usable for implementing Go's struct value semantics.
func TestStructCopyIsIndependent(t *testing.T) {
	original := NewStruct(StructureType())
	_ = original.Set("X", 1)

	copied := original.Copy()
	_ = copied.Set("X", 2)

	originalX, _ := original.Get("X")
	if originalX != 1 {
		t.Errorf("writing to the copy changed the original: X = %v, want 1", originalX)
	}

	copiedX, _ := copied.Get("X")
	if copiedX != 2 {
		t.Errorf("copied.X = %v, want 2", copiedX)
	}
}
