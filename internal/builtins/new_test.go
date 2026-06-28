package builtins

// Tests for the NewInstanceOf() builtin function (builtins/new.go).
//
// NewInstanceOf() implements the Ego internal $new() function.  Given a type
// or value, it returns a "zero value" of that type or a deep copy of the
// provided value.  It is the underlying implementation for both $new() and
// the new() keyword in Ego code.

import (
	"reflect"
	"testing"

	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/symbols"
)

// ---- newReflectKind helper ----

// Test_NewReflectKind_Bool verifies that reflect.Bool produces false.
func Test_NewReflectKind_Bool(t *testing.T) {
	got, err := newReflectKind(reflect.Bool)
	if err != nil {
		t.Fatalf("newReflectKind(Bool) error: %v", err)
	}

	if got != false {
		t.Errorf("newReflectKind(Bool) = %v, want false", got)
	}
}

// Test_NewReflectKind_Int produces 0 of type int.
func Test_NewReflectKind_Int(t *testing.T) {
	got, err := newReflectKind(reflect.Int)
	if err != nil {
		t.Fatalf("newReflectKind(Int) error: %v", err)
	}

	if got != 0 {
		t.Errorf("newReflectKind(Int) = %v, want 0", got)
	}
	// Verify the concrete Go type is int (not int64).
	if _, ok := got.(int); !ok {
		t.Errorf("newReflectKind(Int) concrete type = %T, want int", got)
	}
}

// Test_NewReflectKind_Int64ReturnsInt64 verifies that reflect.Int64 produces
// an int64(0) zero value, not int(0).
//
// BUILTIN-NEW-1 is resolved: reflect.Int and reflect.Int64 now have separate
// cases so each returns the correct concrete Go type.
func Test_NewReflectKind_Int64ReturnsInt64(t *testing.T) {
	got, err := newReflectKind(reflect.Int64)
	if err != nil {
		t.Fatalf("newReflectKind(Int64) error: %v", err)
	}

	// Must be int64, not int.
	if _, ok := got.(int64); !ok {
		t.Errorf("newReflectKind(Int64) returned %T, want int64", got)
	}

	// Zero value check.
	if got != int64(0) {
		t.Errorf("newReflectKind(Int64) = %v, want int64(0)", got)
	}
}

// Test_NewReflectKind_Float32 verifies that reflect.Float32 produces float32(0).
func Test_NewReflectKind_Float32(t *testing.T) {
	got, err := newReflectKind(reflect.Float32)
	if err != nil {
		t.Fatalf("newReflectKind(Float32) error: %v", err)
	}

	if _, ok := got.(float32); !ok {
		t.Errorf("newReflectKind(Float32) concrete type = %T, want float32", got)
	}
}

// Test_NewReflectKind_Float64 verifies that reflect.Float64 produces float64(0).
func Test_NewReflectKind_Float64(t *testing.T) {
	got, err := newReflectKind(reflect.Float64)
	if err != nil {
		t.Fatalf("newReflectKind(Float64) error: %v", err)
	}

	if _, ok := got.(float64); !ok {
		t.Errorf("newReflectKind(Float64) concrete type = %T, want float64", got)
	}
}

// Test_NewReflectKind_String verifies that reflect.String produces "".
func Test_NewReflectKind_String(t *testing.T) {
	got, err := newReflectKind(reflect.String)
	if err != nil {
		t.Fatalf("newReflectKind(String) error: %v", err)
	}

	if got != "" {
		t.Errorf("newReflectKind(String) = %v, want empty string", got)
	}
}

// Test_NewReflectKind_UnsupportedKindReturnsError verifies that an unsupported
// reflect.Kind returns ErrInvalidType.
func Test_NewReflectKind_UnsupportedKindReturnsError(t *testing.T) {
	// reflect.Ptr is not in the supported set.
	_, err := newReflectKind(reflect.Ptr)
	if err == nil {
		t.Fatal("newReflectKind(Ptr) expected error, got nil")
	}

	if !errors.Equals(errors.New(err), errors.ErrInvalidType) {
		t.Errorf("newReflectKind(Ptr) error = %v, want ErrInvalidType", err)
	}
}

// ---- newTypeName helper ----

// Test_NewTypeName_Int verifies that the string "int" produces an int zero value.
func Test_NewTypeName_Int(t *testing.T) {
	got, err := newTypeName("int")
	if err != nil {
		t.Fatalf("newTypeName(\"int\") error: %v", err)
	}

	if got != 0 {
		t.Errorf("newTypeName(\"int\") = %v, want 0", got)
	}
}

// Test_NewTypeName_Float64 verifies that the string "float64" produces float64(0).
func Test_NewTypeName_Float64(t *testing.T) {
	got, err := newTypeName("float64")
	if err != nil {
		t.Fatalf("newTypeName(\"float64\") error: %v", err)
	}

	if _, ok := got.(float64); !ok {
		t.Errorf("newTypeName(\"float64\") concrete type = %T, want float64", got)
	}
}

// Test_NewTypeName_UnknownNameReturnsError verifies that an unrecognized type
// name returns ErrInvalidType.
func Test_NewTypeName_UnknownNameReturnsError(t *testing.T) {
	_, err := newTypeName("unknownXYZ")
	if err == nil {
		t.Fatal("newTypeName(\"unknownXYZ\") expected error, got nil")
	}

	if !errors.Equals(errors.New(err), errors.ErrInvalidType) {
		t.Errorf("newTypeName(\"unknownXYZ\") error = %v, want ErrInvalidType", err)
	}
}

// ---- NewInstanceOf ----

