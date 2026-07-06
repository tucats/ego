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
//	EQUAL-1  equalTypes previously returned errors.ErrNotAType without going
//	         through c.runtimeError when v2 was not a type or string.  The
//	         fix added c.runtimeError decoration.  EQUAL-4 (BUG-13) later
//	         superseded this by changing the non-type path to push false
//	         instead of erroring, so the decoration question became moot.
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
//
//	EQUAL-4  (BUG-13 fix) The type-vs-string cheat in equalTypes is removed.
//	         typeof(n)=="int" now returns false; use typeof(n)==int instead.
//	         A symmetric guard in equalByteCode handles the switch-case
//	         ordering where the type is on the right (v2 = *data.Type).
//	         Verified by: Test_equalTypes_TypeVsMatchingString,
//	                      Test_equalTypes_TypeVsNonType,
//	                      Test_equalTypes_TypeOnRight_StringLeft,
//	                      Test_equalTypes_TypeOnRight_TypeLeft,
//	                      Test_equalTypes_TypeOnRight_IntLeft

import (
	"testing"
	"time"

	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/data"
)

// ─────────────────────────────────────────────────────────────────────────────
// Section 1 — equalTypes and *data.Type handling (EQUAL-4 / BUG-13 fix)
//
// equalTypes is an internal helper called when the left-hand operand (v1) is
// a *data.Type value.  It compares v1 against another *data.Type only; all
// other v2 types now push false (BUG-13 / EQUAL-4 fix).
//
// Code paths:
//   type on left  → equalByteCode → case *data.Type → equalTypes(v2, c, actual)
//   type on right → equalByteCode → EQUAL-4 guard → equalTypes(v1, c, t2)
//                   (covers switch-case ordering where the compiler pushes the
//                    case value before loading the switch expression)
//
// The second operand (v2) may be:
//   (a) another *data.Type  — compare by String() representation
//   (b) anything else       — push false (a type is never equal to a non-type)
//
// Before EQUAL-4, equalTypes also accepted a string v2 and compared it to
// actual.String(), so typeof(n)=="int" returned true.  That cheat is removed.
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

// Test_equalTypes_TypeVsMatchingString verifies that a *data.Type compared
// against a string that spells out its canonical name now returns FALSE.
//
// EQUAL-4 (BUG-13 fix): before this fix, typeof(x) == "int" returned true via
// a legacy cheat in equalTypes.  Now that Ego has a first-class type system,
// a type value is only equal to another type value — never to a string.
func Test_equalTypes_TypeVsMatchingString(t *testing.T) {
	// v1 = *data.Type for int, v2 = the string "int".
	tc := newTestContext(t).withStack(data.IntType, "int")

	tc.assertNoError(equalByteCode(tc.ctx, nil))

	// Must return false, not true.  Use the type constant
	// directly: typeof(n) == int.
	tc.assertTopStack(false)
}

// Test_equalTypes_TypeVsNonMatchingString verifies that a *data.Type compared
// against a string naming a different type returns false.
//
// This was already false before BUG-13 was fixed; the path changes (previously
// it went through the string branch in equalTypes, now it hits the non-type
// fallback) but the result is the same.
func Test_equalTypes_TypeVsNonMatchingString(t *testing.T) {
	// data.IntType.String() = "int", which differs from "float64".
	tc := newTestContext(t).withStack(data.IntType, "float64")

	tc.assertNoError(equalByteCode(tc.ctx, nil))
	tc.assertTopStack(false)
}

// Test_equalTypes_TypeVsNonType verifies that comparing a *data.Type against a
// non-type, non-string value returns FALSE (not an error).
//
// EQUAL-4 (BUG-13 fix): before this fix, the comparison returned ErrNotAType.
// After the fix, a type is simply never equal to a non-type — the comparison
// returns false rather than raising a runtime error, consistent with Go's
// semantics for == between incomparable types.
func Test_equalTypes_TypeVsNonType(t *testing.T) {
	// v1 = *data.Type, v2 = 42 — not a type and not a string.
	tc := newTestContext(t).withStack(data.IntType, 42)

	// Must now return false without error.
	tc.assertNoError(equalByteCode(tc.ctx, nil))
	tc.assertTopStack(false)
}

