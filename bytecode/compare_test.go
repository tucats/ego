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

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
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