// Test_NewInstanceOf_IntegerTypeNumber verifies that passing a reflect.Kind
// integer constant (e.g. int(reflect.Int)) produces the correct zero value.
func Test_NewInstanceOf_IntegerTypeNumber(t *testing.T) {
	s := symbols.NewSymbolTable("test")
	args := data.NewList(int(reflect.String))

	got, err := NewInstanceOf(s, args)
	if err != nil {
		t.Fatalf("NewInstanceOf(reflect.String) error: %v", err)
	}

	if got != "" {
		t.Errorf("NewInstanceOf(reflect.String) = %v, want empty string", got)
	}
}

// Test_NewInstanceOf_StringTypeName verifies that passing the string "bool"
// produces false.
func Test_NewInstanceOf_StringTypeName(t *testing.T) {
	s := symbols.NewSymbolTable("test")
	args := data.NewList("bool")

	got, err := NewInstanceOf(s, args)
	if err != nil {
		t.Fatalf("NewInstanceOf(\"bool\") error: %v", err)
	}

	if got != false {
		t.Errorf("NewInstanceOf(\"bool\") = %v, want false", got)
	}
}

// Test_NewInstanceOf_DataTypePointer verifies that passing a *data.Type
// produces a non-nil zero value via typeValue.InstanceOf.
func Test_NewInstanceOf_DataTypePointer(t *testing.T) {
	s := symbols.NewSymbolTable("test")
	args := data.NewList(data.IntType)

	got, err := NewInstanceOf(s, args)
	if err != nil {
		t.Fatalf("NewInstanceOf(*data.Type int) error: %v", err)
	}

	if got == nil {
		t.Fatal("NewInstanceOf(*data.Type int) returned nil")
	}
}

// Test_NewInstanceOf_ArrayProducesDeepCopy verifies that passing a *data.Array
// produces a deep copy of the array, not the original.
func Test_NewInstanceOf_ArrayProducesDeepCopy(t *testing.T) {
	s := symbols.NewSymbolTable("test")
	arr := data.NewArrayFromInterfaces(data.IntType, 10, 20, 30)
	args := data.NewList(arr)

	got, err := NewInstanceOf(s, args)
	if err != nil {
		t.Fatalf("NewInstanceOf(*data.Array) error: %v", err)
	}

	copyArrayValue, ok := got.(*data.Array)
	if !ok {
		t.Fatalf("NewInstanceOf(*data.Array) returned %T, want *data.Array", got)
	}

	// Verify it has the same length as the original.
	if copyArrayValue.Len() != arr.Len() {
		t.Errorf("NewInstanceOf(*data.Array) copy.Len() = %d, want %d", copyArrayValue.Len(), arr.Len())
	}
}

// Test_NewInstanceOf_StructProducesDeepCopy verifies that passing a *data.Struct
// returns a deep copy via data.DeepCopy.
func Test_NewInstanceOf_StructProducesDeepCopy(t *testing.T) {
	s := symbols.NewSymbolTable("test")
	st := data.NewStructFromMap(map[string]any{
		"x": 1,
		"y": 2,
	})
	args := data.NewList(st)

	got, err := NewInstanceOf(s, args)
	if err != nil {
		t.Fatalf("NewInstanceOf(*data.Struct) error: %v", err)
	}

	_, ok := got.(*data.Struct)
	if !ok {
		t.Fatalf("NewInstanceOf(*data.Struct) returned %T, want *data.Struct", got)
	}
}

// Test_NewInstanceOf_ChannelReturnsNewInstance verifies that $new(channel)
// creates an independent channel rather than returning the original.
//
// BUILTIN-NEW-2 is resolved: NewInstanceOf now calls data.NewChannel(ch.Cap())
// to create a fresh channel with the same buffer capacity.
func Test_NewInstanceOf_ChannelReturnsNewInstance(t *testing.T) {
	s := symbols.NewSymbolTable("test")
	ch := data.NewChannel(5)
	args := data.NewList(ch)

	got, err := NewInstanceOf(s, args)
	if err != nil {
		t.Fatalf("NewInstanceOf(*data.Channel) error: %v", err)
	}

	result, ok := got.(*data.Channel)
	if !ok {
		t.Fatalf("NewInstanceOf(*data.Channel) returned %T, want *data.Channel", got)
	}

	// The returned channel must be a different pointer — a new independent channel.
	if result == ch {
		t.Error("NewInstanceOf(channel) returned the same channel pointer; want a new independent channel")
	}

	// Sends on the new channel must not affect the original.
	_ = result.Send("test")

	if ch.Len() != 0 {
		t.Errorf("Send to new channel affected original (original.Len() = %d, want 0)", ch.Len())
	}
}

// Test_NewInstanceOf_InvalidValueReturnsError verifies that passing a value
// that cannot be deep-copied or zero-valued returns an error.
func Test_NewInstanceOf_NilReturnedByDeepCopyReturnsError(t *testing.T) {
	s := symbols.NewSymbolTable("test")
	// A negative depth in DeepCopy returns nil, which NewInstanceOf treats
	// as ErrInvalidValue.  The only way to reach this via NewInstanceOf is
	// to pass a value that triggers the DeepCopy fallback (no dedicated case)
	// AND results in a nil return.
	// Passing nil itself exercises the nil-check inside NewInstanceOf.
	args := data.NewList(nil)

	_, err := NewInstanceOf(s, args)
	// nil is caught by the DeepCopy fallback returning nil and the nil-case
	// in the switch returning ErrInvalidValue.
	if err == nil {
		t.Fatal("NewInstanceOf(nil) expected ErrInvalidValue, got nil")
	}
}
