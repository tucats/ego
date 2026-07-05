package reflect

// Tests for describeType, which implements the reflect.Type() Ego function
// (runtime/reflect/type.go). It is a near-duplicate of builtins.typeOf,
// which implements the built-in typeof() keyword (see the BUILTIN-TYPES-1
// comment in builtins/types.go) -- the two are kept in sync by convention.

import (
	"testing"

	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/language/symbols"
)

// Every value Ego's "&name" address-of operator produces is represented
// internally as a *any -- a raw Go pointer to the named symbol's storage
// slot (see bytecode.addressOfByteCode and symbols.SymbolTable.GetAddress) --
// regardless of what concrete type is stored in that slot. Before fixing
// the BUG-34 follow-up below, describeType unconditionally reported every
// single one of these as the generic data.PointerType(data.InterfaceType)
// (i.e. "*interface{}"), discarding the pointed-to type entirely:
// reflect.Type(&someInt) and reflect.Type(&someString) were
// indistinguishable. The tests below build a *any exactly the way
// GetAddress does (take the address of a local "any" variable holding a
// concrete value) and confirm the pointed-to type is now preserved.

// Test_describeType_PointerToIntReturnsPointerToIntType verifies that
// reflect.Type(&x) for an int x reports "*int", not the generic
// "*interface{}".
func Test_describeType_PointerToIntReturnsPointerToIntType(t *testing.T) {
	s := symbols.NewSymbolTable("test")

	var v any = 42

	p := &v
	args := data.NewList(p)

	got, err := describeType(s, args)
	if err != nil {
		t.Fatalf("describeType(*any -> int) error: %v", err)
	}

	tp, ok := got.(*data.Type)
	if !ok {
		t.Fatalf("describeType(*any -> int) returned %T, want *data.Type", got)
	}

	if !tp.IsPointer() {
		t.Fatalf("describeType(*any -> int) = %v, want a pointer type", tp)
	}

	if !tp.BaseType().IsType(data.IntType) {
		t.Errorf("describeType(*any -> int) base type = %v, want IntType", tp.BaseType())
	}
}

// Test_describeType_PointerToStructReturnsPointerToStructType verifies that
// reflect.Type(&x) for a struct-valued x reports a pointer wrapping the
// struct's own type, not the generic "*interface{}".
func Test_describeType_PointerToStructReturnsPointerToStructType(t *testing.T) {
	s := symbols.NewSymbolTable("test")

	box := data.NewStructFromMap(map[string]any{"n": 1})

	var v any = box

	p := &v
	args := data.NewList(p)

	got, err := describeType(s, args)
	if err != nil {
		t.Fatalf("describeType(*any -> struct) error: %v", err)
	}

	tp, ok := got.(*data.Type)
	if !ok {
		t.Fatalf("describeType(*any -> struct) returned %T, want *data.Type", got)
	}

	if !tp.IsPointer() {
		t.Fatalf("describeType(*any -> struct) = %v, want a pointer type", tp)
	}

	if tp.BaseType().Kind() != data.StructKind {
		t.Errorf("describeType(*any -> struct) base type = %v, want a struct kind", tp.BaseType())
	}
}

// Test_describeType_PointerToNilInterfaceFallsBackToGenericPointerType
// verifies that a pointer to an as-yet-unassigned interface{} slot (the
// dereferenced value is nil) still falls back to the old, generic
// data.PointerType(data.InterfaceType) answer rather than erroring or
// panicking on the nil.
func Test_describeType_PointerToNilInterfaceFallsBackToGenericPointerType(t *testing.T) {
	s := symbols.NewSymbolTable("test")

	var v any // nil

	p := &v
	args := data.NewList(p)

	got, err := describeType(s, args)
	if err != nil {
		t.Fatalf("describeType(*any -> nil) error: %v", err)
	}

	tp, ok := got.(*data.Type)
	if !ok {
		t.Fatalf("describeType(*any -> nil) returned %T, want *data.Type", got)
	}

	if !tp.IsPointer() {
		t.Fatalf("describeType(*any -> nil) = %v, want a pointer type", tp)
	}

	if !tp.BaseType().IsInterface() {
		t.Errorf("describeType(*any -> nil) base type = %v, want InterfaceType", tp.BaseType())
	}
}
