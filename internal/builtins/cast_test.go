package builtins

// Tests for the Cast() builtin function (builtins/cast.go).
//
// Cast() is the engine behind all type-cast expressions in Ego (e.g. int(x),
// float64(y), []byte(s)).  The target type is always the LAST element of the
// argument list; the source value is the first element.

import (
	"testing"

	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/language/symbols"
)

// ---- Scalar casts ----

// Test_Cast_IntToFloat64 verifies that an integer is correctly promoted to
// float64.
func Test_Cast_IntToFloat64(t *testing.T) {
	s := symbols.NewSymbolTable("test")
	// data.InstanceOfType(float64Type) produces a float64(0) model value.
	// Cast uses the last arg as the target type.
	args := data.NewList(42, float64(0))

	got, err := Cast(s, args)
	if err != nil {
		t.Fatalf("Cast int->float64 error: %v", err)
	}

	v, ok := got.(float64)
	if !ok {
		t.Fatalf("Cast int->float64 returned %T, want float64", got)
	}

	if v != 42.0 {
		t.Errorf("Cast int->float64 = %v, want 42.0", v)
	}
}

// Test_Cast_Float64ToInt verifies that a float64 is truncated toward zero when
// cast to int.
func Test_Cast_Float64ToInt(t *testing.T) {
	s := symbols.NewSymbolTable("test")
	args := data.NewList(3.9, 0) // target model = int

	got, err := Cast(s, args)
	if err != nil {
		t.Fatalf("Cast float64->int error: %v", err)
	}

	v, ok := got.(int)
	if !ok {
		t.Fatalf("Cast float64->int returned %T, want int", got)
	}

	// Go truncates toward zero: 3.9 → 3
	if v != 3 {
		t.Errorf("Cast float64->int = %d, want 3", v)
	}
}

// Test_Cast_NilToString verifies that casting nil produces an empty string,
// consistent with the CLAUDE.md documentation.
func Test_Cast_NilToString(t *testing.T) {
	s := symbols.NewSymbolTable("test")
	args := data.NewList(nil, "")

	got, err := Cast(s, args)
	if err != nil {
		t.Fatalf("Cast nil->string error: %v", err)
	}

	v, ok := got.(string)
	if !ok {
		t.Fatalf("Cast nil->string returned %T, want string", got)
	}

	if v != "" {
		t.Errorf("Cast nil->string = %q, want empty string", v)
	}
}

// Test_Cast_StringToInt verifies that a numeric string is parsed to int.
func Test_Cast_StringToInt(t *testing.T) {
	s := symbols.NewSymbolTable("test")
	args := data.NewList("123", 0)

	got, err := Cast(s, args)
	if err != nil {
		t.Fatalf("Cast string->int error: %v", err)
	}

	v, ok := got.(int)
	if !ok {
		t.Fatalf("Cast string->int returned %T, want int", got)
	}

	if v != 123 {
		t.Errorf("Cast string->int = %d, want 123", v)
	}
}

// Test_Cast_IntToString verifies that an integer is formatted as a string.
func Test_Cast_IntToString(t *testing.T) {
	s := symbols.NewSymbolTable("test")
	args := data.NewList(42, "")

	got, err := Cast(s, args)
	if err != nil {
		t.Fatalf("Cast int->string error: %v", err)
	}

	v, ok := got.(string)
	if !ok {
		t.Fatalf("Cast int->string returned %T, want string", got)
	}

	if v != "42" {
		t.Errorf("Cast int->string = %q, want \"42\"", v)
	}
}

// ---- Array casts ----

// Test_Cast_ByteArrayToString verifies that a []byte array is converted to
// a string.
func Test_Cast_ByteArrayToString(t *testing.T) {
	s := symbols.NewSymbolTable("test")
	// Build a []byte array containing "ABC".
	arr := data.NewArrayFromBytes('A', 'B', 'C')
	args := data.NewList(arr, "")

	got, err := Cast(s, args)
	if err != nil {
		t.Fatalf("Cast []byte->string error: %v", err)
	}

	v, ok := got.(string)
	if !ok {
		t.Fatalf("Cast []byte->string returned %T, want string", got)
	}

	if v != "ABC" {
		t.Errorf("Cast []byte->string = %q, want \"ABC\"", v)
	}
}

// Test_Cast_IntArrayToString verifies that a []int (rune array) is converted
// to a string by treating each integer as a Unicode code point.
func Test_Cast_IntArrayToString(t *testing.T) {
	s := symbols.NewSymbolTable("test")
	// 65='A', 66='B', 67='C'
	arr := data.NewArrayFromInterfaces(data.IntType, 65, 66, 67)
	args := data.NewList(arr, "")

	got, err := Cast(s, args)
	if err != nil {
		t.Fatalf("Cast []int->string error: %v", err)
	}

	v, ok := got.(string)
	if !ok {
		t.Fatalf("Cast []int->string returned %T, want string", got)
	}

	if v != "ABC" {
		t.Errorf("Cast []int->string = %q, want \"ABC\"", v)
	}
}

// Test_Cast_StringToByteArray verifies that a string is converted to a
// []byte array containing the UTF-8 bytes.
func Test_Cast_StringToByteArray(t *testing.T) {
	s := symbols.NewSymbolTable("test")
	// Target type is a []byte model.
	byteModel := data.NewArray(data.ByteType, 0)
	args := data.NewList("Hi", byteModel)

	got, err := Cast(s, args)
	if err != nil {
		t.Fatalf("Cast string->[]byte error: %v", err)
	}

	result, ok := got.(*data.Array)
	if !ok {
		t.Fatalf("Cast string->[]byte returned %T, want *data.Array", got)
	}

	if result.Len() != 2 {
		t.Errorf("Cast string->[]byte length = %d, want 2", result.Len())
	}

	// First byte should be 'H' (72).
	b0, _ := result.Get(0)
	if b0 != byte('H') {
		t.Errorf("Cast string->[]byte [0] = %v, want 'H'", b0)
	}
}

