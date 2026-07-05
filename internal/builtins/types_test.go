package builtins

// Tests for the typeOf() builtin function (builtins/types.go).
//
// typeOf() implements the Ego extension function typeof().  It returns the
// Ego *data.Type descriptor for any value.  This is useful for runtime type
// introspection in Ego programs.

import (
	"testing"

	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/language/symbols"
)

// Test_TypeOf_IntReturnsIntType verifies that typeof(int) returns the integer
// type descriptor.
func Test_TypeOf_IntReturnsIntType(t *testing.T) {
	s := symbols.NewSymbolTable("test")
	args := data.NewList(42)

	got, err := typeOf(s, args)
	if err != nil {
		t.Fatalf("typeOf(int) error: %v", err)
	}

	tp, ok := got.(*data.Type)
	if !ok {
		t.Fatalf("typeOf(int) returned %T, want *data.Type", got)
	}

	if !tp.IsType(data.IntType) {
		t.Errorf("typeOf(int) = %v, want IntType", tp)
	}
}

// Test_TypeOf_StringReturnsStringType verifies that typeof("hello") returns
// the string type descriptor.
func Test_TypeOf_StringReturnsStringType(t *testing.T) {
	s := symbols.NewSymbolTable("test")
	args := data.NewList("hello")

	got, err := typeOf(s, args)
	if err != nil {
		t.Fatalf("typeOf(string) error: %v", err)
	}

	tp, ok := got.(*data.Type)
	if !ok {
		t.Fatalf("typeOf(string) returned %T, want *data.Type", got)
	}

	if !tp.IsType(data.StringType) {
		t.Errorf("typeOf(string) = %v, want StringType", tp)
	}
}

// Test_TypeOf_NilReturnsNilType verifies that typeof(nil) returns the nil
// type descriptor.
func Test_TypeOf_NilReturnsNilType(t *testing.T) {
	s := symbols.NewSymbolTable("test")
	args := data.NewList(nil)

	got, err := typeOf(s, args)
	if err != nil {
		t.Fatalf("typeOf(nil) error: %v", err)
	}

	// nil should return data.NilType
	tp, ok := got.(*data.Type)
	if !ok {
		t.Fatalf("typeOf(nil) returned %T, want *data.Type", got)
	}

	if !tp.IsType(data.NilType) {
		t.Errorf("typeOf(nil) = %v, want NilType", tp)
	}
}

// Test_TypeOf_MapReturnsSelf verifies that typeof(map) returns the map's own
// type descriptor.
func Test_TypeOf_MapReturnsSelf(t *testing.T) {
	s := symbols.NewSymbolTable("test")
	m := data.NewMap(data.StringType, data.IntType)
	args := data.NewList(m)

	got, err := typeOf(s, args)
	if err != nil {
		t.Fatalf("typeOf(*data.Map) error: %v", err)
	}

	if got == nil {
		t.Fatal("typeOf(*data.Map) returned nil")
	}
}

// Test_TypeOf_ArrayReturnsArrayType verifies that typeof(array) returns an
// array type descriptor wrapping the element type.
func Test_TypeOf_ArrayReturnsArrayType(t *testing.T) {
	s := symbols.NewSymbolTable("test")
	arr := data.NewArrayFromInterfaces(data.IntType, 1, 2, 3)
	args := data.NewList(arr)

	got, err := typeOf(s, args)
	if err != nil {
		t.Fatalf("typeOf(*data.Array) error: %v", err)
	}

	tp, ok := got.(*data.Type)
	if !ok {
		t.Fatalf("typeOf(*data.Array) returned %T, want *data.Type", got)
	}

	// The type should be an array type.
	if !tp.IsArray() {
		t.Errorf("typeOf(*data.Array) type is not an array type: %v", tp)
	}
}

// Test_TypeOf_ChannelReturnsChanType verifies that typeof(channel) returns
// the chan type descriptor.
func Test_TypeOf_ChannelReturnsChanType(t *testing.T) {
	s := symbols.NewSymbolTable("test")
	ch := data.NewChannel(1)
	args := data.NewList(ch)

	got, err := typeOf(s, args)
	if err != nil {
		t.Fatalf("typeOf(*data.Channel) error: %v", err)
	}

	tp, ok := got.(*data.Type)
	if !ok {
		t.Fatalf("typeOf(*data.Channel) returned %T, want *data.Type", got)
	}

	if !tp.IsType(data.ChanType) {
		t.Errorf("typeOf(*data.Channel) = %v, want ChanType", tp)
	}
}

