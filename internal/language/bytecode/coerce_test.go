package bytecode

// coerce_test.go contains unit tests for the four functions in coerce.go:
//
//   - coerceByteCode  – the Coerce bytecode instruction handler; converts the
//                       top-of-stack value to the type specified by the operand
//   - coerceStruct    – helper: validates and fills missing fields when the
//                       target type is a struct
//   - requireMatch    – helper: enforces strict type matching (used in strict
//                       type-enforcement mode)
//   - NeedsCoerce     – compiler utility: reports whether the bytecode stream
//                       needs a Coerce instruction for a given target type
//
// # How Coerce works
//
// The Coerce bytecode instruction is emitted by the compiler whenever a value
// must be converted to a specific type — for example, when initializing a
// typed array, assigning to a typed variable, or passing arguments to a
// function with a strict declaration.
//
// The instruction's operand is the TARGET TYPE (a *data.Type value).
// coerceByteCode pops the top of the stack, converts the value to the target
// type, and pushes the converted value back.
//
// The conversion rules are:
//  1. If the value is an Immutable (compile-time constant), it is always
//     coerced regardless of the current type-strictness setting.
//  2. In StrictTypeEnforcement mode without an Immutable value, the types
//     must match exactly (requireMatch is called).
//  3. Composite types (Map, Struct, Array) must already be the correct type;
//     if not, ErrInvalidType is returned.
//  4. Scalar types (int, float64, bool, string, …) are converted using the
//     data package's accessor functions.
//  5. Unknown types fall through to a default case that handles nil, same-type
//     values, and *data.Array element-by-element coercion.
//
// # How to read these tests
//
// Each test uses newTestContext (see testhelpers_test.go), optionally pushes
// a value onto the stack, then calls coerceByteCode (or a helper) directly
// and asserts the resulting stack content or error.
//
// The operand to coerceByteCode is always a *data.Type — the target type to
// coerce the TOS value into.
//
// # Bugs documented here
//
// COERCE-1  NeedsCoerce returns the wrong answer for Push instructions whose
//           operand does NOT match the target type.

import (
	"testing"

	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
)

// ─────────────────────────────────────────────────────────────────────────────
// Section 1: coerceByteCode — error cases
// ─────────────────────────────────────────────────────────────────────────────

// Test_coerceByteCode_StackUnderflow verifies that coerceByteCode returns
// ErrStackUnderflow when there is nothing on the stack to coerce.
func Test_coerceByteCode_StackUnderflow(t *testing.T) {
	tc := newTestContext(t) // empty stack

	err := coerceByteCode(tc.ctx, data.IntType)

	tc.assertError(err, errors.ErrStackUnderflow)
}

// Test_coerceByteCode_StackMarkerOnTOS verifies that a StackMarker on the
// top of stack returns ErrFunctionReturnedVoid.  A marker is placed there
// when a function returns void (no value); attempting to coerce such a
// void-result is an error.
func Test_coerceByteCode_StackMarkerOnTOS(t *testing.T) {
	tc := newTestContext(t).withStack(NewStackMarker("results"))

	err := coerceByteCode(tc.ctx, data.IntType)

	tc.assertError(err, errors.ErrFunctionReturnedVoid)
}