// Test_Cast_StringToIntArray verifies that a string is converted to a
// []int array containing the Unicode code points (rune values).
func Test_Cast_StringToIntArray(t *testing.T) {
	s := symbols.NewSymbolTable("test")
	intModel := data.NewArray(data.IntType, 0)
	args := data.NewList("AB", intModel)

	got, err := Cast(s, args)
	if err != nil {
		t.Fatalf("Cast string->[]int error: %v", err)
	}

	result, ok := got.(*data.Array)
	if !ok {
		t.Fatalf("Cast string->[]int returned %T, want *data.Array", got)
	}

	if result.Len() != 2 {
		t.Errorf("Cast string->[]int length = %d, want 2", result.Len())
	}

	// 'A' = 65
	v0, _ := result.Get(0)
	if v0 != 65 {
		t.Errorf("Cast string->[]int [0] = %v, want 65", v0)
	}
}

// Test_Cast_IntArrayToIntArray verifies that casting an array to the same
// type returns the original array unchanged.
func Test_Cast_IntArrayToSameType(t *testing.T) {
	s := symbols.NewSymbolTable("test")
	arr := data.NewArrayFromInterfaces(data.IntType, 1, 2, 3)
	model := data.NewArray(data.IntType, 0)
	args := data.NewList(arr, model)

	got, err := Cast(s, args)
	if err != nil {
		t.Fatalf("Cast []int->[]int error: %v", err)
	}

	result, ok := got.(*data.Array)
	if !ok {
		t.Fatalf("Cast []int->[]int returned %T, want *data.Array", got)
	}

	if result.Len() != 3 {
		t.Errorf("Cast []int->[]int length = %d, want 3", result.Len())
	}
}

// Test_Cast_IntArrayToFloat64Array verifies element-by-element conversion
// when casting an integer array to a float64 array.
func Test_Cast_IntArrayToFloat64Array(t *testing.T) {
	s := symbols.NewSymbolTable("test")
	arr := data.NewArrayFromInterfaces(data.IntType, 1, 2, 3)
	model := data.NewArray(data.Float64Type, 0)
	args := data.NewList(arr, model)

	got, err := Cast(s, args)
	if err != nil {
		t.Fatalf("Cast []int->[]float64 error: %v", err)
	}

	result, ok := got.(*data.Array)
	if !ok {
		t.Fatalf("Cast []int->[]float64 returned %T, want *data.Array", got)
	}

	v0, _ := result.Get(0)
	if v0 != float64(1) {
		t.Errorf("Cast []int->[]float64 [0] = %v, want 1.0", v0)
	}
}

// ---- Interface cast ----

// Test_Cast_ToInterfaceWrapsValue verifies that casting to interface{} wraps
// the value in a data.Interface wrapper.
func Test_Cast_ToInterfaceWrapsValue(t *testing.T) {
	s := symbols.NewSymbolTable("test")
	// The target model is a data.Interface zero value.
	args := data.NewList(42, data.Interface{})

	got, err := Cast(s, args)
	if err != nil {
		t.Fatalf("Cast to interface error: %v", err)
	}

	// The result should be a wrapped interface.
	if _, ok := got.(data.Interface); !ok {
		t.Errorf("Cast to interface returned %T, want data.Interface", got)
	}
}

// ---- Character literal cast (castToStringValue) ----

// Test_CastToStringValue_ASCIICharLiteral verifies that a 3-character string
// containing an ASCII character literal is converted to int32.
// For example, the string "'A'" (single-quoted A) returns int32(65).
func Test_CastToStringValue_ASCIICharLiteral(t *testing.T) {
	// Note: castToStringValue is an unexported helper, so we test it via Cast.
	s := symbols.NewSymbolTable("test")
	// The string "'A'" (quote, A, quote) should produce int32('A') = 65.
	args := data.NewList("'A'", int32(0))

	got, err := Cast(s, args)
	if err != nil {
		t.Fatalf("Cast char literal error: %v", err)
	}

	v, ok := got.(int32)
	if !ok {
		t.Fatalf("Cast char literal returned %T, want int32", got)
	}

	if v != 65 {
		t.Errorf("Cast char literal = %d, want 65 ('A')", v)
	}
}

// Test_CastToStringValue_MultibyteCharLiteral verifies that a multi-byte Unicode
// character literal string is correctly cast to int32.
//
// BUILTIN-CAST-1 is resolved: the rune-based check handles any Unicode code point,
// not just ASCII.  'é' is U+00E9 = 233.
func Test_CastToStringValue_MultibyteCharLiteral(t *testing.T) {
	s := symbols.NewSymbolTable("test")
	// 'é' is U+00E9.  Its UTF-8 encoding is 2 bytes, making the string "'é'"
	// have byte length 4, but rune length 3 — the fix uses []rune so this works.
	args := data.NewList("'é'", int32(0))

	got, err := Cast(s, args)
	if err != nil {
		t.Fatalf("Cast multi-byte char literal error: %v", err)
	}

	v, ok := got.(int32)
	if !ok {
		t.Fatalf("Cast multi-byte char literal returned %T, want int32", got)
	}

	// U+00E9 = 233
	if v != int32(0xE9) {
		t.Errorf("Cast 'é' = %d, want 233 (U+00E9)", v)
	}
}