// Test_TypeOf_ErrorReturnsErrorType verifies that typeof(error) returns the
// error type descriptor.
func Test_TypeOf_ErrorReturnsErrorType(t *testing.T) {
	s := symbols.NewSymbolTable("test")
	// Use a simple error value (testError satisfies the error interface).
	e := &testError{"test error message"}
	args := data.NewList(e)

	got, err := typeOf(s, args)
	if err != nil {
		t.Fatalf("typeOf(error) error: %v", err)
	}

	// The result should be the error type descriptor.
	tp, ok := got.(*data.Type)
	if !ok {
		t.Fatalf("typeOf(error) returned %T, want *data.Type", got)
	}

	if !tp.IsType(data.ErrorType) {
		t.Errorf("typeOf(error) = %v, want ErrorType", tp)
	}
}

// testError is a minimal error implementation used in tests.
// It satisfies the error interface without depending on the errors package.
type testError struct{ msg string }

func (te *testError) Error() string { return te.msg }

// Test_TypeOf_BuiltinFunctionReturnsFuncType verifies that typeof(builtinFunc)
// returns a *data.Type of FunctionKind rather than the former string "<builtin>".
//
// BUILTIN-TYPES-1 is resolved: the implementation now constructs a minimal
// data.Function descriptor and returns data.FunctionType(&fn), consistent with
// how the data.Function case is handled.
func Test_TypeOf_BuiltinFunctionReturnsFuncType(t *testing.T) {
	s := symbols.NewSymbolTable("test")
	// Pass a native Go function matching the builtin function signature.
	fn := func(s *symbols.SymbolTable, args data.List) (any, error) {
		return nil, nil
	}
	args := data.NewList(fn)

	got, err := typeOf(s, args)
	if err != nil {
		t.Fatalf("typeOf(builtin func) error: %v", err)
	}

	// After the fix, the result must be a *data.Type, not the old string "<builtin>".
	tp, ok := got.(*data.Type)
	if !ok {
		t.Fatalf("typeOf(builtin func) returned %T (%v), want *data.Type", got, got)
	}

	// The type should be a function kind.
	if tp.Kind() != data.FunctionKind {
		t.Errorf("typeOf(builtin func).Kind() = %v, want FunctionKind", tp.Kind())
	}
}

// Test_TypeOf_PackageReturnsPackageType verifies that typeof(package) returns
// a package type descriptor.
func Test_TypeOf_PackageReturnsPackageType(t *testing.T) {
	s := symbols.NewSymbolTable("test")
	pkg := &data.Package{}
	pkg.Name = "mypkg"
	args := data.NewList(pkg)

	got, err := typeOf(s, args)
	if err != nil {
		t.Fatalf("typeOf(*data.Package) error: %v", err)
	}

	if got == nil {
		t.Fatal("typeOf(*data.Package) returned nil")
	}
}

// Test_TypeOf_FunctionValueReturnsFuncType verifies that typeof(data.Function)
// returns a function type descriptor.
func Test_TypeOf_FunctionValueReturnsFuncType(t *testing.T) {
	s := symbols.NewSymbolTable("test")

	fn := data.Function{
		Declaration: &data.Declaration{
			Name: "myFunc",
		},
	}
	args := data.NewList(fn)

	got, err := typeOf(s, args)
	if err != nil {
		t.Fatalf("typeOf(data.Function) error: %v", err)
	}

	_, ok := got.(*data.Type)
	if !ok {
		t.Errorf("typeOf(data.Function) returned %T, want *data.Type", got)
	}
}

// Every value Ego's "&name" address-of operator produces is represented
// internally as a *any -- a raw Go pointer to the named symbol's storage
// slot (see bytecode.addressOfByteCode and symbols.SymbolTable.GetAddress) --
// regardless of what concrete type is stored in that slot. Before 
// the BUG-34 follow-up fix below, typeOf unconditionally reported every single
// one of these as the generic data.PointerType(data.InterfaceType) (i.e.
// "*interface{}"), discarding the pointed-to type entirely: typeof(&someInt)
// and typeof(&someString) were indistinguishable. The tests in this section
// build a *any exactly the way GetAddress does (take the address of a local
// "any" variable holding a concrete value) and confirm the pointed-to type
// is now preserved.

