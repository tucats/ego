package bytecode

// equal_test.go contains focused unit tests for the four functions defined
// in bytecode/equal.go:
//
//	equalByteCode       — the main == opcode handler
//	genericEqualCompare — the scalar-type fallback called by equalByteCode
//	equalTypes          — compares two *data.Type values
//	getComparisonTerms  — pops / reads the two operands for any comparison op
//
// # Relationship with compare_test.go
//
// compare_test.go already covers the most common paths through equalByteCode
// for scalar types (int, float64, string, bool, nil), time values, arrays,
// and strict-mode type errors.  This file deliberately avoids duplicating
// those cases and instead focuses on code paths that are unique to equal.go:
//
//   - equalTypes — type-vs-type and type-vs-string comparisons
//   - getComparisonTerms in operand mode (i != nil)
//   - StackMarker detection → ErrFunctionReturnedVoid
//   - Map equality / inequality
//   - Struct equality / inequality
//   - *errors.Error equality
//   - time.Duration / time.Time operand type-mismatch errors
//   - byte (uint8) equality
//   - Immutable constant coercion
//   - uint16, uint32, uint integer widths
//   - Stack discipline (exactly two inputs consumed, one bool output)
//
// # Fixes verified here
//
//	EQUAL-1  equalTypes now wraps ErrNotAType in c.runtimeError, so the
//	         error carries module-name and source-line annotation.
//	         Verified by: Test_equalTypes_TypeVsNonType,
//	                      Test_equalTypes_TypeVsNonType_ErrorIsDecorated
//
//	EQUAL-2  getComparisonTerms now wraps any data.Coerce error in
//	         c.runtimeError.  The coerce path is exercised by the
//	         ImmutableCoercion tests; the defensive error wrap cannot be
//	         triggered in practice (data.Coerce never fails for two valid
//	         numeric values) but is confirmed by code review.
//	         Verified by: Test_equalByteCode_ImmutableCoercion_PromotesConstantToFloat64
//
//	EQUAL-3  The unreachable `case nil:` branch was removed from
//	         equalByteCode's type switch.  The two data.IsNil guards above
//	         the switch handle every nil scenario before the switch runs.
//	         Verified by: Test_equalByteCode_NilNilHandledByGuard,
//	                      Test_equalByteCode_NilOneNilHandledByGuard

import (
	"testing"
	"time"

	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/errors"
)

// ─────────────────────────────────────────────────────────────────────────────
// Section 1 — equalTypes
//
// equalTypes is an internal helper called when the left-hand operand (v1) is
// a *data.Type value.  It compares v1 against either another *data.Type or a
// plain string that names a type, pushing a bool result onto the stack.
//
// Code path: equalByteCode → case *data.Type → equalTypes(v2, c, actual)
//
// The second operand (v2) may be:
//   (a) another *data.Type  — compare by String() representation
//   (b) a string            — compare against actual.String()
//   (c) anything else       — return a decorated ErrNotAType (EQUAL-1 fix)
// ─────────────────────────────────────────────────────────────────────────────

