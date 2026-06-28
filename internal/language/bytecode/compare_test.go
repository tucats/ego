package bytecode

// compare_test.go contains comprehensive unit tests for the six comparison
// bytecode handlers:
//
//   equalByteCode             (==)  equal.go
//   notEqualByteCode          (!=)  notEqual.go
//   lessThanByteCode          (<)   lessThan.go
//   greaterThanByteCode       (>)   greaterThan.go
//   lessThanOrEqualByteCode    (<=)
//   greaterThanOrEqualByteCode (>=)
//
// # How comparison works
//
// Every comparison instruction pops two values from the execution stack,
// compares them, and pushes a single boolean result.
//
// Stack layout before the call (set up by withStack):
//
//	withStack(v1, v2)  → v1 is at the bottom, v2 on top
//
// getComparisonTerms (shared helper) pops v2 first, then v1, so the
// comparison is effectively "is v1 OP v2?".
//
// Type handling follows these rules:
//   - In NoTypeEnforcement (default): Normalize promotes both values to the
//     same kind (e.g. int + float64 → both float64) before comparing.
//   - In StrictTypeEnforcement: the types must already match; mismatches
//     return ErrTypeMismatch.
//   - Composite types (Map, Array, Struct) use DeepEqual for == and !=; they
//     are not orderable (< > <= >= return ErrInvalidType).
//
// # Test infrastructure
//
// All tests use newTestContext and withStack (see testhelpers_test.go) rather
// than the raw Context{} struct literals used in the original file.  This
// ensures proper initialization and gives the same helpers (assertTopStack,
// assertError, etc.) that every other test file uses.
//
// # Bugs documented here
//
// COMPARE-2  notEqualByteCode used value types (data.Map / data.Array /
//            data.Struct) instead of pointer types in its switch cases.
//            Fixed — pointer types now used, matching equalByteCode.
//
// COMPARE-3  lessThanByteCode, greaterThanByteCode, lessThanOrEqualByteCode,
//            and greaterThanOrEqualByteCode were missing int8 in the
//            signed-integer inner-switch case.  Fixed — int8 now included.

import (
	"testing"
	"time"

	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
)

// ─────────────────────────────────────────────────────────────────────────────
// Section 1: equalByteCode — equality tests across all supported types
// ─────────────────────────────────────────────────────────────────────────────