// Test_TypeOf_PointerToIntReturnsPointerToIntType verifies that typeof(&x)
// for an int x reports "*int", not the generic "*interface{}".
func Test_TypeOf_PointerToIntReturnsPointerToIntType(t *testing.T) {
	s := symbols.NewSymbolTable("test")

	var v any = 42

	p := &v
	args := data.NewList(p)

	got, err := typeOf(s, args)
	if err != nil {
		t.Fatalf("typeOf(*any -> int) error: %v", err)
	}

	tp, ok := got.(*data.Type)
	if !ok {
		t.Fatalf("typeOf(*any -> int) returned %T, want *data.Type", got)
	}

	if !tp.IsPointer() {
		t.Fatalf("typeOf(*any -> int) = %v, want a pointer type", tp)
	}

	if !tp.BaseType().IsType(data.IntType) {
		t.Errorf("typeOf(*any -> int) base type = %v, want IntType", tp.BaseType())
	}
}

// Test_TypeOf_PointerToStringReturnsPointerToStringType verifies that
// typeof(&x) for a string x reports "*string".
func Test_TypeOf_PointerToStringReturnsPointerToStringType(t *testing.T) {
	s := symbols.NewSymbolTable("test")

	var v any = "hello"

	p := &v
	args := data.NewList(p)

	got, err := typeOf(s, args)
	if err != nil {
		t.Fatalf("typeOf(*any -> string) error: %v", err)
	}

	tp, ok := got.(*data.Type)
	if !ok {
		t.Fatalf("typeOf(*any -> string) returned %T, want *data.Type", got)
	}

	if !tp.IsPointer() {
		t.Fatalf("typeOf(*any -> string) = %v, want a pointer type", tp)
	}

	if !tp.BaseType().IsType(data.StringType) {
		t.Errorf("typeOf(*any -> string) base type = %v, want StringType", tp.BaseType())
	}
}

// Test_TypeOf_PointerToNilInterfaceFallsBackToGenericPointerType verifies
// that a pointer to an as-yet-unassigned interface{} slot (the dereferenced
// value is nil) still falls back to the old, generic
// data.PointerType(data.InterfaceType) answer rather than erroring or
// panicking on the nil.
func Test_TypeOf_PointerToNilInterfaceFallsBackToGenericPointerType(t *testing.T) {
	s := symbols.NewSymbolTable("test")

	var v any // nil

	p := &v
	args := data.NewList(p)

	got, err := typeOf(s, args)
	if err != nil {
		t.Fatalf("typeOf(*any -> nil) error: %v", err)
	}

	tp, ok := got.(*data.Type)
	if !ok {
		t.Fatalf("typeOf(*any -> nil) returned %T, want *data.Type", got)
	}

	if !tp.IsPointer() {
		t.Fatalf("typeOf(*any -> nil) = %v, want a pointer type", tp)
	}

	if !tp.BaseType().IsInterface() {
		t.Errorf("typeOf(*any -> nil) base type = %v, want InterfaceType", tp.BaseType())
	}
}

// Test_TypeOf_PointerToStructReturnsPointerToStructType verifies that
// typeof(&x) for a struct-valued x reports a pointer wrapping the struct's
// own type, not the generic "*interface{}".
func Test_TypeOf_PointerToStructReturnsPointerToStructType(t *testing.T) {
	s := symbols.NewSymbolTable("test")

	box := data.NewStructFromMap(map[string]any{"n": 1})

	var v any = box

	p := &v
	args := data.NewList(p)

	got, err := typeOf(s, args)
	if err != nil {
		t.Fatalf("typeOf(*any -> struct) error: %v", err)
	}

	tp, ok := got.(*data.Type)
	if !ok {
		t.Fatalf("typeOf(*any -> struct) returned %T, want *data.Type", got)
	}

	if !tp.IsPointer() {
		t.Fatalf("typeOf(*any -> struct) = %v, want a pointer type", tp)
	}

	if tp.BaseType().Kind() != data.StructKind {
		t.Errorf("typeOf(*any -> struct) base type = %v, want a struct kind", tp.BaseType())
	}
}