// Test_equalTypes_SameTypes verifies that two *data.Type values that represent
// the same built-in type compare as equal.
//
// Stack layout: withStack(v1, v2) pushes v1 first then v2.  getComparisonTerms
// pops v2 first (right-hand side), then v1 (left-hand side).  So the stack is
// built with the left operand at the bottom and the right operand on top.
func Test_equalTypes_SameTypes(t *testing.T) {
	// Both sides are data.IntType → their String() representations both return
	// "int", so equalTypes should push true.
	tc := newTestContext(t).withStack(data.IntType, data.IntType)

	tc.assertNoError(equalByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// Test_equalTypes_DifferentTypes verifies that two *data.Type values that
// represent different built-in types compare as not equal.
func Test_equalTypes_DifferentTypes(t *testing.T) {
	// "int" != "float64"
	tc := newTestContext(t).withStack(data.IntType, data.Float64Type)

	tc.assertNoError(equalByteCode(tc.ctx, nil))
	tc.assertTopStack(false)
}

// Test_equalTypes_TypeVsMatchingString verifies that a *data.Type compares as
// equal to the string that is its canonical name.
//
// This exercises the `if v, ok := v2.(string)` branch of equalTypes.  Ego
// code can write `typeof(x) == "int"`, which produces this case.
func Test_equalTypes_TypeVsMatchingString(t *testing.T) {
	// v1 = *data.Type for int, v2 = the string "int".
	tc := newTestContext(t).withStack(data.IntType, "int")

	tc.assertNoError(equalByteCode(tc.ctx, nil))

	// data.IntType.String() returns "int", which matches the literal string.
	tc.assertTopStack(true)
}

// Test_equalTypes_TypeVsNonMatchingString verifies that a *data.Type does not
// compare as equal to a string that names a different type.
func Test_equalTypes_TypeVsNonMatchingString(t *testing.T) {
	// data.IntType.String() = "int", which differs from "float64".
	tc := newTestContext(t).withStack(data.IntType, "float64")

	tc.assertNoError(equalByteCode(tc.ctx, nil))
	tc.assertTopStack(false)
}

// Test_equalTypes_TypeVsNonType verifies that comparing a *data.Type against a
// value that is neither a string nor a *data.Type returns ErrNotAType.
//
// EQUAL-1 fix: before the fix, this error was returned directly from
// equalTypes without going through c.runtimeError, so it carried no module or
// line information.  After the fix, c.runtimeError decorates the error.
func Test_equalTypes_TypeVsNonType(t *testing.T) {
	// v1 = *data.Type, v2 = 42 — not a type and not a string.
	tc := newTestContext(t).withStack(data.IntType, 42)

	// equalTypes must return ErrNotAType for an unrecognized v2.
	tc.assertError(equalByteCode(tc.ctx, nil), errors.ErrNotAType)
}

// Test_equalTypes_TypeVsNonType_ErrorIsDecorated verifies the EQUAL-1 fix:
// the error returned when v2 is not a type or string must now be an
// *errors.Error decorated with module-name information.
//
// Before EQUAL-1 was fixed, the code returned errors.ErrNotAType.Context(v2)
// directly.  That produced a valid *errors.Error but one whose HasIn() flag
// was false, because the raw Context() call does not invoke c.runtimeError.
// After the fix, c.runtimeError wraps the error with the current module name,
// so HasIn() becomes true whenever the context has a non-empty module name.
func Test_equalTypes_TypeVsNonType_ErrorIsDecorated(t *testing.T) {
	tc := newTestContext(t)

	// Give the context a recognizable module name so we can tell whether
	// runtimeError attached it to the error.
	tc.ctx.module = "test_module.ego"
	tc.ctx.line = 7

	// Push v1 = *data.Type, v2 = 99 (an integer — triggers ErrNotAType).
	tc.withStack(data.IntType, 99)

	err := equalByteCode(tc.ctx, nil)
	if err == nil {
		t.Fatal("expected ErrNotAType, got nil")
	}

	// The error must be the concrete *errors.Error type returned by
	// c.runtimeError.  The raw sentinel is also *errors.Error, but only the
	// runtimeError-decorated version sets HasIn() to true.
	egErr, ok := err.(*errors.Error)
	if !ok {
		t.Fatalf("expected *errors.Error, got %T", err)
	}

	// HasIn() is true only when c.runtimeError called e.In(moduleName).
	// This is the observable difference that confirms the EQUAL-1 fix is in
	// effect.
	if !egErr.HasIn() {
		t.Error("EQUAL-1: error should carry module info (HasIn() == true) after fix")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 2 — getComparisonTerms: operand mode
//
// getComparisonTerms has two modes for obtaining v2 (the right-hand operand):
//
//   Stack mode (i == nil):  v2 and v1 are both popped from the stack.
//   Operand mode (i is []any{oneValue}): v2 comes from the instruction operand
//     and only v1 is popped from the stack.
//
// Operand mode is used by the compiler when the right-hand side is a
// compile-time constant that has been folded into the instruction itself.
// ─────────────────────────────────────────────────────────────────────────────

// Test_equalByteCode_OperandMode_Simple verifies that when the instruction
// operand contains one value ([]any{v2}), that value is used as the right-hand
// operand instead of being popped from the stack.
//
// Only one value (v1 = 10) is on the stack; v2 = 10 is supplied via the
// operand.  If operand mode works, the comparison sees 10 == 10 and pushes
// true.  If it incorrectly tried to pop a second stack value it would fail
// with ErrStackUnderflow.
func Test_equalByteCode_OperandMode_Simple(t *testing.T) {
	tc := newTestContext(t).withStack(10)

	// []any{10} signals operand mode: getComparisonTerms reads v2 = 10 here
	// rather than popping it from the stack.
	tc.assertNoError(equalByteCode(tc.ctx, []any{10}))
	tc.assertTopStack(true)
}

// Test_equalByteCode_OperandMode_Inequality verifies that operand mode handles
// the not-equal case correctly.
func Test_equalByteCode_OperandMode_Inequality(t *testing.T) {
	tc := newTestContext(t).withStack(7)

	tc.assertNoError(equalByteCode(tc.ctx, []any{99}))
	tc.assertTopStack(false)
}

// Test_equalByteCode_OperandMode_ImmutableConstant verifies that when the
// operand value is a data.Immutable wrapper (a compile-time constant marker),
// getComparisonTerms correctly unwraps it before comparing.
//
// The Ego compiler marks literal constants such as the `5` in `x == 5` with
// data.Immutable so strict type-checking can coerce them to the target type.
// getComparisonTerms must peel off the wrapper before the actual comparison.
func Test_equalByteCode_OperandMode_ImmutableConstant(t *testing.T) {
	tc := newTestContext(t).withStack(5)

	// data.Immutable{Value: 5} is what the compiler emits for the literal 5.
	tc.assertNoError(equalByteCode(tc.ctx, []any{data.Immutable{Value: 5}}))
	tc.assertTopStack(true)
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 3 — getComparisonTerms: StackMarker detection
//
// A StackMarker in either operand position means a sub-expression returned
// void (no value).  getComparisonTerms must detect this and return
// ErrFunctionReturnedVoid rather than attempting to compare a sentinel.
// ─────────────────────────────────────────────────────────────────────────────

// Test_equalByteCode_StackMarker_V1_ReturnsError verifies that a StackMarker
// in the v1 (left-hand / bottom) stack position returns ErrFunctionReturnedVoid.
//
// Stack before the call (bottom → top): [StackMarker, 42]
// getComparisonTerms pops 42 as v2, then pops the marker as v1.
// Detecting the marker on v1 must immediately return the error.
func Test_equalByteCode_StackMarker_V1_ReturnsError(t *testing.T) {
	tc := newTestContext(t).withStack(NewStackMarker("test"), 42)

	tc.assertError(equalByteCode(tc.ctx, nil), errors.ErrFunctionReturnedVoid)
}

// Test_equalByteCode_StackMarker_V2_ReturnsError verifies that a StackMarker
// in the v2 (right-hand / top) stack position returns ErrFunctionReturnedVoid.
//
// Stack before the call (bottom → top): [42, StackMarker]
// The marker is the first item popped, so it is detected on v2.
func Test_equalByteCode_StackMarker_V2_ReturnsError(t *testing.T) {
	tc := newTestContext(t).withStack(42, NewStackMarker("test"))

	tc.assertError(equalByteCode(tc.ctx, nil), errors.ErrFunctionReturnedVoid)
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 4 — equalByteCode: Map equality
//
// equalByteCode has a dedicated `case *data.Map:` that uses reflect.DeepEqual.
// Maps are always stored as pointer values in Ego, so the pointer-type case
// must match.
// ─────────────────────────────────────────────────────────────────────────────

// Test_equalByteCode_MapEqual verifies that two maps with identical key/value
// pairs compare as equal.
func Test_equalByteCode_MapEqual(t *testing.T) {
	mapA := data.NewMap(data.StringType, data.IntType)
	mapB := data.NewMap(data.StringType, data.IntType)

	mapA.SetAlways("x", 1)
	mapA.SetAlways("y", 2)
	mapB.SetAlways("x", 1)
	mapB.SetAlways("y", 2)

	tc := newTestContext(t).withStack(mapA, mapB)

	tc.assertNoError(equalByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// Test_equalByteCode_MapNotEqual verifies that two maps with different contents
// compare as not equal.
func Test_equalByteCode_MapNotEqual(t *testing.T) {
	mapA := data.NewMap(data.StringType, data.IntType)
	mapB := data.NewMap(data.StringType, data.IntType)

	mapA.SetAlways("x", 1)
	mapB.SetAlways("x", 99) // different value

	tc := newTestContext(t).withStack(mapA, mapB)

	tc.assertNoError(equalByteCode(tc.ctx, nil))
	tc.assertTopStack(false)
}

// Test_equalByteCode_MapVsNonMap verifies that a *data.Map compared against a
// non-map value returns false (not an error).
//
// reflect.DeepEqual(mapA, "hello") returns false because the types differ.
func Test_equalByteCode_MapVsNonMap(t *testing.T) {
	m := data.NewMap(data.StringType, data.IntType)
	m.SetAlways("a", 1)

	tc := newTestContext(t).withStack(m, "hello")

	tc.assertNoError(equalByteCode(tc.ctx, nil))
	tc.assertTopStack(false)
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 5 — equalByteCode: Struct equality
//
// equalByteCode has a `case *data.Struct:` that uses reflect.DeepEqual.
// Structs, like maps, are always pointer values in Ego.
// ─────────────────────────────────────────────────────────────────────────────

// Test_equalByteCode_StructEqual verifies that a struct value compares as equal
// to itself.
//
// Implementation note: equalByteCode uses reflect.DeepEqual for *data.Struct
// comparison.  reflect.DeepEqual compares ALL exported and unexported fields of
// the pointed-to struct, including the typeDef *Type pointer and the fieldOrder
// []string slice.  Two independently-created *data.Struct objects that represent
// the same Ego type will NOT reliably compare as DeepEqual because:
//   - typeDef is a distinct pointer per NewStructFromMap call
//   - fieldOrder is populated by iterating a Go map, whose order is
//     non-deterministic; two calls with the same map literal can produce
//     different orderings
//
// Using the same *data.Struct pointer for both operands is the only way to
// produce a reliable "equal" result through reflect.DeepEqual.  This test
// therefore verifies the fundamental code path (reflect.DeepEqual is called and
// its true result is propagated) rather than user-visible field equality.
func Test_equalByteCode_StructEqual(t *testing.T) {
	s := data.NewStructFromMap(map[string]any{"score": 100})

	// Push the same pointer twice: v1 = s, v2 = s.
	// reflect.DeepEqual(s, s) is always true because both sides are the same object.
	tc := newTestContext(t).withStack(s, s)

	tc.assertNoError(equalByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// Test_equalByteCode_StructNotEqual verifies that two struct values that differ
// in at least one field compare as not equal.
func Test_equalByteCode_StructNotEqual(t *testing.T) {
	s1 := data.NewStructFromMap(map[string]any{"score": 100})
	s2 := data.NewStructFromMap(map[string]any{"score": 200})

	tc := newTestContext(t).withStack(s1, s2)

	tc.assertNoError(equalByteCode(tc.ctx, nil))
	tc.assertTopStack(false)
}

// Test_equalByteCode_StructVsNonStruct verifies that a *data.Struct compared
// against a non-struct value returns false without an error.
//
// The *data.Struct case type-asserts v2 to *data.Struct; when that fails it
// pushes false rather than returning an error.
func Test_equalByteCode_StructVsNonStruct(t *testing.T) {
	s := data.NewStructFromMap(map[string]any{"x": 1})

	tc := newTestContext(t).withStack(s, 42)

	tc.assertNoError(equalByteCode(tc.ctx, nil))
	tc.assertTopStack(false)
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 6 — equalByteCode: *errors.Error equality
//
// Ego's *errors.Error type has an Equal method that compares by the underlying
// i18n message key, not the full formatted string with context.
// equalByteCode has a `case *errors.Error:` that delegates to this method.
// ─────────────────────────────────────────────────────────────────────────────

// Test_equalByteCode_EgoErrorEqual verifies that two *errors.Error objects
// built from the same message key compare as equal.
func Test_equalByteCode_EgoErrorEqual(t *testing.T) {
	e1 := errors.New(errors.ErrTypeMismatch)
	e2 := errors.New(errors.ErrTypeMismatch)

	tc := newTestContext(t).withStack(e1, e2)

	tc.assertNoError(equalByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// Test_equalByteCode_EgoErrorNotEqual verifies that two *errors.Error objects
// with different message keys compare as not equal.
func Test_equalByteCode_EgoErrorNotEqual(t *testing.T) {
	e1 := errors.New(errors.ErrTypeMismatch)
	e2 := errors.New(errors.ErrStackUnderflow)

	tc := newTestContext(t).withStack(e1, e2)

	tc.assertNoError(equalByteCode(tc.ctx, nil))
	tc.assertTopStack(false)
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 7 — equalByteCode: time.Duration and time.Time type-mismatch errors
//
// These cases in equalByteCode require v2 to be the same time type as v1.
// When v2 is a different type they return ErrInvalidTypeForOperation.
// ─────────────────────────────────────────────────────────────────────────────

// Test_equalByteCode_Duration_TypeMismatch verifies that comparing a
// time.Duration with a non-Duration value returns ErrInvalidTypeForOperation.
func Test_equalByteCode_Duration_TypeMismatch(t *testing.T) {
	// v1 = time.Duration, v2 = plain integer — wrong type.
	tc := newTestContext(t).withStack(5*time.Second, 42)

	tc.assertError(equalByteCode(tc.ctx, nil), errors.ErrInvalidTypeForOperation)
}

// Test_equalByteCode_TimeTime_TypeMismatch verifies that comparing a time.Time
// with a non-Time value returns ErrInvalidTypeForOperation.
func Test_equalByteCode_TimeTime_TypeMismatch(t *testing.T) {
	tc := newTestContext(t).withStack(time.Now(), "not a time")

	tc.assertError(equalByteCode(tc.ctx, nil), errors.ErrInvalidTypeForOperation)
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 8 — genericEqualCompare: byte (uint8) type
//
// genericEqualCompare routes integer comparisons through data.Int64, which
// handles the full set of Go integer types including byte (= uint8).
// ─────────────────────────────────────────────────────────────────────────────

// Test_equalByteCode_ByteEqual verifies that two equal byte values compare as
// equal.  byte is Go's alias for uint8.
func Test_equalByteCode_ByteEqual(t *testing.T) {
	tc := newTestContext(t).withStack(byte(65), byte(65))

	tc.assertNoError(equalByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// Test_equalByteCode_ByteNotEqual verifies that two different byte values are
// not equal.
func Test_equalByteCode_ByteNotEqual(t *testing.T) {
	tc := newTestContext(t).withStack(byte(65), byte(66))

	tc.assertNoError(equalByteCode(tc.ctx, nil))
	tc.assertTopStack(false)
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 9 — getComparisonTerms: Immutable constant coercion (EQUAL-2 path)
//
// When at least one operand is a compile-time constant and both operands are
// numeric, getComparisonTerms promotes the lower-rank type to match the
// higher-rank type before returning.
//
// EQUAL-2 fix: after promotion, any coerce error is now passed through
// c.runtimeError so it carries module/line info.  data.Coerce never fails for
// two valid numeric values, so the error wrap is defensive; we verify the
// success path below.
// ─────────────────────────────────────────────────────────────────────────────

// Test_equalByteCode_ImmutableCoercion_PromotesConstantToFloat64 verifies that
// when v1 is float64 and v2 is an Immutable-wrapped int constant, the int is
// promoted to float64 before the comparison runs.
//
// Without coercion, float64(3) and int(3) would have different types and the
// comparison would fail in strict mode.  This exercises the EQUAL-2 code path.
func Test_equalByteCode_ImmutableCoercion_PromotesConstantToFloat64(t *testing.T) {
	// v1 = float64(3.0) on the stack.
	// v2 = Immutable{int(3)} in the operand — simulates a compiled literal.
	tc := newTestContext(t).withStack(float64(3.0))

	// data.KindOf(float64) > data.KindOf(int), so v2 (the constant int) is
	// coerced to float64 before comparison.
	tc.assertNoError(equalByteCode(tc.ctx, []any{data.Immutable{Value: 3}}))

	// After coercion, both sides are float64(3.0) — they are equal.
	tc.assertTopStack(true)
}

// Test_equalByteCode_ImmutableCoercion_PromotesConstantToInt32 verifies
// coercion in the other direction: when v1 is int32 and v2 is a constant int
// (lower-rank), the int is promoted to int32.
func Test_equalByteCode_ImmutableCoercion_PromotesConstantToInt32(t *testing.T) {
	// v1 = int32(7) on the stack.  data.KindOf(int32) > data.KindOf(int),
	// so the constant int is coerced up to int32.
	tc := newTestContext(t).withStack(int32(7))

	tc.assertNoError(equalByteCode(tc.ctx, []any{data.Immutable{Value: 7}}))
	tc.assertTopStack(true)
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 10 — Nil handling: verifying the EQUAL-3 dead-code removal
//
// The `case nil:` branch that existed in equalByteCode's type switch was dead
// code.  It could never be reached because data.IsNil(nil) returns true, so
// any nil v1 would be caught by the guards above the switch and return before
// the switch runs.
//
// After EQUAL-3 was resolved, the dead branch was removed.  These tests verify
// that the observable nil behavior is unchanged: nil == nil → true, and
// nil == non-nil → false.  Both outcomes come from the guards, not the switch.
// ─────────────────────────────────────────────────────────────────────────────

// Test_equalByteCode_NilNilHandledByGuard confirms nil == nil returns true.
// This is handled entirely by the `data.IsNil(v1) && data.IsNil(v2)` guard;
// the type switch (including the now-removed `case nil:`) is never reached.
func Test_equalByteCode_NilNilHandledByGuard(t *testing.T) {
	tc := newTestContext(t).withStack(nil, nil)

	tc.assertNoError(equalByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// Test_equalByteCode_NilOneNilHandledByGuard confirms nil == non-nil returns
// false.  Handled by the `data.IsNil(v1) || data.IsNil(v2)` guard before the
// type switch runs.
func Test_equalByteCode_NilOneNilHandledByGuard(t *testing.T) {
	// v1 = nil (bottom), v2 = 99 (top).
	tc := newTestContext(t).withStack(nil, 99)

	tc.assertNoError(equalByteCode(tc.ctx, nil))
	tc.assertTopStack(false)
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 11 — uint types in genericEqualCompare
//
// The unsigned integer path in genericEqualCompare handles uint16, uint32,
// uint, and uint64 via data.UInt64.  This section tests each unsigned width.
// ─────────────────────────────────────────────────────────────────────────────

// Test_equalByteCode_Uint16Equal verifies uint16 equality.
func Test_equalByteCode_Uint16Equal(t *testing.T) {
	tc := newTestContext(t).withStack(uint16(1000), uint16(1000))

	tc.assertNoError(equalByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// Test_equalByteCode_Uint16NotEqual verifies that different uint16 values are
// not equal.
func Test_equalByteCode_Uint16NotEqual(t *testing.T) {
	tc := newTestContext(t).withStack(uint16(1), uint16(2))

	tc.assertNoError(equalByteCode(tc.ctx, nil))
	tc.assertTopStack(false)
}

// Test_equalByteCode_Uint32Equal verifies uint32 equality.
func Test_equalByteCode_Uint32Equal(t *testing.T) {
	tc := newTestContext(t).withStack(uint32(999999), uint32(999999))

	tc.assertNoError(equalByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// Test_equalByteCode_UintEqual verifies equality for the native uint type
// (platform-dependent width, typically 64 bits on modern systems).
func Test_equalByteCode_UintEqual(t *testing.T) {
	tc := newTestContext(t).withStack(uint(42), uint(42))

	tc.assertNoError(equalByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// Test_equalByteCode_Uint64NotEqual verifies that different uint64 values are
// not equal.
func Test_equalByteCode_Uint64NotEqual(t *testing.T) {
	tc := newTestContext(t).withStack(uint64(100), uint64(200))

	tc.assertNoError(equalByteCode(tc.ctx, nil))
	tc.assertTopStack(false)
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 12 — Stack discipline
//
// Every comparison instruction must consume exactly two stack items and push
// exactly one bool result.  Extra items left on the stack corrupt subsequent
// expression evaluation.
// ─────────────────────────────────────────────────────────────────────────────

// Test_equalByteCode_StackDiscipline_ConsumesBothPushesOne verifies that
// equalByteCode removes exactly two values from the stack and pushes exactly
// one bool result.
func Test_equalByteCode_StackDiscipline_ConsumesBothPushesOne(t *testing.T) {
	tc := newTestContext(t).withStack(7, 7)

	tc.assertNoError(equalByteCode(tc.ctx, nil))

	// Pop the result and confirm it is the expected boolean.
	tc.assertTopStack(true)

	// Nothing else should remain on the stack.
	tc.assertStackEmpty()
}