// Test_coerceByteCode_MapTypeMismatch verifies that coercing a value to a Map
// type returns ErrInvalidType when the value is not already a map of that
// type.  Map, Struct, and Array coercion requires an exact type match; the
// instruction does not attempt deep conversion for these composite types.
func Test_coerceByteCode_MapTypeMismatch(t *testing.T) {
	tc := newTestContext(t).withStack(42) // an int, not a map

	mapType := data.MapType(data.StringType, data.IntType)

	err := coerceByteCode(tc.ctx, mapType)

	tc.assertError(err, errors.ErrInvalidType)
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 2: coerceByteCode — integer scalar conversions
// ─────────────────────────────────────────────────────────────────────────────
//
// The following tests each push a float64 (or int) value onto the stack and
// coerce it to one of the supported integer types.  This mirrors what the
// compiler does when initializing a typed variable or array element.

// Test_coerceByteCode_ToInt verifies that a float64 value is truncated to
// a plain Go int when coerced to data.IntType.
func Test_coerceByteCode_ToInt(t *testing.T) {
	tc := newTestContext(t).withStack(float64(7.9))

	err := coerceByteCode(tc.ctx, data.IntType)

	tc.assertNoError(err)
	tc.assertTopStack(7) // truncated toward zero
}

// Test_coerceByteCode_ToInt32 verifies int32 coercion.
func Test_coerceByteCode_ToInt32(t *testing.T) {
	tc := newTestContext(t).withStack(99)

	err := coerceByteCode(tc.ctx, data.Int32Type)

	tc.assertNoError(err)
	tc.assertTopStack(int32(99))
}

// Test_coerceByteCode_ToInt64 verifies int64 coercion.
func Test_coerceByteCode_ToInt64(t *testing.T) {
	tc := newTestContext(t).withStack(42)

	err := coerceByteCode(tc.ctx, data.Int64Type)

	tc.assertNoError(err)
	tc.assertTopStack(int64(42))
}

// Test_coerceByteCode_ToInt16 verifies int16 coercion.
func Test_coerceByteCode_ToInt16(t *testing.T) {
	tc := newTestContext(t).withStack(100)

	err := coerceByteCode(tc.ctx, data.Int16Type)

	tc.assertNoError(err)
	tc.assertTopStack(int16(100))
}

// Test_coerceByteCode_ToInt8 verifies int8 coercion.
func Test_coerceByteCode_ToInt8(t *testing.T) {
	tc := newTestContext(t).withStack(127)

	err := coerceByteCode(tc.ctx, data.Int8Type)

	tc.assertNoError(err)
	tc.assertTopStack(int8(127))
}

// Test_coerceByteCode_ToUInt verifies the COERCE-2 fix: coercing to UIntType
// now returns a clean uint value rather than panicking.
//
// The bug was that data.Coerce(v, UIntType) routed through coerceUInt64,
// which returned uint64.  data.UInt() then asserted b.(uint) on that uint64
// value, causing a runtime panic.  The fix adds a coerceUInt helper that
// returns uint and wires it into the case uint: branch of Coerce.
func Test_coerceByteCode_ToUInt(t *testing.T) {
	tc := newTestContext(t).withStack(10)

	err := coerceByteCode(tc.ctx, data.UIntType)

	tc.assertNoError(err)
	tc.assertTopStack(uint(10))
}

// Test_coerceByteCode_ToUInt16 verifies uint16 coercion.
func Test_coerceByteCode_ToUInt16(t *testing.T) {
	tc := newTestContext(t).withStack(500)

	err := coerceByteCode(tc.ctx, data.UInt16Type)

	tc.assertNoError(err)
	tc.assertTopStack(uint16(500))
}

// Test_coerceByteCode_ToUInt32 verifies uint32 coercion.
func Test_coerceByteCode_ToUInt32(t *testing.T) {
	tc := newTestContext(t).withStack(1000)

	err := coerceByteCode(tc.ctx, data.UInt32Type)

	tc.assertNoError(err)
	tc.assertTopStack(uint32(1000))
}

// Test_coerceByteCode_ToUInt64 verifies uint64 coercion.
func Test_coerceByteCode_ToUInt64(t *testing.T) {
	tc := newTestContext(t).withStack(50)

	err := coerceByteCode(tc.ctx, data.UInt64Type)

	tc.assertNoError(err)
	tc.assertTopStack(uint64(50))
}

// Test_coerceByteCode_ToByte verifies byte coercion.
func Test_coerceByteCode_ToByte(t *testing.T) {
	tc := newTestContext(t).withStack(65)

	err := coerceByteCode(tc.ctx, data.ByteType)

	tc.assertNoError(err)
	tc.assertTopStack(byte(65))
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 3: coerceByteCode — float and other scalar conversions
// ─────────────────────────────────────────────────────────────────────────────

// Test_coerceByteCode_ToFloat64 verifies that an int is widened to float64.
func Test_coerceByteCode_ToFloat64(t *testing.T) {
	tc := newTestContext(t).withStack(3)

	err := coerceByteCode(tc.ctx, data.Float64Type)

	tc.assertNoError(err)
	tc.assertTopStack(float64(3))
}

// Test_coerceByteCode_ToFloat32 verifies float32 coercion from float64.
func Test_coerceByteCode_ToFloat32(t *testing.T) {
	tc := newTestContext(t).withStack(float64(1.5))

	err := coerceByteCode(tc.ctx, data.Float32Type)

	tc.assertNoError(err)
	tc.assertTopStack(float32(1.5))
}

// Test_coerceByteCode_ToBool_True verifies that a non-zero int coerces to true.
func Test_coerceByteCode_ToBool_True(t *testing.T) {
	tc := newTestContext(t).withStack(1)

	err := coerceByteCode(tc.ctx, data.BoolType)

	tc.assertNoError(err)
	tc.assertTopStack(true)
}

// Test_coerceByteCode_ToBool_False verifies that 0 coerces to false.
func Test_coerceByteCode_ToBool_False(t *testing.T) {
	tc := newTestContext(t).withStack(0)

	err := coerceByteCode(tc.ctx, data.BoolType)

	tc.assertNoError(err)
	tc.assertTopStack(false)
}

// Test_coerceByteCode_ToString verifies that any value is formatted as a
// string when the target type is data.StringType.
func Test_coerceByteCode_ToString(t *testing.T) {
	tc := newTestContext(t).withStack(123)

	err := coerceByteCode(tc.ctx, data.StringType)

	tc.assertNoError(err)
	tc.assertTopStack("123")
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 4: coerceByteCode — no-op types (push value unchanged)
// ─────────────────────────────────────────────────────────────────────────────
//
// ErrorKind, InterfaceKind, MapKind (matching), and UndefinedKind all have
// empty switch bodies.  The value is pushed onto the stack unchanged.

// Test_coerceByteCode_ErrorKind_PushesUnchanged verifies that coercing to
// ErrorType leaves the value on the stack untouched.
func Test_coerceByteCode_ErrorKind_PushesUnchanged(t *testing.T) {
	tc := newTestContext(t).withStack(errors.ErrAssert)

	err := coerceByteCode(tc.ctx, data.ErrorType)

	tc.assertNoError(err)

	v, _ := tc.ctx.Pop()
	if v == nil {
		t.Error("expected the error value to remain on the stack")
	}
}

// Test_coerceByteCode_InterfaceKind_PushesUnchanged verifies that coercing to
// InterfaceType leaves any value untouched.
func Test_coerceByteCode_InterfaceKind_PushesUnchanged(t *testing.T) {
	tc := newTestContext(t).withStack(42)

	err := coerceByteCode(tc.ctx, data.InterfaceType)

	tc.assertNoError(err)
	tc.assertTopStack(42)
}

// Test_coerceByteCode_MapKind_MatchingType verifies that a map value is pushed
// unchanged when the target type matches the map's actual type.
func Test_coerceByteCode_MapKind_MatchingType(t *testing.T) {
	mapType := data.MapType(data.StringType, data.IntType)
	mapVal := data.NewMap(data.StringType, data.IntType)

	tc := newTestContext(t).withStack(mapVal)

	err := coerceByteCode(tc.ctx, mapType)

	tc.assertNoError(err)

	v, _ := tc.ctx.Pop()
	if v != mapVal {
		t.Errorf("expected the same map value, got %T", v)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 5: coerceByteCode — Immutable (compile-time constant) values
// ─────────────────────────────────────────────────────────────────────────────
//
// An Immutable wraps a constant value.  When coerceByteCode finds an Immutable
// on the stack it unwraps it and sets coerceOk=true, which bypasses the strict
// type-enforcement check.  This allows literal constants to be coerced even in
// strict mode.

// Test_coerceByteCode_Immutable_CoercedInStrictMode verifies that an Immutable
// constant IS coerced even when the context is in StrictTypeEnforcement mode.
// Without the Immutable wrapper, coercing an int to float64 in strict mode
// would return ErrTypeMismatch.
func Test_coerceByteCode_Immutable_CoercedInStrictMode(t *testing.T) {
	tc := newTestContext(t).
		withTypeStrictness(defs.StrictTypeEnforcement).
		withStack(data.Constant(3)) // Immutable wrapping int 3

	err := coerceByteCode(tc.ctx, data.Float64Type)

	tc.assertNoError(err)
	tc.assertTopStack(float64(3))
}

// Test_coerceByteCode_Immutable_Int_ToFloat64 verifies the standard Immutable
// coercion path in default (dynamic) mode.
func Test_coerceByteCode_Immutable_Int_ToFloat64(t *testing.T) {
	tc := newTestContext(t).withStack(data.Constant(7))

	err := coerceByteCode(tc.ctx, data.Float64Type)

	tc.assertNoError(err)
	tc.assertTopStack(float64(7))
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 6: coerceByteCode — strict type enforcement (no Immutable)
// ─────────────────────────────────────────────────────────────────────────────
//
// In StrictTypeEnforcement mode, a non-constant value must already be the
// exact target type.  requireMatch is called; it pushes the value on success
// or returns ErrTypeMismatch on failure.

// Test_coerceByteCode_StrictMode_MatchingType verifies that an exact-type
// match in strict mode succeeds and pushes the value.
func Test_coerceByteCode_StrictMode_MatchingType(t *testing.T) {
	tc := newTestContext(t).
		withTypeStrictness(defs.StrictTypeEnforcement).
		withStack(42) // int, coercing to IntType

	err := coerceByteCode(tc.ctx, data.IntType)

	tc.assertNoError(err)
	tc.assertTopStack(42)
}

// Test_coerceByteCode_StrictMode_MismatchedType verifies that a type mismatch
// in strict mode returns ErrTypeMismatch.  int != float64 in strict mode.
func Test_coerceByteCode_StrictMode_MismatchedType(t *testing.T) {
	tc := newTestContext(t).
		withTypeStrictness(defs.StrictTypeEnforcement).
		withStack(42) // int, trying to coerce to float64

	err := coerceByteCode(tc.ctx, data.Float64Type)

	tc.assertError(err, errors.ErrTypeMismatch)
}

// Test_coerceByteCode_StrictMode_InterfaceAlwaysAccepted verifies that
// interface{} (InterfaceType) always accepts any value in strict mode.
// requireMatch special-cases interface types.
func Test_coerceByteCode_StrictMode_InterfaceAlwaysAccepted(t *testing.T) {
	tc := newTestContext(t).
		withTypeStrictness(defs.StrictTypeEnforcement).
		withStack(42)

	err := coerceByteCode(tc.ctx, data.InterfaceType)

	tc.assertNoError(err)
	tc.assertTopStack(42)
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 7: coerceByteCode — struct coercion
// ─────────────────────────────────────────────────────────────────────────────
//
// Struct coercion verifies that the struct value's fields are a subset of the
// target type's fields (no unknown fields), and fills in any missing fields
// with their zero value.

// Test_coerceByteCode_Struct_ExactMatch verifies that a struct whose fields
// exactly match the target type is coerced without error.
func Test_coerceByteCode_Struct_ExactMatch(t *testing.T) {
	structType := data.StructureType(
		data.Field{Name: "x", Type: data.IntType},
		data.Field{Name: "y", Type: data.IntType},
	)

	// Create a struct with both declared fields set.
	s := data.NewStruct(structType)
	s.SetAlways("x", 1)
	s.SetAlways("y", 2)

	tc := newTestContext(t).withStack(s)

	err := coerceByteCode(tc.ctx, structType)

	tc.assertNoError(err)
}

// Test_coerceByteCode_Struct_MissingFieldGetsZeroValue verifies that a struct
// value that is MISSING a field declared in the target type has that field
// filled in with the type's zero value.  This lets a partial initializer like
// `MyStruct{x: 1}` (where y is absent) still produce a complete value.
func Test_coerceByteCode_Struct_MissingFieldGetsZeroValue(t *testing.T) {
	structType := data.StructureType(
		data.Field{Name: "x", Type: data.IntType},
		data.Field{Name: "y", Type: data.IntType},
	)

	// Create a struct with only "x" set; "y" is absent.
	s := data.NewStruct(structType)
	s.SetAlways("x", 99)
	// deliberately do NOT set "y"

	tc := newTestContext(t).withStack(s)

	err := coerceByteCode(tc.ctx, structType)

	tc.assertNoError(err)

	result, _ := tc.ctx.Pop()

	resultStruct, ok := result.(*data.Struct)
	if !ok {
		t.Fatalf("expected *data.Struct on stack, got %T", result)
	}

	// "y" must now have been added with the zero value for IntType.
	yVal, found := resultStruct.Get("y")
	if !found {
		t.Fatal("expected 'y' to be filled in with zero value, but it is absent")
	}

	if yVal == nil {
		t.Error("expected 'y' to be a non-nil zero value, got nil")
	}
}

// Test_coerceByteCode_Struct_ExtraFieldRejected verifies that a struct whose
// value contains a field NOT declared in the target type returns an error.
// This prevents assigning a struct value with unknown fields to a type that
// doesn't declare those fields.
func Test_coerceByteCode_Struct_ExtraFieldRejected(t *testing.T) {
	narrowType := data.StructureType(
		data.Field{Name: "x", Type: data.IntType},
	)

	// Create a struct with "x" and an extra field "z" not in narrowType.
	wideType := data.StructureType(
		data.Field{Name: "x", Type: data.IntType},
		data.Field{Name: "z", Type: data.StringType},
	)
	s := data.NewStruct(wideType)
	s.SetAlways("x", 10)
	s.SetAlways("z", "extra")

	tc := newTestContext(t).withStack(s)

	// Trying to coerce a wider struct into a narrower type must fail.
	err := coerceByteCode(tc.ctx, narrowType)

	if err == nil {
		t.Error("expected error for struct with extra fields, got nil")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 8: coerceByteCode — default case (nil, same-type, array coercion)
// ─────────────────────────────────────────────────────────────────────────────

// Test_coerceByteCode_Default_Nil verifies that a nil value on the stack is
// pushed back as nil when the target type is not a handled scalar.
func Test_coerceByteCode_Default_Nil(t *testing.T) {
	tc := newTestContext(t).withStack(nil)

	// Use a FunctionType target which falls through to the default case.
	fnType := data.FunctionType(&data.Function{
		Declaration: &data.Declaration{Name: "fn"},
	})

	err := coerceByteCode(tc.ctx, fnType)

	tc.assertNoError(err)
	tc.assertTopStack(nil)
}

// Test_coerceByteCode_Default_SameTypePushedUnchanged verifies that when the
// value's type already matches the target (in the default case), the value is
// pushed back unchanged.
func Test_coerceByteCode_Default_SameTypePushedUnchanged(t *testing.T) {
	// Use a *data.Array; coerceByteCode's switch has no case for ArrayType
	// directly — it falls into default.  If element types already match, the
	// type-check (TypeOf(v).IsType(t)) succeeds and we push unchanged.
	arrType := data.ArrayType(data.IntType)
	arr := data.NewArrayFromInterfaces(data.IntType, 1, 2, 3)

	tc := newTestContext(t).withStack(arr)

	err := coerceByteCode(tc.ctx, arrType)

	tc.assertNoError(err)

	v, _ := tc.ctx.Pop()
	if v == nil {
		t.Error("expected array value on stack, got nil")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 9: coerceStruct — helper function
// ─────────────────────────────────────────────────────────────────────────────

// Test_coerceStruct_AllFieldsPresent verifies that a struct with all required
// fields returns the same struct without modification.
func Test_coerceStruct_AllFieldsPresent(t *testing.T) {
	structType := data.StructureType(
		data.Field{Name: "a", Type: data.IntType},
		data.Field{Name: "b", Type: data.StringType},
	)

	s := data.NewStruct(structType)
	s.SetAlways("a", 1)
	s.SetAlways("b", "hello")

	result, err := coerceStruct(s, structType)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

// Test_coerceStruct_UnknownFieldReturnsError verifies that coerceStruct
// returns an error when the struct value has a field that is not declared in
// the target type.
func Test_coerceStruct_UnknownFieldReturnsError(t *testing.T) {
	narrowType := data.StructureType(
		data.Field{Name: "known", Type: data.IntType},
	)

	wideType := data.StructureType(
		data.Field{Name: "known", Type: data.IntType},
		data.Field{Name: "unknown", Type: data.StringType},
	)

	s := data.NewStruct(wideType)
	s.SetAlways("known", 5)
	s.SetAlways("unknown", "extra")

	_, err := coerceStruct(s, narrowType)

	if err == nil {
		t.Error("expected error for unknown struct field, got nil")
	}
}

// Test_coerceStruct_MissingFieldFilled verifies that a field declared in the
// type but absent from the struct value is added with a zero value.
func Test_coerceStruct_MissingFieldFilled(t *testing.T) {
	structType := data.StructureType(
		data.Field{Name: "present", Type: data.IntType},
		data.Field{Name: "absent", Type: data.BoolType},
	)

	// Only set "present"; leave "absent" out.
	s := data.NewStruct(structType)
	s.SetAlways("present", 42)

	result, err := coerceStruct(s, structType)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	rs := result.(*data.Struct)

	if _, found := rs.Get("absent"); !found {
		t.Error("'absent' field was not filled in with zero value")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 10: requireMatch — strict type matching
// ─────────────────────────────────────────────────────────────────────────────
//
// requireMatch is called by coerceByteCode in StrictTypeEnforcement mode when
// the stack value is not an Immutable constant.

// Test_requireMatch_InterfaceAlwaysMatches verifies that requireMatch always
// succeeds when the target type is interface{}, regardless of the value type.
// Interface is the Ego equivalent of Go's any.
func Test_requireMatch_InterfaceAlwaysMatches(t *testing.T) {
	tc := newTestContext(t)

	err := requireMatch(tc.ctx, data.InterfaceType, 42)

	tc.assertNoError(err)
	tc.assertTopStack(42)
}

// Test_requireMatch_PointerTypeWithNilValue verifies that a nil value is
// accepted when the target type is a pointer type (the zero value of a pointer
// is nil, so this is always valid).
func Test_requireMatch_PointerTypeWithNilValue(t *testing.T) {
	tc := newTestContext(t)

	ptrType := data.PointerType(data.IntType)

	err := requireMatch(tc.ctx, ptrType, nil)

	tc.assertNoError(err)
	tc.assertTopStack(nil)
}

// Test_requireMatch_ErrorTypeWithNilValue verifies that a nil value is
// accepted when the target type is the built-in "error" type. error's Kind is
// data.ErrorKind rather than data.InterfaceKind, so it is not caught by the
// IsInterface() check above; before this fix, a nil error return/parameter
// under strict type enforcement failed with ErrTypeMismatch even though nil
// is Go's zero value for error.
func Test_requireMatch_ErrorTypeWithNilValue(t *testing.T) {
	tc := newTestContext(t)

	err := requireMatch(tc.ctx, data.ErrorType, nil)

	tc.assertNoError(err)
	tc.assertTopStack(nil)
}

// Test_requireMatch_MatchingConcreteType verifies that when the value's type
// matches the target type exactly, the value is pushed and nil is returned.
func Test_requireMatch_MatchingConcreteType(t *testing.T) {
	tc := newTestContext(t)

	err := requireMatch(tc.ctx, data.IntType, 42) // 42 is int, target is IntType

	tc.assertNoError(err)
	tc.assertTopStack(42)
}

// Test_requireMatch_MismatchedType verifies that when the value's type does
// not match the target type, ErrTypeMismatch is returned and nothing is pushed.
func Test_requireMatch_MismatchedType(t *testing.T) {
	tc := newTestContext(t)

	err := requireMatch(tc.ctx, data.Float64Type, 42) // int != float64

	tc.assertError(err, errors.ErrTypeMismatch)
	tc.assertStackEmpty()
}

// Test_requireMatch_StringValue verifies that a string value matches StringType.
func Test_requireMatch_StringValue(t *testing.T) {
	tc := newTestContext(t)

	err := requireMatch(tc.ctx, data.StringType, "hello")

	tc.assertNoError(err)
	tc.assertTopStack("hello")
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 11: NeedsCoerce — compiler utility
// ─────────────────────────────────────────────────────────────────────────────
//
// NeedsCoerce(kind) tells the compiler whether to emit a Coerce instruction
// for the given target type, based on what the most recently emitted bytecode
// instruction was.

// Test_NeedsCoerce_EmptyBytecode verifies that NeedsCoerce returns false when
// the bytecode stream has no instructions.  There is nothing to coerce.
func Test_NeedsCoerce_EmptyBytecode(t *testing.T) {
	bc := &ByteCode{}

	if bc.NeedsCoerce(data.IntType) {
		t.Error("NeedsCoerce on empty bytecode: expected false, got true")
	}
}

// Test_NeedsCoerce_LastInstructionPush_MatchingType verifies the COERCE-1 fix:
// when the most recently emitted Push operand IS already the target type, no
// Coerce is needed and NeedsCoerce returns false.  Emitting a Coerce for a
// value that is already the right type would be a silent no-op at runtime;
// the fix avoids that redundant instruction.
func Test_NeedsCoerce_LastInstructionPush_MatchingType(t *testing.T) {
	bc := &ByteCode{}
	bc.Emit(Push, 42) // operand is int; target is IntType — already correct

	got := bc.NeedsCoerce(data.IntType)

	// After the COERCE-1 fix: types match → no Coerce needed → false.
	if got {
		t.Error("expected NeedsCoerce=false for Push(int) with IntType (already matches), got true")
	}
}

// Test_NeedsCoerce_LastInstructionPush_NonMatchingType verifies the COERCE-1
// fix: when the most recently emitted Push operand is NOT the target type,
// NeedsCoerce now returns true so the compiler emits a Coerce instruction.
//
// Before the fix, this returned false, meaning a Push of int(42) into a
// []float64 array would leave the element as int rather than float64.
func Test_NeedsCoerce_LastInstructionPush_NonMatchingType(t *testing.T) {
	bc := &ByteCode{}
	bc.Emit(Push, 42) // int operand; target type is Float64 — mismatch

	got := bc.NeedsCoerce(data.Float64Type)

	// After the COERCE-1 fix: types don't match → Coerce IS needed → true.
	if !got {
		t.Error("expected NeedsCoerce=true for Push(int) with Float64Type target, got false")
	}
}

// Test_NeedsCoerce_LastInstructionNotPush verifies that any instruction other
// than Push causes NeedsCoerce to return true.  For computed values (whose
// type is not statically known) a Coerce instruction is always needed.
func Test_NeedsCoerce_LastInstructionNotPush(t *testing.T) {
	bc := &ByteCode{}
	bc.Emit(Add) // not a Push

	got := bc.NeedsCoerce(data.IntType)

	if !got {
		t.Error("NeedsCoerce after non-Push instruction: expected true, got false")
	}
}

// Test_NeedsCoerce_MultipleInstructions verifies that NeedsCoerce inspects
// only the LAST emitted instruction, not earlier ones.  We use a non-matching
// type so the result is unambiguous: if NeedsCoerce looks at the LAST Push
// (int, target Float64) it returns true; if it looks at an earlier Push it
// would return something different.
func Test_NeedsCoerce_MultipleInstructions(t *testing.T) {
	bc := &ByteCode{}
	bc.Emit(Push, 3.14) // first instruction — float64, matches Float64Type
	bc.Emit(Push, 42)   // last instruction — int, does NOT match Float64Type

	got := bc.NeedsCoerce(data.Float64Type)

	// Last instruction is Push(int) with target Float64Type → mismatch → true.
	if !got {
		t.Error("NeedsCoerce should inspect the LAST instruction; expected true (int!=float64), got false")
	}
}