// Test_equalByteCode_BoolEqual verifies that two identical bool values compare
// as equal.
func Test_equalByteCode_BoolEqual(t *testing.T) {
	tc := newTestContext(t).withStack(true, true)

	tc.assertNoError(equalByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// Test_equalByteCode_BoolNotEqual verifies that true != false.
func Test_equalByteCode_BoolNotEqual(t *testing.T) {
	tc := newTestContext(t).withStack(true, false)

	tc.assertNoError(equalByteCode(tc.ctx, nil))
	tc.assertTopStack(false)
}

// Test_equalByteCode_IntEqual verifies int equality.
func Test_equalByteCode_IntEqual(t *testing.T) {
	tc := newTestContext(t).withStack(42, 42)

	tc.assertNoError(equalByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// Test_equalByteCode_IntNotEqual verifies that different ints are not equal.
func Test_equalByteCode_IntNotEqual(t *testing.T) {
	tc := newTestContext(t).withStack(42, 45)

	tc.assertNoError(equalByteCode(tc.ctx, nil))
	tc.assertTopStack(false)
}

// Test_equalByteCode_Int8Equal verifies int8 equality.
func Test_equalByteCode_Int8Equal(t *testing.T) {
	tc := newTestContext(t).withStack(int8(10), int8(10))

	tc.assertNoError(equalByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// Test_equalByteCode_Int16Equal verifies int16 equality.
func Test_equalByteCode_Int16Equal(t *testing.T) {
	tc := newTestContext(t).withStack(int16(300), int16(300))

	tc.assertNoError(equalByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// Test_equalByteCode_Int32Equal verifies int32 equality.
func Test_equalByteCode_Int32Equal(t *testing.T) {
	tc := newTestContext(t).withStack(int32(1000), int32(1000))

	tc.assertNoError(equalByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// Test_equalByteCode_Int64Equal verifies int64 equality.
func Test_equalByteCode_Int64Equal(t *testing.T) {
	tc := newTestContext(t).withStack(int64(99), int64(99))

	tc.assertNoError(equalByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// Test_equalByteCode_Uint64Equal verifies uint64 equality.
func Test_equalByteCode_Uint64Equal(t *testing.T) {
	tc := newTestContext(t).withStack(uint64(5), uint64(5))

	tc.assertNoError(equalByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// Test_equalByteCode_Float64Equal verifies float64 equality.
func Test_equalByteCode_Float64Equal(t *testing.T) {
	tc := newTestContext(t).withStack(3.14, 3.14)

	tc.assertNoError(equalByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// Test_equalByteCode_Float64NotEqual verifies that slightly different float64
// values are not equal.
func Test_equalByteCode_Float64NotEqual(t *testing.T) {
	tc := newTestContext(t).withStack(3.14, 3.14001)

	tc.assertNoError(equalByteCode(tc.ctx, nil))
	tc.assertTopStack(false)
}

// Test_equalByteCode_Float32Equal verifies float32 equality.
func Test_equalByteCode_Float32Equal(t *testing.T) {
	tc := newTestContext(t).withStack(float32(1.5), float32(1.5))

	tc.assertNoError(equalByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// Test_equalByteCode_StringEqual verifies string equality (case-sensitive).
func Test_equalByteCode_StringEqual(t *testing.T) {
	tc := newTestContext(t).withStack("hello", "hello")

	tc.assertNoError(equalByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// Test_equalByteCode_StringNotEqual verifies that case matters in string
// comparison.
func Test_equalByteCode_StringNotEqual(t *testing.T) {
	tc := newTestContext(t).withStack("tom", "Tom")

	tc.assertNoError(equalByteCode(tc.ctx, nil))
	tc.assertTopStack(false)
}

// Test_equalByteCode_NilBothNil verifies that nil == nil.
func Test_equalByteCode_NilBothNil(t *testing.T) {
	tc := newTestContext(t).withStack(nil, nil)

	tc.assertNoError(equalByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// Test_equalByteCode_NilOneNil verifies that nil != non-nil.
func Test_equalByteCode_NilOneNil(t *testing.T) {
	tc := newTestContext(t).withStack(nil, 42)

	tc.assertNoError(equalByteCode(tc.ctx, nil))
	tc.assertTopStack(false)
}

// Test_equalByteCode_NumericPromotion verifies that int == float64 is handled
// by type promotion (normalization) in dynamic mode.
func Test_equalByteCode_NumericPromotion(t *testing.T) {
	tc := newTestContext(t).withStack(42, float64(42))

	tc.assertNoError(equalByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// Test_equalByteCode_StringPromotion verifies that int == "42" is handled by
// normalization in dynamic mode (string is parsed as a number).
func Test_equalByteCode_StringPromotion(t *testing.T) {
	tc := newTestContext(t).withStack(42, "42")

	tc.assertNoError(equalByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// Test_equalByteCode_ArrayEqual verifies that two arrays with identical
// contents and type compare as equal.
func Test_equalByteCode_ArrayEqual(t *testing.T) {
	arr1 := data.NewArrayFromList(data.IntType, data.NewList(5, 2, 6))
	arr2 := data.NewArrayFromList(data.IntType, data.NewList(5, 2, 6))
	tc := newTestContext(t).withStack(arr1, arr2)

	tc.assertNoError(equalByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// Test_equalByteCode_ArrayInequalityType verifies that two arrays with the
// same values but different element types are not equal.
func Test_equalByteCode_ArrayInequalityType(t *testing.T) {
	arr1 := data.NewArrayFromList(data.IntType, data.NewList(1, 2, 3))
	arr2 := data.NewArrayFromList(data.Float64Type, data.NewList(1, 2, 3))
	tc := newTestContext(t).withStack(arr1, arr2)

	tc.assertNoError(equalByteCode(tc.ctx, nil))
	tc.assertTopStack(false)
}

// Test_equalByteCode_ArrayInequalityValues verifies that arrays with the same
// type but different values are not equal.
func Test_equalByteCode_ArrayInequalityValues(t *testing.T) {
	arr1 := data.NewArrayFromList(data.IntType, data.NewList(1, 2, 3))
	arr2 := data.NewArrayFromList(data.IntType, data.NewList(1, 3, 2))
	tc := newTestContext(t).withStack(arr1, arr2)

	tc.assertNoError(equalByteCode(tc.ctx, nil))
	tc.assertTopStack(false)
}

// Test_equalByteCode_TimeDurationEqual verifies time.Duration equality.
func Test_equalByteCode_TimeDurationEqual(t *testing.T) {
	d := 5 * time.Second
	tc := newTestContext(t).withStack(d, d)

	tc.assertNoError(equalByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// Test_equalByteCode_TimeDurationNotEqual verifies that different durations
// are not equal.
func Test_equalByteCode_TimeDurationNotEqual(t *testing.T) {
	tc := newTestContext(t).withStack(time.Second, time.Minute)

	tc.assertNoError(equalByteCode(tc.ctx, nil))
	tc.assertTopStack(false)
}

// Test_equalByteCode_TimeTimeEqual verifies time.Time equality.
func Test_equalByteCode_TimeTimeEqual(t *testing.T) {
	ts := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	tc := newTestContext(t).withStack(ts, ts)

	tc.assertNoError(equalByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// Test_equalByteCode_StrictMode_MatchingType verifies that identical types
// pass the equality check in strict mode.
func Test_equalByteCode_StrictMode_MatchingType(t *testing.T) {
	tc := newTestContext(t).
		withTypeStrictness(defs.StrictTypeEnforcement).
		withStack(42, 42)

	tc.assertNoError(equalByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// Test_equalByteCode_StrictMode_MismatchedType verifies that comparing an int
// with a float64 in strict mode returns ErrTypeMismatch.
func Test_equalByteCode_StrictMode_MismatchedType(t *testing.T) {
	tc := newTestContext(t).
		withTypeStrictness(defs.StrictTypeEnforcement).
		withStack(42, float64(42))

	tc.assertError(equalByteCode(tc.ctx, nil), errors.ErrTypeMismatch)
}

// Test_equalByteCode_StackUnderflow verifies ErrStackUnderflow when the stack
// has fewer than two values.
func Test_equalByteCode_StackUnderflow(t *testing.T) {
	tc := newTestContext(t).withStack(42) // only one item

	tc.assertError(equalByteCode(tc.ctx, nil), errors.ErrStackUnderflow)
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 2: notEqualByteCode — inequality tests
// ─────────────────────────────────────────────────────────────────────────────

// Test_notEqualByteCode_IntNotEqual verifies that two different ints return
// true from !=.
func Test_notEqualByteCode_IntNotEqual(t *testing.T) {
	tc := newTestContext(t).withStack(1, 2)

	tc.assertNoError(notEqualByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// Test_notEqualByteCode_IntEqual verifies that equal ints return false from !=.
func Test_notEqualByteCode_IntEqual(t *testing.T) {
	tc := newTestContext(t).withStack(5, 5)

	tc.assertNoError(notEqualByteCode(tc.ctx, nil))
	tc.assertTopStack(false)
}

// Test_notEqualByteCode_Float64NotEqual verifies float64 inequality.
func Test_notEqualByteCode_Float64NotEqual(t *testing.T) {
	tc := newTestContext(t).withStack(1.1, 1.2)

	tc.assertNoError(notEqualByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// Test_notEqualByteCode_StringNotEqual verifies string inequality.
func Test_notEqualByteCode_StringNotEqual(t *testing.T) {
	tc := newTestContext(t).withStack("a", "b")

	tc.assertNoError(notEqualByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// Test_notEqualByteCode_StringEqual verifies that identical strings return
// false from !=.
func Test_notEqualByteCode_StringEqual(t *testing.T) {
	tc := newTestContext(t).withStack("same", "same")

	tc.assertNoError(notEqualByteCode(tc.ctx, nil))
	tc.assertTopStack(false)
}

// Test_notEqualByteCode_BoolNotEqual verifies bool inequality.
func Test_notEqualByteCode_BoolNotEqual(t *testing.T) {
	tc := newTestContext(t).withStack(true, false)

	tc.assertNoError(notEqualByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// Test_notEqualByteCode_NilBothNil verifies that nil != nil is false (both nil
// means they ARE equal, so not-equal is false).
func Test_notEqualByteCode_NilBothNil(t *testing.T) {
	tc := newTestContext(t).withStack(nil, nil)

	tc.assertNoError(notEqualByteCode(tc.ctx, nil))
	tc.assertTopStack(false)
}

// Test_notEqualByteCode_NilOneNil verifies that nil != non-nil is true.
func Test_notEqualByteCode_NilOneNil(t *testing.T) {
	tc := newTestContext(t).withStack(nil, 42)

	tc.assertNoError(notEqualByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// Test_notEqualByteCode_ArrayNotEqualValues verifies that two arrays with
// different contents return true from !=.
// The original code used value type (data.Array) which never matched; fixed
// by COMPARE-2 to use the pointer type (*data.Array).
func Test_notEqualByteCode_ArrayNotEqualValues(t *testing.T) {
	arr1 := data.NewArrayFromInterfaces(data.IntType, 1, 2, 3)
	arr2 := data.NewArrayFromInterfaces(data.IntType, 1, 2, 4) // last element differs

	tc := newTestContext(t).withStack(arr1, arr2)

	tc.assertNoError(notEqualByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// Test_notEqualByteCode_ArrayEqualValues verifies that identical arrays return
// false from !=.
func Test_notEqualByteCode_ArrayEqualValues(t *testing.T) {
	arr1 := data.NewArrayFromInterfaces(data.IntType, 1, 2, 3)
	arr2 := data.NewArrayFromInterfaces(data.IntType, 1, 2, 3)
	tc := newTestContext(t).withStack(arr1, arr2)

	tc.assertNoError(notEqualByteCode(tc.ctx, nil))
	tc.assertTopStack(false)
}

// Test_notEqualByteCode_MapNotEqual verifies that two maps with different
// values return true from !=.
// The original code used value type (data.Map) which never matched; fixed
// by COMPARE-2 to use the pointer type (*data.Map).
func Test_notEqualByteCode_MapNotEqual(t *testing.T) {
	mapA := data.NewMap(data.StringType, data.IntType)
	mapB := data.NewMap(data.StringType, data.IntType)
	
	mapA.SetAlways("key", 1)
	mapB.SetAlways("key", 2) // different value

	tc := newTestContext(t).withStack(mapA, mapB)

	tc.assertNoError(notEqualByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 3: lessThanByteCode
// ─────────────────────────────────────────────────────────────────────────────

// Test_lessThanByteCode_IntLess verifies that a smaller int is less than a
// larger one.
func Test_lessThanByteCode_IntLess(t *testing.T) {
	tc := newTestContext(t).withStack(3, 7)

	tc.assertNoError(lessThanByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// Test_lessThanByteCode_IntNotLess verifies that a larger int is not less than
// a smaller one.
func Test_lessThanByteCode_IntNotLess(t *testing.T) {
	tc := newTestContext(t).withStack(10, 3)

	tc.assertNoError(lessThanByteCode(tc.ctx, nil))
	tc.assertTopStack(false)
}

// Test_lessThanByteCode_IntEqual verifies that equal ints are not less than
// each other.
func Test_lessThanByteCode_IntEqual(t *testing.T) {
	tc := newTestContext(t).withStack(5, 5)

	tc.assertNoError(lessThanByteCode(tc.ctx, nil))
	tc.assertTopStack(false)
}

// Test_lessThanByteCode_Int8 verifies that int8 values can be compared with <.
// int8 was previously missing from the signed-integer case (COMPARE-3).
func Test_lessThanByteCode_Int8(t *testing.T) {
	tc := newTestContext(t).withStack(int8(1), int8(2))

	tc.assertNoError(lessThanByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// Test_lessThanByteCode_Float64Less verifies float64 less-than.
func Test_lessThanByteCode_Float64Less(t *testing.T) {
	tc := newTestContext(t).withStack(1.0, 2.0)

	tc.assertNoError(lessThanByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// Test_lessThanByteCode_Float32Less verifies float32 less-than.
func Test_lessThanByteCode_Float32Less(t *testing.T) {
	tc := newTestContext(t).withStack(float32(1.0), float32(2.0))

	tc.assertNoError(lessThanByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// Test_lessThanByteCode_StringLess verifies lexicographic less-than for
// strings.
func Test_lessThanByteCode_StringLess(t *testing.T) {
	tc := newTestContext(t).withStack("apple", "banana")

	tc.assertNoError(lessThanByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// Test_lessThanByteCode_StringNotLess verifies that "z" is not less than "a".
func Test_lessThanByteCode_StringNotLess(t *testing.T) {
	tc := newTestContext(t).withStack("zebra", "apple")

	tc.assertNoError(lessThanByteCode(tc.ctx, nil))
	tc.assertTopStack(false)
}

// Test_lessThanByteCode_BoolInvalid verifies that bool values cannot be
// ordered; an error is expected.
func Test_lessThanByteCode_BoolInvalid(t *testing.T) {
	tc := newTestContext(t).withStack(true, false)

	if err := lessThanByteCode(tc.ctx, nil); err == nil {
		t.Error("expected error for bool < bool, got nil")
	}
}

// Test_lessThanByteCode_MapInvalid verifies that map values cannot be ordered.
func Test_lessThanByteCode_MapInvalid(t *testing.T) {
	m := data.NewMap(data.StringType, data.IntType)
	tc := newTestContext(t).withStack(m, m)

	if err := lessThanByteCode(tc.ctx, nil); err == nil {
		t.Error("expected error for map < map, got nil")
	}
}

// Test_lessThanByteCode_NilReturnsFalse verifies that nil < nil returns false
// (nil comparisons are defined to be false for ordering).
func Test_lessThanByteCode_NilReturnsFalse(t *testing.T) {
	tc := newTestContext(t).withStack(nil, nil)

	tc.assertNoError(lessThanByteCode(tc.ctx, nil))
	tc.assertTopStack(false)
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 4: greaterThanByteCode
// ─────────────────────────────────────────────────────────────────────────────

// Test_greaterThanByteCode_IntGreater verifies that a larger int is greater.
func Test_greaterThanByteCode_IntGreater(t *testing.T) {
	tc := newTestContext(t).withStack(10, 3)

	tc.assertNoError(greaterThanByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// Test_greaterThanByteCode_IntNotGreater verifies that a smaller int is not
// greater.
func Test_greaterThanByteCode_IntNotGreater(t *testing.T) {
	tc := newTestContext(t).withStack(3, 10)

	tc.assertNoError(greaterThanByteCode(tc.ctx, nil))
	tc.assertTopStack(false)
}

// Test_greaterThanByteCode_IntEqual verifies equal ints are not greater.
func Test_greaterThanByteCode_IntEqual(t *testing.T) {
	tc := newTestContext(t).withStack(5, 5)

	tc.assertNoError(greaterThanByteCode(tc.ctx, nil))
	tc.assertTopStack(false)
}

// Test_greaterThanByteCode_Int8 verifies that int8 values can be compared with >.
// int8 was previously missing from the signed-integer case (COMPARE-3).
func Test_greaterThanByteCode_Int8(t *testing.T) {
	tc := newTestContext(t).withStack(int8(5), int8(2))

	tc.assertNoError(greaterThanByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// Test_greaterThanByteCode_Float64Greater verifies float64 greater-than.
func Test_greaterThanByteCode_Float64Greater(t *testing.T) {
	tc := newTestContext(t).withStack(9.9, 1.1)

	tc.assertNoError(greaterThanByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// Test_greaterThanByteCode_StringGreater verifies string lexicographic
// greater-than.
func Test_greaterThanByteCode_StringGreater(t *testing.T) {
	tc := newTestContext(t).withStack("z", "a")

	tc.assertNoError(greaterThanByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// Test_greaterThanByteCode_BoolInvalid verifies that bool values cannot be
// ordered with >.
func Test_greaterThanByteCode_BoolInvalid(t *testing.T) {
	tc := newTestContext(t).withStack(true, false)

	if err := greaterThanByteCode(tc.ctx, nil); err == nil {
		t.Error("expected error for bool > bool, got nil")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 5: lessThanOrEqualByteCode
// ─────────────────────────────────────────────────────────────────────────────

// Test_lessThanOrEqualByteCode_IntLess verifies v1 < v2 returns true for <=.
func Test_lessThanOrEqualByteCode_IntLess(t *testing.T) {
	tc := newTestContext(t).withStack(3, 7)

	tc.assertNoError(lessThanOrEqualByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// Test_lessThanOrEqualByteCode_IntEqual verifies equal ints return true for <=.
func Test_lessThanOrEqualByteCode_IntEqual(t *testing.T) {
	tc := newTestContext(t).withStack(5, 5)

	tc.assertNoError(lessThanOrEqualByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// Test_lessThanOrEqualByteCode_IntGreater verifies v1 > v2 returns false
// for <=.
func Test_lessThanOrEqualByteCode_IntGreater(t *testing.T) {
	tc := newTestContext(t).withStack(10, 3)

	tc.assertNoError(lessThanOrEqualByteCode(tc.ctx, nil))
	tc.assertTopStack(false)
}

// Test_lessThanOrEqualByteCode_Int8 verifies that int8 values can be compared
// with <=.  int8 was previously missing from the signed-integer case (COMPARE-3).
func Test_lessThanOrEqualByteCode_Int8(t *testing.T) {
	tc := newTestContext(t).withStack(int8(3), int8(5))

	tc.assertNoError(lessThanOrEqualByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// Test_lessThanOrEqualByteCode_Float64LessEqual verifies float64 <=.
func Test_lessThanOrEqualByteCode_Float64LessEqual(t *testing.T) {
	tc := newTestContext(t).withStack(2.0, 2.0)

	tc.assertNoError(lessThanOrEqualByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// Test_lessThanOrEqualByteCode_StringLessEqual verifies string lexicographic <=.
func Test_lessThanOrEqualByteCode_StringLessEqual(t *testing.T) {
	tc := newTestContext(t).withStack("abc", "abd")

	tc.assertNoError(lessThanOrEqualByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// Test_lessThanOrEqualByteCode_BoolInvalid verifies that bools cannot be
// compared with <=.
func Test_lessThanOrEqualByteCode_BoolInvalid(t *testing.T) {
	tc := newTestContext(t).withStack(true, false)

	if err := lessThanOrEqualByteCode(tc.ctx, nil); err == nil {
		t.Error("expected error for bool <= bool, got nil")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 6: greaterThanOrEqualByteCode  (previously untested)
// ─────────────────────────────────────────────────────────────────────────────

// Test_greaterThanOrEqualByteCode_IntGreater verifies v1 > v2 returns true
// for >=.
func Test_greaterThanOrEqualByteCode_IntGreater(t *testing.T) {
	tc := newTestContext(t).withStack(10, 3)

	tc.assertNoError(greaterThanOrEqualByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// Test_greaterThanOrEqualByteCode_IntEqual verifies that equal ints return
// true for >=.
func Test_greaterThanOrEqualByteCode_IntEqual(t *testing.T) {
	tc := newTestContext(t).withStack(5, 5)

	tc.assertNoError(greaterThanOrEqualByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// Test_greaterThanOrEqualByteCode_IntLess verifies that v1 < v2 returns false
// for >=.
func Test_greaterThanOrEqualByteCode_IntLess(t *testing.T) {
	tc := newTestContext(t).withStack(3, 10)

	tc.assertNoError(greaterThanOrEqualByteCode(tc.ctx, nil))
	tc.assertTopStack(false)
}

// Test_greaterThanOrEqualByteCode_Int8 verifies that int8 values can be compared
// with >=.  int8 was previously missing from the signed-integer case (COMPARE-3).
func Test_greaterThanOrEqualByteCode_Int8(t *testing.T) {
	tc := newTestContext(t).withStack(int8(5), int8(5))

	tc.assertNoError(greaterThanOrEqualByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// Test_greaterThanOrEqualByteCode_Float64GreaterEqual verifies float64 >=.
func Test_greaterThanOrEqualByteCode_Float64GreaterEqual(t *testing.T) {
	tc := newTestContext(t).withStack(3.0, 3.0)

	tc.assertNoError(greaterThanOrEqualByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// Test_greaterThanOrEqualByteCode_StringGreaterEqual verifies string >= where
// both strings are the same.
func Test_greaterThanOrEqualByteCode_StringGreaterEqual(t *testing.T) {
	tc := newTestContext(t).withStack("same", "same")

	tc.assertNoError(greaterThanOrEqualByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// Test_greaterThanOrEqualByteCode_BoolInvalid verifies that bools cannot be
// compared with >=.
func Test_greaterThanOrEqualByteCode_BoolInvalid(t *testing.T) {
	tc := newTestContext(t).withStack(true, false)

	if err := greaterThanOrEqualByteCode(tc.ctx, nil); err == nil {
		t.Error("expected error for bool >= bool, got nil")
	}
}

// Test_greaterThanOrEqualByteCode_MapInvalid verifies that maps cannot be
// compared with >=.
func Test_greaterThanOrEqualByteCode_MapInvalid(t *testing.T) {
	m := data.NewMap(data.StringType, data.IntType)
	tc := newTestContext(t).withStack(m, m)

	if err := greaterThanOrEqualByteCode(tc.ctx, nil); err == nil {
		t.Error("expected error for map >= map, got nil")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 7: unsigned integer comparisons across all ordering operators
// ─────────────────────────────────────────────────────────────────────────────

// Test_lessThanByteCode_Uint32Less verifies uint32 <.
func Test_lessThanByteCode_Uint32Less(t *testing.T) {
	tc := newTestContext(t).withStack(uint32(1), uint32(2))

	tc.assertNoError(lessThanByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// Test_greaterThanByteCode_Uint64Greater verifies uint64 >.
func Test_greaterThanByteCode_Uint64Greater(t *testing.T) {
	tc := newTestContext(t).withStack(uint64(100), uint64(1))

	tc.assertNoError(greaterThanByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// Test_lessThanOrEqualByteCode_Uint16Equal verifies uint16 <=.
func Test_lessThanOrEqualByteCode_Uint16Equal(t *testing.T) {
	tc := newTestContext(t).withStack(uint16(10), uint16(10))

	tc.assertNoError(lessThanOrEqualByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// Test_greaterThanOrEqualByteCode_Uint32Greater verifies uint32 >=.
func Test_greaterThanOrEqualByteCode_Uint32Greater(t *testing.T) {
	tc := newTestContext(t).withStack(uint32(5), uint32(3))

	tc.assertNoError(greaterThanOrEqualByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 8: strict type enforcement across all operators
// ─────────────────────────────────────────────────────────────────────────────

// Test_notEqualByteCode_StrictMode_Match verifies that identical types pass
// in strict mode.
func Test_notEqualByteCode_StrictMode_Match(t *testing.T) {
	tc := newTestContext(t).
		withTypeStrictness(defs.StrictTypeEnforcement).
		withStack(1, 2)

	tc.assertNoError(notEqualByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// Test_notEqualByteCode_StrictMode_TypeMismatch verifies that int vs float64
// in strict mode returns ErrTypeMismatch.
func Test_notEqualByteCode_StrictMode_TypeMismatch(t *testing.T) {
	tc := newTestContext(t).
		withTypeStrictness(defs.StrictTypeEnforcement).
		withStack(1, float64(1))

	tc.assertError(notEqualByteCode(tc.ctx, nil), errors.ErrTypeMismatch)
}

// Test_lessThanByteCode_StrictMode_Match verifies identical types pass in
// strict mode for <.
func Test_lessThanByteCode_StrictMode_Match(t *testing.T) {
	tc := newTestContext(t).
		withTypeStrictness(defs.StrictTypeEnforcement).
		withStack(3, 7)

	tc.assertNoError(lessThanByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// Test_lessThanByteCode_StrictMode_TypeMismatch verifies that int vs float64
// in strict mode returns ErrTypeMismatch for <.
func Test_lessThanByteCode_StrictMode_TypeMismatch(t *testing.T) {
	tc := newTestContext(t).
		withTypeStrictness(defs.StrictTypeEnforcement).
		withStack(3, float64(7))

	tc.assertError(lessThanByteCode(tc.ctx, nil), errors.ErrTypeMismatch)
}

// Test_greaterThanOrEqualByteCode_StrictMode_Match verifies >= in strict mode
// with matching types.
func Test_greaterThanOrEqualByteCode_StrictMode_Match(t *testing.T) {
	tc := newTestContext(t).
		withTypeStrictness(defs.StrictTypeEnforcement).
		withStack(7, 3)

	tc.assertNoError(greaterThanOrEqualByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 9: Additional notEqualByteCode tests — time, float32, int variants,
//            uint variants, bool-equal, struct, and stack underflow
//
// notEqualByteCode has a richer outer switch than the ordering operators: it
// handles time.Duration, time.Time, *errors.Error, error, and all composite
// pointer types before falling through to the generic scalar default.  These
// tests exercise every branch that was not yet reached by Section 2.
// ─────────────────────────────────────────────────────────────────────────────

// Test_notEqualByteCode_TimeDurationEqual verifies that two equal time.Duration
// values return false from != (they are equal, so "not equal" is false).
//
// time.Duration is handled by the outer switch's explicit `case time.Duration:`
// branch, not by the generic scalar path.
func Test_notEqualByteCode_TimeDurationEqual(t *testing.T) {
	d := 5 * time.Second
	tc := newTestContext(t).withStack(d, d)

	tc.assertNoError(notEqualByteCode(tc.ctx, nil))
	tc.assertTopStack(false)
}

// Test_notEqualByteCode_TimeDurationNotEqual verifies that two different
// time.Duration values return true from !=.
func Test_notEqualByteCode_TimeDurationNotEqual(t *testing.T) {
	tc := newTestContext(t).withStack(time.Second, time.Minute)

	tc.assertNoError(notEqualByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// Test_notEqualByteCode_TimeDurationVsNonDuration verifies that comparing a
// time.Duration on the left with a non-Duration value on the right returns an
// error.  The `case time.Duration:` branch asserts v2 is also a Duration; when
// that fails it returns ErrInvalidTypeForOperation.
func Test_notEqualByteCode_TimeDurationVsNonDuration(t *testing.T) {
	tc := newTestContext(t).withStack(time.Second, 42)

	// We expect an error, not a boolean result.
	if err := notEqualByteCode(tc.ctx, nil); err == nil {
		t.Error("expected error for time.Duration != int, got nil")
	}
}

// Test_notEqualByteCode_TimeTimeEqual verifies that two equal time.Time values
// return false from !=.
func Test_notEqualByteCode_TimeTimeEqual(t *testing.T) {
	ts := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	tc := newTestContext(t).withStack(ts, ts)

	tc.assertNoError(notEqualByteCode(tc.ctx, nil))
	tc.assertTopStack(false)
}

// Test_notEqualByteCode_TimeTimeNotEqual verifies that two different time.Time
// values return true from !=.
func Test_notEqualByteCode_TimeTimeNotEqual(t *testing.T) {
	t1 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	tc := newTestContext(t).withStack(t1, t2)

	tc.assertNoError(notEqualByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// Test_notEqualByteCode_Float32Equal verifies that two equal float32 values
// return false from !=.  float32 is handled in the inner switch after the outer
// switch falls through to `default:`.
func Test_notEqualByteCode_Float32Equal(t *testing.T) {
	tc := newTestContext(t).withStack(float32(3.14), float32(3.14))

	tc.assertNoError(notEqualByteCode(tc.ctx, nil))
	tc.assertTopStack(false)
}

// Test_notEqualByteCode_Float32NotEqual verifies that different float32 values
// return true from !=.
func Test_notEqualByteCode_Float32NotEqual(t *testing.T) {
	tc := newTestContext(t).withStack(float32(1.0), float32(2.0))

	tc.assertNoError(notEqualByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// Test_notEqualByteCode_Int8NotEqual verifies that different int8 values return
// true from !=.  int8 is handled in the `case byte, int32, int8, int16, int, int64:`
// branch of the inner switch.
func Test_notEqualByteCode_Int8NotEqual(t *testing.T) {
	tc := newTestContext(t).withStack(int8(10), int8(20))

	tc.assertNoError(notEqualByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// Test_notEqualByteCode_Int16NotEqual verifies int16 inequality.
func Test_notEqualByteCode_Int16NotEqual(t *testing.T) {
	tc := newTestContext(t).withStack(int16(100), int16(200))

	tc.assertNoError(notEqualByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// Test_notEqualByteCode_Int32Equal verifies that equal int32 values return false
// from !=.
func Test_notEqualByteCode_Int32Equal(t *testing.T) {
	tc := newTestContext(t).withStack(int32(500), int32(500))

	tc.assertNoError(notEqualByteCode(tc.ctx, nil))
	tc.assertTopStack(false)
}

// Test_notEqualByteCode_Int64NotEqual verifies int64 inequality.
func Test_notEqualByteCode_Int64NotEqual(t *testing.T) {
	tc := newTestContext(t).withStack(int64(9999), int64(1000))

	tc.assertNoError(notEqualByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// Test_notEqualByteCode_Uint16NotEqual verifies uint16 inequality.
// uint16 is handled in the `case uint16, uint32, uint, uint64:` branch of the
// inner switch.
func Test_notEqualByteCode_Uint16NotEqual(t *testing.T) {
	tc := newTestContext(t).withStack(uint16(10), uint16(20))

	tc.assertNoError(notEqualByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// Test_notEqualByteCode_Uint32NotEqual verifies uint32 inequality.
func Test_notEqualByteCode_Uint32NotEqual(t *testing.T) {
	tc := newTestContext(t).withStack(uint32(100), uint32(200))

	tc.assertNoError(notEqualByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// Test_notEqualByteCode_BoolEqual verifies that two equal bool values return
// false from !=.  bool is handled in the inner switch's `case bool:` branch.
func Test_notEqualByteCode_BoolEqual(t *testing.T) {
	tc := newTestContext(t).withStack(true, true)

	tc.assertNoError(notEqualByteCode(tc.ctx, nil))
	tc.assertTopStack(false)
}

// Test_notEqualByteCode_StructEqual verifies that two *data.Struct values with
// identical contents return false from !=.  The `case *data.Struct:` branch
// uses reflect.DeepEqual for comparison (COMPARE-2 fixed this to use pointer
// types instead of value types).
func Test_notEqualByteCode_StructEqual(t *testing.T) {
	// Build two structs with the same type and the same field value.
	// reflect.DeepEqual compares both the type pointer and the data map, so we
	// use identical construction sequences to produce equal structs.
	structType := data.StructureType(data.Field{Name: "x", Type: data.IntType})
	s1 := data.NewStruct(structType)
	s1.SetAlways("x", 1)

	s2 := data.NewStruct(structType)
	
	s2.SetAlways("x", 1)

	tc := newTestContext(t).withStack(s1, s2)

	tc.assertNoError(notEqualByteCode(tc.ctx, nil))
	// Same contents → they are equal → != returns false.
	tc.assertTopStack(false)
}

// Test_notEqualByteCode_StructNotEqual verifies that two *data.Struct values
// with different field contents return true from !=.
func Test_notEqualByteCode_StructNotEqual(t *testing.T) {
	structType := data.StructureType(data.Field{Name: "x", Type: data.IntType})
	s1 := data.NewStruct(structType)
	s1.SetAlways("x", 1)

	s2 := data.NewStruct(structType)
	
	s2.SetAlways("x", 2) // different value

	tc := newTestContext(t).withStack(s1, s2)

	tc.assertNoError(notEqualByteCode(tc.ctx, nil))
	// Different contents → not equal → != returns true.
	tc.assertTopStack(true)
}

// Test_notEqualByteCode_StackUnderflow verifies that a stack underflow returns
// an error.  Pushing only one value means the second Pop (for v1) will fail.
//
// Before the COMPARE-4 fix, this error was returned raw (without module/line
// decoration).  After the fix, c.runtimeError wraps it consistently with all
// other error returns in the comparison operators.
func Test_notEqualByteCode_StackUnderflow(t *testing.T) {
	tc := newTestContext(t).withStack(42) // only one item — second pop fails

	tc.assertError(notEqualByteCode(tc.ctx, nil), errors.ErrStackUnderflow)
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 10: Additional greaterThanByteCode tests — float32, additional int
//             and uint variants, composite-type rejection, nil, and underflow
// ─────────────────────────────────────────────────────────────────────────────

// Test_greaterThanByteCode_Float32Greater verifies that a larger float32 is
// reported as greater.  float32 has its own case in the inner switch because
// float32 and float64 are distinct Go types — they don't mix after Normalize.
func Test_greaterThanByteCode_Float32Greater(t *testing.T) {
	tc := newTestContext(t).withStack(float32(9.0), float32(1.0))

	tc.assertNoError(greaterThanByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// Test_greaterThanByteCode_Float32NotGreater verifies that a smaller float32
// returns false from >.
func Test_greaterThanByteCode_Float32NotGreater(t *testing.T) {
	tc := newTestContext(t).withStack(float32(1.0), float32(9.0))

	tc.assertNoError(greaterThanByteCode(tc.ctx, nil))
	tc.assertTopStack(false)
}

// Test_greaterThanByteCode_Int16Greater verifies int16 greater-than.  int16 is
// included in the `case byte, int8, int16, int32, int, int64:` branch of the
// inner switch via data.Int64 conversion.
func Test_greaterThanByteCode_Int16Greater(t *testing.T) {
	tc := newTestContext(t).withStack(int16(200), int16(100))

	tc.assertNoError(greaterThanByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// Test_greaterThanByteCode_Int32Greater verifies int32 greater-than.
func Test_greaterThanByteCode_Int32Greater(t *testing.T) {
	tc := newTestContext(t).withStack(int32(1000), int32(500))

	tc.assertNoError(greaterThanByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// Test_greaterThanByteCode_Int64Greater verifies int64 greater-than.
func Test_greaterThanByteCode_Int64Greater(t *testing.T) {
	tc := newTestContext(t).withStack(int64(9999), int64(1))

	tc.assertNoError(greaterThanByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// Test_greaterThanByteCode_Uint16Greater verifies uint16 greater-than.  uint16
// is included in the `case uint16, uint32, uint, uint64:` branch and compared
// via data.UInt64.
func Test_greaterThanByteCode_Uint16Greater(t *testing.T) {
	tc := newTestContext(t).withStack(uint16(50), uint16(10))

	tc.assertNoError(greaterThanByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// Test_greaterThanByteCode_Uint32Greater verifies uint32 greater-than.
func Test_greaterThanByteCode_Uint32Greater(t *testing.T) {
	tc := newTestContext(t).withStack(uint32(200), uint32(100))

	tc.assertNoError(greaterThanByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// Test_greaterThanByteCode_StructInvalid verifies that comparing two struct
// values with > returns an error.  The outer switch matches `*data.Struct` and
// returns ErrInvalidType before any comparison is attempted.
func Test_greaterThanByteCode_StructInvalid(t *testing.T) {
	s := data.NewStructFromMap(map[string]any{"x": 1})
	tc := newTestContext(t).withStack(s, s)

	if err := greaterThanByteCode(tc.ctx, nil); err == nil {
		t.Error("expected error for struct > struct, got nil")
	}
}

// Test_greaterThanByteCode_ArrayInvalid verifies that comparing two array values
// with > returns an error.
func Test_greaterThanByteCode_ArrayInvalid(t *testing.T) {
	arr := data.NewArrayFromInterfaces(data.IntType, 1, 2, 3)
	tc := newTestContext(t).withStack(arr, arr)

	if err := greaterThanByteCode(tc.ctx, nil); err == nil {
		t.Error("expected error for array > array, got nil")
	}
}

// Test_greaterThanByteCode_NilReturnsFalse verifies that nil > nil returns
// false.  The nil guard fires before the type switch: when either operand is
// nil the function pushes false and returns without error.
func Test_greaterThanByteCode_NilReturnsFalse(t *testing.T) {
	tc := newTestContext(t).withStack(nil, nil)

	tc.assertNoError(greaterThanByteCode(tc.ctx, nil))
	tc.assertTopStack(false)
}

// Test_greaterThanByteCode_StackUnderflow verifies ErrStackUnderflow when only
// one value is on the stack.
//
// After the COMPARE-4 fix, the error from getComparisonTerms is now wrapped
// with c.runtimeError (matching greaterThanByteCode's existing behavior), so
// all five ordering operators are consistent.
func Test_greaterThanByteCode_StackUnderflow(t *testing.T) {
	tc := newTestContext(t).withStack(42)

	tc.assertError(greaterThanByteCode(tc.ctx, nil), errors.ErrStackUnderflow)
}

// Test_greaterThanByteCode_StrictMode_TypeMismatch verifies that comparing an
// int with a float64 in strict mode returns ErrTypeMismatch.
func Test_greaterThanByteCode_StrictMode_TypeMismatch(t *testing.T) {
	tc := newTestContext(t).
		withTypeStrictness(defs.StrictTypeEnforcement).
		withStack(10, float64(3))

	tc.assertError(greaterThanByteCode(tc.ctx, nil), errors.ErrTypeMismatch)
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 11: Additional lessThanByteCode tests — int variants, uint variants,
//             composite-type rejection, underflow, and numeric promotion
// ─────────────────────────────────────────────────────────────────────────────

// Test_lessThanByteCode_Int16Less verifies int16 less-than.
func Test_lessThanByteCode_Int16Less(t *testing.T) {
	tc := newTestContext(t).withStack(int16(10), int16(20))

	tc.assertNoError(lessThanByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// Test_lessThanByteCode_Int32Less verifies int32 less-than.
func Test_lessThanByteCode_Int32Less(t *testing.T) {
	tc := newTestContext(t).withStack(int32(100), int32(200))

	tc.assertNoError(lessThanByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// Test_lessThanByteCode_Int64Less verifies int64 less-than.
func Test_lessThanByteCode_Int64Less(t *testing.T) {
	tc := newTestContext(t).withStack(int64(1), int64(9999))

	tc.assertNoError(lessThanByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// Test_lessThanByteCode_Uint16Less verifies uint16 less-than.
func Test_lessThanByteCode_Uint16Less(t *testing.T) {
	tc := newTestContext(t).withStack(uint16(5), uint16(10))

	tc.assertNoError(lessThanByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// Test_lessThanByteCode_Uint64Less verifies uint64 less-than.
func Test_lessThanByteCode_Uint64Less(t *testing.T) {
	tc := newTestContext(t).withStack(uint64(1), uint64(1000))

	tc.assertNoError(lessThanByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// Test_lessThanByteCode_StructInvalid verifies that comparing two struct values
// with < returns an error.
func Test_lessThanByteCode_StructInvalid(t *testing.T) {
	s := data.NewStructFromMap(map[string]any{"x": 1})
	tc := newTestContext(t).withStack(s, s)

	if err := lessThanByteCode(tc.ctx, nil); err == nil {
		t.Error("expected error for struct < struct, got nil")
	}
}

// Test_lessThanByteCode_ArrayInvalid verifies that comparing two array values
// with < returns an error.
func Test_lessThanByteCode_ArrayInvalid(t *testing.T) {
	arr := data.NewArrayFromInterfaces(data.IntType, 1, 2, 3)
	tc := newTestContext(t).withStack(arr, arr)

	if err := lessThanByteCode(tc.ctx, nil); err == nil {
		t.Error("expected error for array < array, got nil")
	}
}

// Test_lessThanByteCode_StackUnderflow verifies ErrStackUnderflow when fewer
// than two values are on the stack.
//
// Before the COMPARE-4 fix, this error was returned raw from getComparisonTerms
// without module/line decoration.  After the fix, c.runtimeError is called,
// making lessThanByteCode consistent with greaterThanByteCode.
func Test_lessThanByteCode_StackUnderflow(t *testing.T) {
	tc := newTestContext(t).withStack(42)

	tc.assertError(lessThanByteCode(tc.ctx, nil), errors.ErrStackUnderflow)
}

// Test_lessThanByteCode_NumericPromotion verifies that in dynamic mode an int
// and a float64 are promoted to the same type by data.Normalize before
// comparison.  1 < 2.5 should return true even though the operands have
// different Go types.
func Test_lessThanByteCode_NumericPromotion(t *testing.T) {
	// v1 = int(1), v2 = float64(2.5); Normalize promotes v1 to float64(1.0).
	tc := newTestContext(t).withStack(1, float64(2.5))

	tc.assertNoError(lessThanByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 12: Additional lessThanOrEqualByteCode tests — int variants, uint
//             variants, composite rejection, nil, float32, underflow, strict
//             mode mismatch, and numeric promotion
// ─────────────────────────────────────────────────────────────────────────────

// Test_lessThanOrEqualByteCode_Int16LessEqual verifies int16 <=.
func Test_lessThanOrEqualByteCode_Int16LessEqual(t *testing.T) {
	tc := newTestContext(t).withStack(int16(10), int16(20))

	tc.assertNoError(lessThanOrEqualByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// Test_lessThanOrEqualByteCode_Int32LessEqual verifies int32 <=.
func Test_lessThanOrEqualByteCode_Int32LessEqual(t *testing.T) {
	tc := newTestContext(t).withStack(int32(5), int32(5))

	tc.assertNoError(lessThanOrEqualByteCode(tc.ctx, nil))
	tc.assertTopStack(true) // equal → still satisfies <=
}

// Test_lessThanOrEqualByteCode_Int64LessEqual verifies int64 <=.
func Test_lessThanOrEqualByteCode_Int64LessEqual(t *testing.T) {
	tc := newTestContext(t).withStack(int64(100), int64(200))

	tc.assertNoError(lessThanOrEqualByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// Test_lessThanOrEqualByteCode_Uint32LessEqual verifies uint32 <=.
// Section 7 tests uint16; this fills the uint32 gap.
func Test_lessThanOrEqualByteCode_Uint32LessEqual(t *testing.T) {
	tc := newTestContext(t).withStack(uint32(3), uint32(7))

	tc.assertNoError(lessThanOrEqualByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// Test_lessThanOrEqualByteCode_Uint64LessEqual verifies uint64 <=.
func Test_lessThanOrEqualByteCode_Uint64LessEqual(t *testing.T) {
	tc := newTestContext(t).withStack(uint64(50), uint64(50))

	tc.assertNoError(lessThanOrEqualByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// Test_lessThanOrEqualByteCode_StructInvalid verifies that comparing two struct
// values with <= returns an error.
func Test_lessThanOrEqualByteCode_StructInvalid(t *testing.T) {
	s := data.NewStructFromMap(map[string]any{"x": 1})
	tc := newTestContext(t).withStack(s, s)

	if err := lessThanOrEqualByteCode(tc.ctx, nil); err == nil {
		t.Error("expected error for struct <= struct, got nil")
	}
}

// Test_lessThanOrEqualByteCode_ArrayInvalid verifies that comparing two array
// values with <= returns an error.
func Test_lessThanOrEqualByteCode_ArrayInvalid(t *testing.T) {
	arr := data.NewArrayFromInterfaces(data.IntType, 1, 2, 3)
	tc := newTestContext(t).withStack(arr, arr)

	if err := lessThanOrEqualByteCode(tc.ctx, nil); err == nil {
		t.Error("expected error for array <= array, got nil")
	}
}

// Test_lessThanOrEqualByteCode_StackUnderflow verifies ErrStackUnderflow when
// fewer than two values are on the stack.
//
// After the COMPARE-4 fix, the raw error from getComparisonTerms is now wrapped
// with c.runtimeError, consistent with greaterThanByteCode.
func Test_lessThanOrEqualByteCode_StackUnderflow(t *testing.T) {
	tc := newTestContext(t).withStack(42)

	tc.assertError(lessThanOrEqualByteCode(tc.ctx, nil), errors.ErrStackUnderflow)
}

// Test_lessThanOrEqualByteCode_NilReturnsFalse verifies that nil <= nil returns
// false.  When either operand is nil the function pushes false immediately
// without entering the type switch.
func Test_lessThanOrEqualByteCode_NilReturnsFalse(t *testing.T) {
	tc := newTestContext(t).withStack(nil, nil)

	tc.assertNoError(lessThanOrEqualByteCode(tc.ctx, nil))
	tc.assertTopStack(false)
}

// Test_lessThanOrEqualByteCode_Float32LessEqual verifies float32 <=.  float32
// has its own explicit case in the inner switch.
func Test_lessThanOrEqualByteCode_Float32LessEqual(t *testing.T) {
	tc := newTestContext(t).withStack(float32(1.5), float32(2.5))

	tc.assertNoError(lessThanOrEqualByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// Test_lessThanOrEqualByteCode_StrictMode_TypeMismatch verifies that comparing
// an int with a float64 in strict mode returns ErrTypeMismatch for <=.
func Test_lessThanOrEqualByteCode_StrictMode_TypeMismatch(t *testing.T) {
	tc := newTestContext(t).
		withTypeStrictness(defs.StrictTypeEnforcement).
		withStack(3, float64(7))

	tc.assertError(lessThanOrEqualByteCode(tc.ctx, nil), errors.ErrTypeMismatch)
}

// Test_lessThanOrEqualByteCode_NumericPromotion verifies that in dynamic mode
// an int and float64 are promoted to the same type before comparison.
// 1 <= 1.0 should return true (int 1 promoted to float64 1.0).
func Test_lessThanOrEqualByteCode_NumericPromotion(t *testing.T) {
	tc := newTestContext(t).withStack(1, float64(1.0))

	tc.assertNoError(lessThanOrEqualByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 13: Additional greaterThanOrEqualByteCode tests — float32, int
//             variants, uint variants, composite rejection, nil, underflow,
//             and strict mode mismatch
// ─────────────────────────────────────────────────────────────────────────────

// Test_greaterThanOrEqualByteCode_Float32GreaterEqual verifies float32 >=.
// float32 has its own case in the inner switch.
func Test_greaterThanOrEqualByteCode_Float32GreaterEqual(t *testing.T) {
	tc := newTestContext(t).withStack(float32(5.0), float32(5.0))

	tc.assertNoError(greaterThanOrEqualByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// Test_greaterThanOrEqualByteCode_Float32Greater verifies float32 >= when the
// left side is strictly greater.
func Test_greaterThanOrEqualByteCode_Float32Greater(t *testing.T) {
	tc := newTestContext(t).withStack(float32(9.0), float32(1.0))

	tc.assertNoError(greaterThanOrEqualByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// Test_greaterThanOrEqualByteCode_Int16GreaterEqual verifies int16 >=.
func Test_greaterThanOrEqualByteCode_Int16GreaterEqual(t *testing.T) {
	tc := newTestContext(t).withStack(int16(20), int16(10))

	tc.assertNoError(greaterThanOrEqualByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// Test_greaterThanOrEqualByteCode_Int32GreaterEqual verifies int32 >=.
func Test_greaterThanOrEqualByteCode_Int32GreaterEqual(t *testing.T) {
	tc := newTestContext(t).withStack(int32(100), int32(100))

	tc.assertNoError(greaterThanOrEqualByteCode(tc.ctx, nil))
	tc.assertTopStack(true) // equal also satisfies >=
}

// Test_greaterThanOrEqualByteCode_Int64GreaterEqual verifies int64 >=.
func Test_greaterThanOrEqualByteCode_Int64GreaterEqual(t *testing.T) {
	tc := newTestContext(t).withStack(int64(9999), int64(1))

	tc.assertNoError(greaterThanOrEqualByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// Test_greaterThanOrEqualByteCode_Uint16GreaterEqual verifies uint16 >=.
// Section 7 tests uint32; this fills the uint16 gap.
func Test_greaterThanOrEqualByteCode_Uint16GreaterEqual(t *testing.T) {
	tc := newTestContext(t).withStack(uint16(10), uint16(5))

	tc.assertNoError(greaterThanOrEqualByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// Test_greaterThanOrEqualByteCode_Uint64GreaterEqual verifies uint64 >=.
func Test_greaterThanOrEqualByteCode_Uint64GreaterEqual(t *testing.T) {
	tc := newTestContext(t).withStack(uint64(1000), uint64(1000))

	tc.assertNoError(greaterThanOrEqualByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}

// Test_greaterThanOrEqualByteCode_StructInvalid verifies that comparing two
// struct values with >= returns an error.
func Test_greaterThanOrEqualByteCode_StructInvalid(t *testing.T) {
	s := data.NewStructFromMap(map[string]any{"x": 1})
	tc := newTestContext(t).withStack(s, s)

	if err := greaterThanOrEqualByteCode(tc.ctx, nil); err == nil {
		t.Error("expected error for struct >= struct, got nil")
	}
}

// Test_greaterThanOrEqualByteCode_ArrayInvalid verifies that comparing two
// array values with >= returns an error.
func Test_greaterThanOrEqualByteCode_ArrayInvalid(t *testing.T) {
	arr := data.NewArrayFromInterfaces(data.IntType, 1, 2, 3)
	tc := newTestContext(t).withStack(arr, arr)

	if err := greaterThanOrEqualByteCode(tc.ctx, nil); err == nil {
		t.Error("expected error for array >= array, got nil")
	}
}

// Test_greaterThanOrEqualByteCode_StackUnderflow verifies ErrStackUnderflow
// when fewer than two values are on the stack.
//
// After the COMPARE-4 fix, the raw error from getComparisonTerms is wrapped
// with c.runtimeError in greaterThanOrEqualByteCode, consistent with the other
// five comparison operators.
func Test_greaterThanOrEqualByteCode_StackUnderflow(t *testing.T) {
	tc := newTestContext(t).withStack(42)

	tc.assertError(greaterThanOrEqualByteCode(tc.ctx, nil), errors.ErrStackUnderflow)
}

// Test_greaterThanOrEqualByteCode_NilReturnsFalse verifies that nil >= nil
// returns false.  The nil guard fires before the type switch and pushes false.
func Test_greaterThanOrEqualByteCode_NilReturnsFalse(t *testing.T) {
	tc := newTestContext(t).withStack(nil, nil)

	tc.assertNoError(greaterThanOrEqualByteCode(tc.ctx, nil))
	tc.assertTopStack(false)
}

// Test_greaterThanOrEqualByteCode_StrictMode_TypeMismatch verifies that
// comparing an int with a float64 in strict mode returns ErrTypeMismatch for >=.
func Test_greaterThanOrEqualByteCode_StrictMode_TypeMismatch(t *testing.T) {
	tc := newTestContext(t).
		withTypeStrictness(defs.StrictTypeEnforcement).
		withStack(10, float64(3))

	tc.assertError(greaterThanOrEqualByteCode(tc.ctx, nil), errors.ErrTypeMismatch)
}

// Test_greaterThanOrEqualByteCode_StringGreaterNotEqual verifies string >=
// where the left side is lexicographically greater (not equal).
// "z" >= "a" must return true.
func Test_greaterThanOrEqualByteCode_StringGreaterNotEqual(t *testing.T) {
	tc := newTestContext(t).withStack("z", "a")

	tc.assertNoError(greaterThanOrEqualByteCode(tc.ctx, nil))
	tc.assertTopStack(true)
}
