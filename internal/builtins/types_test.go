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