// Test_equalTypes_TypeOnRight_StringLeft verifies the EQUAL-4 guard for the
// reverse operand order produced by switch-case compilation.
//
// When the compiler emits code for `switch typeof(n) { case "int": ... }`, it
// pushes the case value ("int") first, then loads the switch expression (typeof(n)).
// getComparisonTerms thus produces v1="int" (string) and v2=*data.Type.
// Before EQUAL-4 this path fell through to genericEqualCompare → Normalize →
// Coerce("int", intTypeInstance) → coerceToInt("int") → error
// "invalid integer value: int".  After the fix, the early guard in
// equalByteCode pushes false without reaching genericEqualCompare.
func Test_equalTypes_TypeOnRight_StringLeft(t *testing.T) {
	// Stack: v1="int" (bottom), v2=data.IntType (top).
	// This is the switch-case direction.
	tc := newTestContext(t).withStack("int", data.IntType)

	tc.assertNoError(equalByteCode(tc.ctx, nil))
	// A string is never equal to a type.
	tc.assertTopStack(false)
}

// Test_equalTypes_TypeOnRight_TypeLeft verifies that type == type still works
// correctly when the type is the right-hand operand (EQUAL-4 path).
func Test_equalTypes_TypeOnRight_TypeLeft(t *testing.T) {
	// Both int types, but placed so data.IntType is on the right (top of stack).
	tc := newTestContext(t).withStack(data.IntType, data.IntType)

	tc.assertNoError(equalByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// Test_equalTypes_TypeOnRight_IntLeft verifies the EQUAL-4 guard for an integer
// left-hand operand, confirming no error from genericEqualCompare's Normalize path.
func Test_equalTypes_TypeOnRight_IntLeft(t *testing.T) {
	// v1 = 42 (int), v2 = *data.Type (top).  Before EQUAL-4 this would also
	// trigger the Coerce path, producing an "invalid integer value" error.
	tc := newTestContext(t).withStack(42, data.IntType)

	tc.assertNoError(equalByteCode(tc.ctx, nil))
	tc.assertTopStack(false)
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

// ─────────────────────────────────────────────────────────────────────────────
// Section 13 — BUG-34: scalar/native pointer identity comparison
//
// Before this fix, neither equalByteCode nor notEqualByteCode had a case for
// a plain Go pointer value (e.g. *int, *string, or the *any that Ego's real
// "&name" address-of operator actually produces -- see
// addressOfByteCode/symbols.SymbolTable.GetAddress). Such a value fell
// through to genericEqualCompare, whose inner switch also has no pointer
// case, leaving `result` at its zero value (false) unconditionally. That
// made "pa == pb" AND "pa != pb" both false for the same pair of pointers --
// even "pa == pa" was false -- violating the basic invariant that exactly one
// of == and != must be true.
//
// isPointerValue + the new default-case branches in equalByteCode and
// notEqualByteCode fix this by comparing pointer identity (the Go pointer
// value itself, comparable via plain "==") whenever both operands are
// pointers, and treating a pointer as unequal to any non-pointer value.
// ─────────────────────────────────────────────────────────────────────────────

// Test_equalByteCode_SamePointerIsEqual verifies that a pointer compares
// equal to itself.
func Test_equalByteCode_SamePointerIsEqual(t *testing.T) {
	n := 5
	p := &n

	tc := newTestContext(t).withStack(p, p)

	tc.assertNoError(equalByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// Test_equalByteCode_DifferentPointersSameValueAreNotEqual is the
// core BUG-34 regression test: two distinct *int pointers whose
// pointed-to values happen to be equal (5 == 5) must NOT compare
// as == -- pointer comparison is by address/identity, not by the
// pointed-to value. Before the fix this returned false anyway,
// but only by accident (result defaulted to false for every pointer
// pair); Test_notEqualByteCode_DifferentPointersSameValueAreNotEqual
// below confirms != is now the correct, consistent negation of this
// result.
func Test_equalByteCode_DifferentPointersSameValueAreNotEqual(t *testing.T) {
	a := 5
	b := 5
	pa := &a
	pb := &b

	tc := newTestContext(t).withStack(pa, pb)

	tc.assertNoError(equalByteCode(tc.ctx, nil))
	tc.assertTopStack(false)
}

// Test_equalByteCode_DifferentConcretePointerTypesAreNotEqual verifies that
// comparing pointers of two different concrete Go types (here *int vs
// *string) is well-defined as false and does not panic, even though both
// operands are pointer-kind values.
func Test_equalByteCode_DifferentConcretePointerTypesAreNotEqual(t *testing.T) {
	n := 5
	s := "5"
	pn := &n
	ps := &s

	tc := newTestContext(t).withStack(pn, ps)

	tc.assertNoError(equalByteCode(tc.ctx, nil))
	tc.assertTopStack(false)
}

// Test_equalByteCode_PointerVsNonPointerIsNotEqual verifies that a pointer is
// never equal to a non-pointer value (e.g. "pa == 5").
func Test_equalByteCode_PointerVsNonPointerIsNotEqual(t *testing.T) {
	n := 5
	pn := &n

	tc := newTestContext(t).withStack(pn, 5)

	tc.assertNoError(equalByteCode(tc.ctx, nil))
	tc.assertTopStack(false)
}

// Test_equalByteCode_AnyPointerIdentity exercises the *any pointer
// representation that Ego's actual "&name" operator produces at runtime
// (see symbols.SymbolTable.GetAddress), as opposed to the *int used by the
// other tests in this section (which stand in for what data.AddressOf would
// produce, a code path that is not currently reachable from "&name" but is
// still handled defensively). Two different *any pointers must compare as
// not equal even when the values they point to are equal.
func Test_equalByteCode_AnyPointerIdentity(t *testing.T) {
	var (
		v1 any = 5
		v2 any = 5
	)

	p1 := &v1
	p2 := &v2

	tc := newTestContext(t).withStack(p1, p1)
	tc.assertNoError(equalByteCode(tc.ctx, nil))
	tc.assertTopStack(true)

	tc2 := newTestContext(t).withStack(p1, p2)
	tc2.assertNoError(equalByteCode(tc2.ctx, nil))
	tc2.assertTopStack(false)
}

// Test_notEqualByteCode_SamePointerIsNotUnequal verifies that a pointer does
// not compare != to itself (the exact negation of
// Test_equalByteCode_SamePointerIsEqual).
func Test_notEqualByteCode_SamePointerIsNotUnequal(t *testing.T) {
	n := 5
	p := &n

	tc := newTestContext(t).withStack(p, p)

	tc.assertNoError(notEqualByteCode(tc.ctx, nil))
	tc.assertTopStack(false)
}

// Test_notEqualByteCode_DifferentPointersSameValueAreNotEqual is the BUG-34
// regression test for !=: two distinct pointers to variables holding equal
// values must compare as != true. Before the fix this was false, making !=
// completely unusable for pointer comparisons (it could never fire).
func Test_notEqualByteCode_DifferentPointersSameValueAreNotEqual(t *testing.T) {
	a := 5
	b := 5
	pa := &a
	pb := &b

	tc := newTestContext(t).withStack(pa, pb)

	tc.assertNoError(notEqualByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// Test_notEqualByteCode_PointerVsNonPointerIsUnequal verifies that a pointer
// is always != a non-pointer value.
func Test_notEqualByteCode_PointerVsNonPointerIsUnequal(t *testing.T) {
	n := 5
	pn := &n

	tc := newTestContext(t).withStack(pn, 5)

	tc.assertNoError(notEqualByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// Test_equalNotEqual_PointerResultsAreConsistentNegations is the
// core BUG-34 regression test: for a representative set of 
// pointer/value pairs, == and != must always disagree (exactly
// one is true), never both false (the original bug) and never 
// both true.
func Test_equalNotEqual_PointerResultsAreConsistentNegations(t *testing.T) {
	a := 5
	b := 5
	pa := &a
	pb := &b

	cases := []struct {
		name string
		v1   any
		v2   any
	}{
		{"same pointer", pa, pa},
		{"different pointers, equal values", pa, pb},
		{"pointer vs scalar", pa, 5},
		{"pointer vs nil", pa, nil},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			eqTc := newTestContext(t).withStack(tt.v1, tt.v2)
			eqTc.assertNoError(equalByteCode(eqTc.ctx, nil))

			eqResult, err := eqTc.ctx.Pop()
			if err != nil {
				t.Fatalf("Pop (==) failed: %v", err)
			}

			neTc := newTestContext(t).withStack(tt.v1, tt.v2)
			neTc.assertNoError(notEqualByteCode(neTc.ctx, nil))

			neResult, err := neTc.ctx.Pop()
			if err != nil {
				t.Fatalf("Pop (!=) failed: %v", err)
			}

			eqBool, _ := eqResult.(bool)
			neBool, _ := neResult.(bool)

			if eqBool == neBool {
				t.Errorf("%s: == returned %v and != returned %v; exactly one must be true (BUG-34)",
					tt.name, eqBool, neBool)
			}
		})
	}
}
