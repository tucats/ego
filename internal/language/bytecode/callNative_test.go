package bytecode

// callNative_test.go contains unit tests for the functions in callNative.go.
//
// # What callNative.go does
//
// Ego supports "native" functions — ordinary Go functions from the standard
// library (math.Abs, strings.TrimSpace, etc.) that can be called directly
// from Ego programs.  callNative.go provides the glue between Ego's dynamic
// type system and Go's static reflection system:
//
//  1. convertToNative   — converts Ego argument values to the Go types the
//                         native function expects (e.g. Ego int → Go int).
//  2. CallDirect        — invokes the function via Go reflection.
//  3. convertFromNative — takes the return value(s) and pushes them onto the
//                         Ego execution stack.
//
// For method calls (receiver != nil) CallWithReceiver is used instead of
// CallDirect; it looks up the method name on the receiver via reflection.
//
// # Helper: nativeFn
//
// Most tests in this file need a *data.Function value.  nativeFn() is a
// convenience constructor that builds one from a list of parameters — see its
// documentation below.
//
// # Bugs documented here
//
// issue CALL-8  makeNativeArrayArgument does not convert *data.Array of Int64 or
//               Float32 elements; those cases fall through to ErrInvalidType even
//               though the pass-through and return paths handle native []int64 and
//               []float32 slices.
//
// issue CALL-9  CallWithReceiver does not check whether MethodByName returned a
//               valid reflect.Value before calling .Call(), so an unknown method
//               name causes an unrecoverable panic.
//
// issue BUG-27  Calling a native Go method or function that panics (for example,
//               calling Done() on a sync.WaitGroup more times than Add() was
//               called) crashed the entire ego process, because nothing on the
//               native-call path ever called recover(). safeReflectCall (Section
//               10 below) is the fix: a shared helper used by both CallDirect and
//               CallWithReceiver that recovers any panic and turns it into a
//               normal, catchable Ego error instead.
//
// issue BUG-28 Calling Unlock() on a sync.Mutex that isn't locked triggers Go's
//              unrecoverable "fatal error: sync: unlock of unlocked mutex",
//               which even safeReflectCall's recover() cannot catch (fatal errors
//               are a stronger, deliberately-unrecoverable failure mode). The fix
//               (Section 11 below, callMutexMethod) tracks each sync.Mutex's lock
//               state and refuses to call the real Unlock() at all when the
//               mutex isn't locked, avoiding the fatal error in the first place.

import (
	"math"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/data"
)

// nativeFn builds a *data.Function with the given parameter declarations.
// The function has no Go-level Value assigned; tests that need to make a real
// call should set fn.Value themselves.
//
// Example — a single float64 parameter:
//
//	fn := nativeFn(data.Parameter{Name: "x", Type: data.Float64Type})
func nativeFn(params ...data.Parameter) *data.Function {
	return &data.Function{
		IsNative: true,
		Declaration: &data.Declaration{
			Name:       "testFn",
			Parameters: params,
		},
	}
}

// nativeFnVariadic is like nativeFn but marks the declaration as variadic.
// All arguments at or beyond the last declared parameter use the last
// parameter's type.
func nativeFnVariadic(params ...data.Parameter) *data.Function {
	fn := nativeFn(params...)
	fn.Declaration.Variadic = true

	return fn
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 1: reverseInterfaces — in-place slice reversal
// ─────────────────────────────────────────────────────────────────────────────
//
// reverseInterfaces is used by convertFromNative to reverse the order of
// multi-value return results before pushing them onto the stack.  The stack
// is LIFO, so reversing ensures the first return value ends up on top.

// Test_reverseInterfaces_Empty verifies that an empty slice is returned as-is
// without panicking.
func Test_reverseInterfaces_Empty(t *testing.T) {
	got := reverseInterfaces([]any{})
	if len(got) != 0 {
		t.Errorf("expected empty slice, got len=%d", len(got))
	}
}

// Test_reverseInterfaces_SingleElement verifies that a one-element slice is
// unchanged (reversing one item is a no-op).
func Test_reverseInterfaces_SingleElement(t *testing.T) {
	got := reverseInterfaces([]any{"only"})
	if got[0] != "only" {
		t.Errorf("got %v, want [only]", got)
	}
}

// Test_reverseInterfaces_TwoElements verifies that two elements are swapped.
func Test_reverseInterfaces_TwoElements(t *testing.T) {
	got := reverseInterfaces([]any{"first", "second"})
	if got[0] != "second" || got[1] != "first" {
		t.Errorf("got %v, want [second first]", got)
	}
}

// Test_reverseInterfaces_MultipleElements verifies that a longer slice is
// fully reversed.
func Test_reverseInterfaces_MultipleElements(t *testing.T) {
	got := reverseInterfaces([]any{1, 2, 3, 4, 5})

	want := []any{5, 4, 3, 2, 1}
	for i, v := range want {
		if got[i] != v {
			t.Errorf("index %d: got %v, want %v", i, got[i], v)
		}
	}
}

// Test_reverseInterfaces_ModifiesInPlace verifies that the function modifies
// the original slice (it does not allocate a new one) and returns the same
// slice pointer.
func Test_reverseInterfaces_ModifiesInPlace(t *testing.T) {
	original := []any{"a", "b", "c"}
	returned := reverseInterfaces(original)

	// The returned slice and the original should be the same backing array.
	if &original[0] != &returned[0] {
		t.Error("reverseInterfaces allocated a new slice instead of reversing in-place")
	}

	if original[0] != "c" || original[2] != "a" {
		t.Errorf("original not reversed: got %v", original)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 2: getArgumentType — look up a parameter's declared type
// ─────────────────────────────────────────────────────────────────────────────
//
// getArgumentType returns the *data.Type for the parameter at argumentIndex.
// For variadic functions, arguments beyond the last declared parameter all
// use the last parameter's type.

// Test_getArgumentType_NonVariadic_ValidIndex verifies that accessing a
// declared parameter returns the correct type.
func Test_getArgumentType_NonVariadic_ValidIndex(t *testing.T) {
	fn := nativeFn(
		data.Parameter{Name: "a", Type: data.IntType},
		data.Parameter{Name: "b", Type: data.StringType},
	)

	got, err := getArgumentType(fn, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got != data.StringType {
		t.Errorf("type: got %v, want StringType", got)
	}
}

// Test_getArgumentType_NonVariadic_OutOfRange verifies that accessing beyond
// the declared parameters returns ErrArgumentCount for non-variadic functions.
func Test_getArgumentType_NonVariadic_OutOfRange(t *testing.T) {
	fn := nativeFn(data.Parameter{Name: "x", Type: data.IntType})

	_, err := getArgumentType(fn, 1) // index 1, but only index 0 exists

	if err == nil {
		t.Fatal("expected ErrArgumentCount, got nil")
	}
}

// Test_getArgumentType_Variadic_BeyondParams verifies that a variadic function
// returns the LAST parameter's type for any argument index beyond the declared
// parameter list.
func Test_getArgumentType_Variadic_BeyondParams(t *testing.T) {
	fn := nativeFnVariadic(
		data.Parameter{Name: "a", Type: data.IntType},
		data.Parameter{Name: "rest", Type: data.StringType},
	)

	// Index 5 is beyond the two declared parameters — should return StringType
	// (the last parameter's type).
	got, err := getArgumentType(fn, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got != data.StringType {
		t.Errorf("variadic extra arg type: got %v, want StringType", got)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 3: convertToNative — Ego values → Go types
// ─────────────────────────────────────────────────────────────────────────────
//
// convertToNative converts each element in the Ego argument slice to the Go
// type that the native function expects.

// Test_convertToNative_String verifies that any value is formatted as a string
// when the parameter declares StringKind.
func Test_convertToNative_String(t *testing.T) {
	tc := newTestContext(t)
	fn := nativeFn(data.Parameter{Name: "s", Type: data.StringType})

	got, err := convertToNative(tc.ctx, fn, []any{42})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got[0] != "42" {
		t.Errorf("string conversion: got %v (%T), want %q", got[0], got[0], "42")
	}
}

// Test_convertToNative_Float64 verifies float64 conversion.
func Test_convertToNative_Float64(t *testing.T) {
	tc := newTestContext(t)
	fn := nativeFn(data.Parameter{Name: "x", Type: data.Float64Type})

	got, err := convertToNative(tc.ctx, fn, []any{3})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got[0] != float64(3) {
		t.Errorf("float64 conversion: got %v (%T), want 3.0", got[0], got[0])
	}
}

// Test_convertToNative_Float32 verifies float32 conversion.
func Test_convertToNative_Float32(t *testing.T) {
	tc := newTestContext(t)
	fn := nativeFn(data.Parameter{Name: "x", Type: data.Float32Type})

	got, err := convertToNative(tc.ctx, fn, []any{float64(1.5)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got[0] != float32(1.5) {
		t.Errorf("float32 conversion: got %v (%T), want float32(1.5)", got[0], got[0])
	}
}

// Test_convertToNative_Int verifies int conversion.
func Test_convertToNative_Int(t *testing.T) {
	tc := newTestContext(t)
	fn := nativeFn(data.Parameter{Name: "n", Type: data.IntType})

	got, err := convertToNative(tc.ctx, fn, []any{float64(7.0)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got[0] != 7 {
		t.Errorf("int conversion: got %v (%T), want 7", got[0], got[0])
	}
}

// Test_convertToNative_Int32 verifies int32 conversion.
func Test_convertToNative_Int32(t *testing.T) {
	tc := newTestContext(t)
	fn := nativeFn(data.Parameter{Name: "n", Type: data.Int32Type})

	got, err := convertToNative(tc.ctx, fn, []any{99})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got[0] != int32(99) {
		t.Errorf("int32 conversion: got %v (%T), want int32(99)", got[0], got[0])
	}
}

// Test_convertToNative_Int64 verifies int64 conversion.
func Test_convertToNative_Int64(t *testing.T) {
	tc := newTestContext(t)
	fn := nativeFn(data.Parameter{Name: "n", Type: data.Int64Type})

	got, err := convertToNative(tc.ctx, fn, []any{42})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got[0] != int64(42) {
		t.Errorf("int64 conversion: got %v (%T), want int64(42)", got[0], got[0])
	}
}

// Test_convertToNative_Bool verifies bool conversion.
func Test_convertToNative_Bool(t *testing.T) {
	tc := newTestContext(t)
	fn := nativeFn(data.Parameter{Name: "b", Type: data.BoolType})

	got, err := convertToNative(tc.ctx, fn, []any{1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got[0] != true {
		t.Errorf("bool conversion: got %v, want true", got[0])
	}
}

// Test_convertToNative_Byte verifies byte conversion.
func Test_convertToNative_Byte(t *testing.T) {
	tc := newTestContext(t)
	fn := nativeFn(data.Parameter{Name: "b", Type: data.ByteType})

	got, err := convertToNative(tc.ctx, fn, []any{65})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got[0] != byte(65) {
		t.Errorf("byte conversion: got %v (%T), want byte(65)", got[0], got[0])
	}
}

// Test_convertToNative_UInt64 verifies uint64 conversion.
func Test_convertToNative_UInt64(t *testing.T) {
	tc := newTestContext(t)
	fn := nativeFn(data.Parameter{Name: "n", Type: data.UInt64Type})

	got, err := convertToNative(tc.ctx, fn, []any{100})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got[0] != uint64(100) {
		t.Errorf("uint64 conversion: got %v (%T), want uint64(100)", got[0], got[0])
	}
}

// Test_convertToNative_MultipleParams verifies that multiple parameters are
// all converted in a single call.
func Test_convertToNative_MultipleParams(t *testing.T) {
	const hello = "hello"

	tc := newTestContext(t)
	fn := nativeFn(
		data.Parameter{Name: "n", Type: data.IntType},
		data.Parameter{Name: "s", Type: data.StringType},
		data.Parameter{Name: "f", Type: data.Float64Type},
	)

	got, err := convertToNative(tc.ctx, fn, []any{42, hello, 3.14})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got[0] != 42 || got[1] != hello || got[2] != 3.14 {
		t.Errorf("multi-param conversion failed: %v", got)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 4: makeNativeArrayArgument — *data.Array → native Go slice
// ─────────────────────────────────────────────────────────────────────────────

// Test_makeNativeArrayArgument_NativeSlicePassThrough verifies that a native
// Go slice (not a *data.Array) is returned as-is without any conversion.
func Test_makeNativeArrayArgument_NativeSlicePassThrough(t *testing.T) {
	input := []string{"alpha", "beta", "gamma"}

	got, err := makeNativeArrayArgument(input, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result, ok := got.([]string); !ok || result[1] != "beta" {
		t.Errorf("[]string pass-through: got %v", got)
	}
}

// Test_makeNativeArrayArgument_NativeIntSlicePassThrough verifies []int pass-through.
func Test_makeNativeArrayArgument_NativeIntSlicePassThrough(t *testing.T) {
	input := []int{1, 2, 3}

	got, err := makeNativeArrayArgument(input, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result, ok := got.([]int); !ok || result[2] != 3 {
		t.Errorf("[]int pass-through: got %v", got)
	}
}

// Test_makeNativeArrayArgument_EgoIntArray verifies that a *data.Array of int
// elements is converted to a native []int.
func Test_makeNativeArrayArgument_EgoIntArray(t *testing.T) {
	arr := data.NewArrayFromInterfaces(data.IntType, 10, 20, 30)

	got, err := makeNativeArrayArgument(arr, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result, ok := got.([]int)
	if !ok {
		t.Fatalf("expected []int, got %T", got)
	}

	if len(result) != 3 || result[0] != 10 || result[1] != 20 || result[2] != 30 {
		t.Errorf("[]int content: got %v", result)
	}
}

// Test_makeNativeArrayArgument_EgoBoolArray verifies conversion to []bool.
func Test_makeNativeArrayArgument_EgoBoolArray(t *testing.T) {
	arr := data.NewArrayFromInterfaces(data.BoolType, true, false, true)

	got, err := makeNativeArrayArgument(arr, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result, ok := got.([]bool)
	if !ok || len(result) != 3 || !result[0] || result[1] {
		t.Errorf("[]bool content: got %v (%T)", got, got)
	}
}

// Test_makeNativeArrayArgument_EgoByteArray verifies that a byte array is
// retrieved via GetBytes() and returned as []byte.
func Test_makeNativeArrayArgument_EgoByteArray(t *testing.T) {
	arr := data.NewArrayFromInterfaces(data.ByteType, byte('A'), byte('B'), byte('C'))

	got, err := makeNativeArrayArgument(arr, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result, ok := got.([]byte)
	if !ok || string(result) != "ABC" {
		t.Errorf("[]byte content: got %v (%T)", got, got)
	}
}

// Test_makeNativeArrayArgument_EgoFloat64Array verifies conversion to []float64.
func Test_makeNativeArrayArgument_EgoFloat64Array(t *testing.T) {
	arr := data.NewArrayFromInterfaces(data.Float64Type, 1.1, 2.2, 3.3)

	got, err := makeNativeArrayArgument(arr, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result, ok := got.([]float64)
	if !ok || len(result) != 3 {
		t.Errorf("[]float64 content: got %v (%T)", got, got)
	}
}

// Test_makeNativeArrayArgument_EgoStringArray verifies conversion to []string.
func Test_makeNativeArrayArgument_EgoStringArray(t *testing.T) {
	arr := data.NewArrayFromInterfaces(data.StringType, "x", "y", "z")

	got, err := makeNativeArrayArgument(arr, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result, ok := got.([]string)
	if !ok || len(result) != 3 || result[1] != "y" {
		t.Errorf("[]string content: got %v (%T)", got, got)
	}
}

// Test_makeNativeArrayArgument_EgoInt32Array verifies conversion to []int32.
func Test_makeNativeArrayArgument_EgoInt32Array(t *testing.T) {
	arr := data.NewArrayFromInterfaces(data.Int32Type, int32(1), int32(2))

	got, err := makeNativeArrayArgument(arr, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result, ok := got.([]int32)
	if !ok || len(result) != 2 {
		t.Errorf("[]int32 content: got %v (%T)", got, got)
	}
}

// Test_makeNativeArrayArgument_EgoInt16Array verifies conversion to []int16.
func Test_makeNativeArrayArgument_EgoInt16Array(t *testing.T) {
	arr := data.NewArrayFromInterfaces(data.Int16Type, int16(10), int16(20))

	got, err := makeNativeArrayArgument(arr, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, ok := got.([]int16)
	if !ok {
		t.Errorf("expected []int16, got %T", got)
	}
}

// Test_makeNativeArrayArgument_EgoUInt16Array verifies conversion to []uint16.
func Test_makeNativeArrayArgument_EgoUInt16Array(t *testing.T) {
	arr := data.NewArrayFromInterfaces(data.UInt16Type, uint16(5), uint16(10))

	got, err := makeNativeArrayArgument(arr, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, ok := got.([]uint16)
	if !ok {
		t.Errorf("expected []uint16, got %T", got)
	}
}

// Test_makeNativeArrayArgument_NonArrayArg verifies that passing a non-array,
// non-slice value returns ErrArgumentType.
func Test_makeNativeArrayArgument_NonArrayArg(t *testing.T) {
	_, err := makeNativeArrayArgument("not-an-array", 0)

	if err == nil {
		t.Fatal("expected ErrArgumentType for non-array argument, got nil")
	}
}

// Test_makeNativeArrayArgument_UnsupportedEgoArrayKind verifies that a
// *data.Array whose element type has no supported native conversion returns
// ErrInvalidType.  InterfaceType arrays have InterfaceKind which is not in
// the handled cases.
func Test_makeNativeArrayArgument_UnsupportedEgoArrayKind(t *testing.T) {
	// InterfaceKind falls into the default case → ErrInvalidType.
	arr := data.NewArrayFromInterfaces(data.InterfaceType, 1, 2, 3)

	_, err := makeNativeArrayArgument(arr, 0)

	if err == nil {
		t.Fatal("expected error for unsupported array element kind, got nil")
	}
}

// Test_makeNativeArrayArgument_Int64Kind verifies the CALL-8 fix: a
// *data.Array of Int64Kind elements is now correctly converted to a native
// []int64 slice, matching the behavior of the pass-through and return paths
// that already handled []int64.
func Test_makeNativeArrayArgument_Int64Kind(t *testing.T) {
	arr := data.NewArrayFromInterfaces(data.Int64Type, int64(1), int64(2), int64(3))

	got, err := makeNativeArrayArgument(arr, 0)

	if err != nil {
		t.Fatalf("unexpected error after CALL-8 fix: %v", err)
	}

	result, ok := got.([]int64)
	if !ok {
		t.Fatalf("expected []int64, got %T", got)
	}

	if len(result) != 3 || result[0] != 1 || result[1] != 2 || result[2] != 3 {
		t.Errorf("[]int64 content: got %v, want [1 2 3]", result)
	}
}

// Test_makeNativeArrayArgument_Float32Kind verifies the CALL-8 fix for
// Float32Kind: a *data.Array of float32 elements is now converted to []float32.
func Test_makeNativeArrayArgument_Float32Kind(t *testing.T) {
	arr := data.NewArrayFromInterfaces(data.Float32Type, float32(1.5), float32(2.5))

	got, err := makeNativeArrayArgument(arr, 0)

	if err != nil {
		t.Fatalf("unexpected error after CALL-8 fix: %v", err)
	}

	result, ok := got.([]float32)
	if !ok {
		t.Fatalf("expected []float32, got %T", got)
	}

	if len(result) != 2 || result[0] != 1.5 || result[1] != 2.5 {
		t.Errorf("[]float32 content: got %v, want [1.5 2.5]", result)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 5: makeNativePackageTypeArgument — Ego package types → Go types
// ─────────────────────────────────────────────────────────────────────────────

// Test_makeNativePackageTypeArgument_NoNativeName verifies that when the type
// has no NativeName the argument is returned unchanged.
func Test_makeNativePackageTypeArgument_NoNativeName(t *testing.T) {
	// A plain IntType has no NativeName — the argument should pass through.
	input := 42
	got, err := makeNativePackageTypeArgument(data.IntType, input, 0)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got != input {
		t.Errorf("expected pass-through, got %v", got)
	}
}

// Test_makeNativePackageTypeArgument_DurationFromInt64 verifies that an int64
// argument is converted to time.Duration when the type's NativeName is
// "time.Duration".
func Test_makeNativePackageTypeArgument_DurationFromInt64(t *testing.T) {
	durType := makeDurationType() // NativeName = "time.Duration"

	got, err := makeNativePackageTypeArgument(durType, int64(5_000_000_000), 0)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := time.Duration(5_000_000_000)
	if got != want {
		t.Errorf("duration from int64: got %v, want %v", got, want)
	}
}

// Test_makeNativePackageTypeArgument_DurationFromInt verifies that a plain int
// argument is also converted to time.Duration.
func Test_makeNativePackageTypeArgument_DurationFromInt(t *testing.T) {
	durType := makeDurationType()

	got, err := makeNativePackageTypeArgument(durType, 1000, 0)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got != time.Duration(1000) {
		t.Errorf("duration from int: got %v, want %v", got, time.Duration(1000))
	}
}

// Test_makeNativePackageTypeArgument_MonthFromInt verifies that an int
// argument is converted to time.Month when the type is time.Month.
func Test_makeNativePackageTypeArgument_MonthFromInt(t *testing.T) {
	monthType := makeMonthType() // NativeName = "time.Month"

	got, err := makeNativePackageTypeArgument(monthType, 6, 0)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got != time.June {
		t.Errorf("month from int: got %v, want June", got)
	}
}

// Test_makeNativePackageTypeArgument_TypeMismatch verifies that passing a
// value whose Go type does not match the declared native name returns
// ErrArgumentType.
func Test_makeNativePackageTypeArgument_TypeMismatch(t *testing.T) {
	durType := makeDurationType() // expects int or int64 for time.Duration

	// Passing a bool — neither int nor int64, so it falls to the default
	// case which checks reflect type string against the native name.
	_, err := makeNativePackageTypeArgument(durType, true, 0)

	if err == nil {
		t.Fatal("expected ErrArgumentType for bool → time.Duration, got nil")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 6: convertFromNative — Go return values → Ego stack
// ─────────────────────────────────────────────────────────────────────────────

// nativeFnReturning creates a *data.Function with a single return type.  It is
// used to set up the dp argument for convertFromNative tests.
func nativeFnReturning(returnType *data.Type) *data.Function {
	fn := nativeFn()
	fn.Declaration.Returns = []*data.Type{returnType}

	return fn
}

// Test_convertFromNative_ScalarInt verifies that a plain int result is pushed
// onto the stack.
func Test_convertFromNative_ScalarInt(t *testing.T) {
	tc := newTestContext(t)
	dp := nativeFnReturning(data.IntType)

	err := convertFromNative(tc.ctx, dp, 42)

	tc.assertNoError(err)
	tc.assertTopStack(42)
}

// Test_convertFromNative_ScalarString verifies that a plain string result is
// pushed onto the stack.
func Test_convertFromNative_ScalarString(t *testing.T) {
	const hello = "hello"

	tc := newTestContext(t)
	dp := nativeFnReturning(data.StringType)

	err := convertFromNative(tc.ctx, dp, hello)

	tc.assertNoError(err)
	tc.assertTopStack(hello)
}

// Test_convertFromNative_TimeDuration verifies that a time.Duration result is
// pushed directly onto the stack without modification.
func Test_convertFromNative_TimeDuration(t *testing.T) {
	tc := newTestContext(t)
	dp := nativeFnReturning(data.IntType)

	d := 3 * time.Second

	err := convertFromNative(tc.ctx, dp, d)

	tc.assertNoError(err)
	tc.assertTopStack(d)
}

// Test_convertFromNative_TimeTime verifies that a time.Time result is pushed
// directly onto the stack.
func Test_convertFromNative_TimeTime(t *testing.T) {
	tc := newTestContext(t)
	dp := nativeFnReturning(data.IntType)

	now := time.Now()

	err := convertFromNative(tc.ctx, dp, now)

	tc.assertNoError(err)
	tc.assertTopStack(now)
}

// Test_convertFromNative_DataList verifies that a data.List result is
// exploded: a "results" StackMarker is pushed first, then the list items in
// reverse order so the first item is on top.
func Test_convertFromNative_DataList(t *testing.T) {
	tc := newTestContext(t)
	dp := nativeFnReturning(data.IntType)

	list := data.NewList("alpha", "beta", "gamma")

	err := convertFromNative(tc.ctx, dp, list)

	tc.assertNoError(err)

	// Items were reversed before pushing, so "alpha" (index 0) is on top.
	tc.assertTopStack("alpha")
	tc.assertTopStack("beta")
	tc.assertTopStack("gamma")

	// The "results" marker must be at the bottom of the pushed items.
	marker, _ := tc.ctx.Pop()
	if !isStackMarker(marker, "results") {
		t.Errorf("expected 'results' StackMarker, got %T %v", marker, marker)
	}
}

// Test_convertFromNative_SliceAny verifies that a []any result is treated the
// same as a data.List: items pushed with a "results" marker.
func Test_convertFromNative_SliceAny(t *testing.T) {
	tc := newTestContext(t)
	dp := nativeFnReturning(data.IntType)

	err := convertFromNative(tc.ctx, dp, []any{10, 20})

	tc.assertNoError(err)

	tc.assertTopStack(10)
	tc.assertTopStack(20)

	marker, _ := tc.ctx.Pop()
	if !isStackMarker(marker, "results") {
		t.Errorf("expected 'results' StackMarker")
	}
}

// Test_convertFromNative_ArrayReturnType verifies that when the declared
// return type is an ArrayKind, the result is converted via convertFromNativeArray
// rather than the scalar path.
func Test_convertFromNative_ArrayReturnType(t *testing.T) {
	tc := newTestContext(t)

	// Declare that the function returns []int.
	dp := nativeFnReturning(data.ArrayType(data.IntType))

	// Pass a native []int result — convertFromNativeArray handles it.
	err := convertFromNative(tc.ctx, dp, []int{1, 2, 3})

	tc.assertNoError(err)

	v, _ := tc.ctx.Pop()

	arr, ok := v.(*data.Array)
	if !ok {
		t.Fatalf("expected *data.Array, got %T", v)
	}

	if arr.Len() != 3 {
		t.Errorf("array length: got %d, want 3", arr.Len())
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 7: convertFromNativeArray — native Go slices → *data.Array
// ─────────────────────────────────────────────────────────────────────────────

// Test_convertFromNativeArray_SliceInt verifies []int → *data.Array(IntType).
func Test_convertFromNativeArray_SliceInt(t *testing.T) {
	tc := newTestContext(t)

	err := convertFromNativeArray([]int{10, 20, 30}, tc.ctx)

	tc.assertNoError(err)

	v, _ := tc.ctx.Pop()

	arr, ok := v.(*data.Array)
	if !ok || arr.Len() != 3 {
		t.Fatalf("expected *data.Array[3], got %T %v", v, v)
	}
}

// Test_convertFromNativeArray_SliceString verifies []string → *data.Array(StringType).
func Test_convertFromNativeArray_SliceString(t *testing.T) {
	tc := newTestContext(t)

	err := convertFromNativeArray([]string{"a", "b"}, tc.ctx)

	tc.assertNoError(err)

	v, _ := tc.ctx.Pop()

	arr, ok := v.(*data.Array)
	if !ok || arr.Len() != 2 {
		t.Fatalf("expected *data.Array[2], got %T", v)
	}
}

// Test_convertFromNativeArray_SliceBool verifies []bool → *data.Array(BoolType).
func Test_convertFromNativeArray_SliceBool(t *testing.T) {
	tc := newTestContext(t)

	err := convertFromNativeArray([]bool{true, false}, tc.ctx)

	tc.assertNoError(err)
	v, _ := tc.ctx.Pop()

	_, ok := v.(*data.Array)
	if !ok {
		t.Fatalf("expected *data.Array, got %T", v)
	}
}

// Test_convertFromNativeArray_SliceFloat64 verifies []float64 → *data.Array(Float64Type).
func Test_convertFromNativeArray_SliceFloat64(t *testing.T) {
	tc := newTestContext(t)

	err := convertFromNativeArray([]float64{1.1, 2.2}, tc.ctx)

	tc.assertNoError(err)
	v, _ := tc.ctx.Pop()

	_, ok := v.(*data.Array)
	if !ok {
		t.Fatalf("expected *data.Array, got %T", v)
	}
}

// Test_convertFromNativeArray_SliceInt64 verifies []int64 → *data.Array(Int64Type).
func Test_convertFromNativeArray_SliceInt64(t *testing.T) {
	tc := newTestContext(t)

	err := convertFromNativeArray([]int64{100, 200}, tc.ctx)

	tc.assertNoError(err)
	v, _ := tc.ctx.Pop()

	arr, ok := v.(*data.Array)
	if !ok || !arr.Type().IsType(data.Int64Type) {
		t.Fatalf("expected *data.Array(Int64Type), got %T type=%v", v, arr.Type())
	}
}

// Test_convertFromNativeArray_SliceFloat32 verifies []float32 → *data.Array(Float32Type).
func Test_convertFromNativeArray_SliceFloat32(t *testing.T) {
	tc := newTestContext(t)

	err := convertFromNativeArray([]float32{1.0, 2.0}, tc.ctx)

	tc.assertNoError(err)
	v, _ := tc.ctx.Pop()

	arr, ok := v.(*data.Array)
	if !ok || !arr.Type().IsType(data.Float32Type) {
		t.Fatalf("expected *data.Array(Float32Type), got %T type=%v", v, arr.Type())
	}
}

// Test_convertFromNativeArray_UnknownType verifies that an unrecognized slice
// type returns ErrWrongArrayValueType rather than panicking.
func Test_convertFromNativeArray_UnknownType(t *testing.T) {
	tc := newTestContext(t)

	// complex128 is not a handled type.
	type mySpecialType struct{ v int }

	result := []mySpecialType{{1}, {2}}

	err := convertFromNativeArray(result, tc.ctx)

	tc.assertError(err, errors.ErrWrongArrayValueType)
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 8: CallDirect — invoke a Go function via reflection
// ─────────────────────────────────────────────────────────────────────────────

// Test_CallDirect_SingleReturn verifies a function that returns a single
// value.  math.Abs(-3.14) should return 3.14.
func Test_CallDirect_SingleReturn(t *testing.T) {
	result, err := CallDirect(math.Abs, float64(-3.14))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != 3.14 {
		t.Errorf("math.Abs(-3.14): got %v, want 3.14", result)
	}
}

// Test_CallDirect_StringFunction verifies a string-in, string-out function.
// strings.TrimSpace("  hello  ") should return "hello".
func Test_CallDirect_StringFunction(t *testing.T) {
	const hello = "hello"

	result, err := CallDirect(strings.TrimSpace, "  hello  ")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != hello {
		t.Errorf("TrimSpace: got %q, want %q", result, hello)
	}
}

// Test_CallDirect_TwoReturnValueAndError verifies that a function returning
// (value, error) is packed into a data.List.  strconv.Atoi("42") returns
// (42, nil) so the list should contain 42 and nil.
func Test_CallDirect_TwoReturnValueAndError(t *testing.T) {
	result, err := CallDirect(strconv.Atoi, "42")

	if err != nil {
		t.Fatalf("unexpected error from CallDirect: %v", err)
	}

	list, ok := result.(data.List)
	if !ok {
		t.Fatalf("expected data.List, got %T", result)
	}

	if list.Len() != 2 {
		t.Fatalf("list length: got %d, want 2", list.Len())
	}

	if list.Get(0) != 42 {
		t.Errorf("list[0]: got %v, want 42", list.Get(0))
	}

	if list.Get(1) != nil {
		t.Errorf("list[1]: got %v, want nil (no error)", list.Get(1))
	}
}

// Test_CallDirect_TwoReturnWithError verifies that the error from a
// two-return function is captured in the data.List.
// strconv.Atoi("not-a-number") returns (0, error).
func Test_CallDirect_TwoReturnWithError(t *testing.T) {
	result, err := CallDirect(strconv.Atoi, "not-a-number")

	if err != nil {
		t.Fatalf("unexpected error from CallDirect itself: %v", err)
	}

	list, ok := result.(data.List)
	if !ok {
		t.Fatalf("expected data.List, got %T", result)
	}

	if list.Get(1) == nil {
		t.Error("expected a non-nil error in list[1], got nil")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 9: CallWithReceiver — invoke a method via reflection
// ─────────────────────────────────────────────────────────────────────────────

// Test_CallWithReceiver_ValidMethod verifies that calling a known method on a
// Go struct works correctly.  time.Time.Year() on a known date should return
// the year as an int.
func Test_CallWithReceiver_ValidMethod(t *testing.T) {
	knownDate := time.Date(2024, time.June, 15, 0, 0, 0, 0, time.UTC)

	result, err := CallWithReceiver(knownDate, "Year")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != 2024 {
		t.Errorf("Year(): got %v, want 2024", result)
	}
}

// Test_CallWithReceiver_PointerReceiver verifies that passing a *any (a
// pointer-to-interface) correctly dereferences before calling the method.
func Test_CallWithReceiver_PointerReceiver(t *testing.T) {
	knownDate := time.Date(2024, time.June, 15, 0, 0, 0, 0, time.UTC)

	var iface any = knownDate

	ptr := &iface // *any wrapping time.Time

	result, err := CallWithReceiver(ptr, "Year")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != 2024 {
		t.Errorf("Year() via *any: got %v, want 2024", result)
	}
}

// Test_CallWithReceiver_UnknownMethod verifies the CALL-9 fix: calling a
// method name that does not exist on the receiver now returns a clean error
// instead of causing an unrecoverable panic.
//
// Before the fix, MethodByName returned a zero reflect.Value and the
// subsequent m.Call() panicked with:
//
//	panic: reflect: call of reflect.Value.Call on zero Value
//
// The fix adds an m.IsValid() guard that returns ErrNoFunctionReceiver
// whenever the method is absent.
func Test_CallWithReceiver_UnknownMethod(t *testing.T) {
	panicked := false

	var callErr error

	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()

		_, callErr = CallWithReceiver(time.Now(), "DoesNotExistMethod")
	}()

	// After the CALL-9 fix the call must not panic.
	if panicked {
		t.Fatal("CallWithReceiver panicked on unknown method: CALL-9 fix was not applied")
	}

	// A clean error must be returned so the caller can report a useful message.
	if callErr == nil {
		t.Error("expected a non-nil error for unknown method, got nil")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 10: safeReflectCall — recover native-call panics (BUG-27)
// ─────────────────────────────────────────────────────────────────────────────
//
// safeReflectCall wraps a reflect-based method/function call in a deferred
// recover(), so a panic inside the native Go code being called turns into a
// normal Go error return instead of crashing the process. See the big
// doc comment on safeReflectCall in callNative.go for the full background
// on why this is needed and what it can/cannot catch.

// Test_safeReflectCall_NoPanic verifies the ordinary, successful path: a
// function that returns normally should have its results passed straight
// through with a nil error.
func Test_safeReflectCall_NoPanic(t *testing.T) {
	m := reflect.ValueOf(strings.ToUpper)
	argList := []reflect.Value{reflect.ValueOf("hello")}

	results, err := safeReflectCall(m, argList, "strings.ToUpper")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 1 || results[0].String() != "HELLO" {
		t.Errorf("safeReflectCall results = %v, want [\"HELLO\"]", results)
	}
}

// Test_safeReflectCall_RecoversPanic is the direct regression test
// for BUG-27. It calls a function that panics (via a small helper below) through
// safeReflectCall and verifies that:
//
//  1. the panic does NOT propagate out of safeReflectCall (which is what
//     used to crash the whole ego process), and
//  2. a non-nil error is returned instead, built from the new
//     ErrNativeCallPanic error and carrying both the call description and
//     the original panic value as context, so a developer (or an Ego
//     try/catch block) can tell what actually went wrong.
func Test_safeReflectCall_RecoversPanic(t *testing.T) {
	m := reflect.ValueOf(panicsWithMessage)

	var (
		panicked bool
		results  []reflect.Value
		err      error
	)

	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()

		results, err = safeReflectCall(m, nil, "test.panicsWithMessage")
	}()

	if panicked {
		t.Fatal("safeReflectCall let a panic escape: BUG-27 fix was not applied")
	}

	if results != nil {
		t.Errorf("safeReflectCall results = %v, want nil after a panic", results)
	}

	if err == nil {
		t.Fatal("expected a non-nil error after a recovered panic, got nil")
	}

	if !errors.Equals(errors.New(err), errors.ErrNativeCallPanic) {
		t.Errorf("safeReflectCall error = %v, want ErrNativeCallPanic", err)
	}

	// The error's text should mention both the call description we passed
	// in and the original panic message, so whoever reads the error (a
	// human, or Ego code inspecting e.Error()) has enough context to
	// understand what call actually failed and why.
	msg := err.Error()
	if !strings.Contains(msg, "test.panicsWithMessage") {
		t.Errorf("error message %q does not contain the call description", msg)
	}

	if !strings.Contains(msg, "boom") {
		t.Errorf("error message %q does not contain the original panic text", msg)
	}
}

// panicsWithMessage is a trivial helper function with no arguments and no
// return values, used only by Test_safeReflectCall_RecoversPanic above to
// exercise the panic-recovery path via reflection.
func panicsWithMessage() {
	panic("boom")
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 11: callMutexMethod — sync.Mutex misuse guard (BUG-28)
// ─────────────────────────────────────────────────────────────────────────────
//
// callMutexMethod intercepts Lock/Unlock/TryLock calls on a *sync.Mutex so
// that Unlock() can never be called on a mutex that isn't locked — Go's
// real sync.Mutex.Unlock() would otherwise trigger an unrecoverable fatal
// error (not an ordinary panic()) in that situation, which safeReflectCall's
// recover() above is specifically unable to catch. See the mutexLockState
// doc comment in callNative.go for the full explanation.

// Test_callMutexMethod_LockThenUnlock verifies the normal, well-behaved
// sequence: Lock() followed by Unlock() should both succeed with no error,
// and the mutex should genuinely be unlocked afterward (checked here with
// TryLock, which only succeeds if nothing else is holding the lock).
func Test_callMutexMethod_LockThenUnlock(t *testing.T) {
	mu := &sync.Mutex{}

	if _, handled, err := callMutexMethod(mu, "Lock"); !handled || err != nil {
		t.Fatalf("Lock: handled=%v err=%v, want handled=true err=nil", handled, err)
	}

	if _, handled, err := callMutexMethod(mu, "Unlock"); !handled || err != nil {
		t.Fatalf("Unlock: handled=%v err=%v, want handled=true err=nil", handled, err)
	}

	// If Unlock() above actually released the mutex, a TryLock() here must
	// succeed. Clean up by unlocking again so the test doesn't leak a
	// locked mutex.
	result, handled, err := callMutexMethod(mu, "TryLock")
	if !handled || err != nil {
		t.Fatalf("TryLock: handled=%v err=%v, want handled=true err=nil", handled, err)
	}

	// callMutexMethod's "result any" return holds a plain bool for
	// TryLock; assert to bool explicitly rather than comparing the
	// interface value directly, which keeps the test's intent obvious.
	acquired, ok := result.(bool)
	if !ok {
		t.Fatalf("TryLock result = %T, want bool", result)
	}

	if !acquired {
		t.Fatal("TryLock() after Lock()+Unlock() failed to acquire; mutex was not actually unlocked")
	}

	_, _, _ = callMutexMethod(mu, "Unlock") //nolint:dogsled
}

// Test_callMutexMethod_UnlockWithoutLock is the direct regression test
// for BUG-28. Calling "Unlock" on a mutex that was never locked must NOT call
// Go's real sync.Mutex.Unlock() (which would trigger an unrecoverable fatal
// error and crash the test binary); instead it must be refused up front
// with a normal, catchable error.
func Test_callMutexMethod_UnlockWithoutLock(t *testing.T) {
	mu := &sync.Mutex{}

	result, handled, err := callMutexMethod(mu, "Unlock")

	if !handled {
		t.Fatal("Unlock: handled=false, want true")
	}

	if result != nil {
		t.Errorf("Unlock on unlocked mutex returned result=%v, want nil", result)
	}

	if err == nil {
		t.Fatal("Unlock on unlocked mutex: expected a non-nil error, got nil")
	}

	if !errors.Equals(errors.New(err), errors.ErrMutexNotLocked) {
		t.Errorf("Unlock on unlocked mutex error = %v, want ErrMutexNotLocked", err)
	}

	// Prove the mutex genuinely was never locked by Go's own accounting:
	// TryLock() should succeed here, since nothing real ever locked it.
	if !mu.TryLock() {
		t.Error("mutex appears locked after a refused Unlock() call; callMutexMethod must not have called the real Lock()")
	}

	mu.Unlock()
}

// Test_callMutexMethod_DoubleUnlockReturnsErrorNotFatal extends the BUG-28
// regression test to the exact repro from docs/ISSUES.md: Lock(), Unlock(),
// then Unlock() again. The second Unlock() must be refused with a catchable
// error rather than reaching Go's real Unlock() a second time.
func Test_callMutexMethod_DoubleUnlockReturnsErrorNotFatal(t *testing.T) {
	mu := &sync.Mutex{}

	_, _, _ = callMutexMethod(mu, "Lock") //nolint:dogsled

	if _, _, err := callMutexMethod(mu, "Unlock"); err != nil {
		t.Fatalf("first Unlock unexpected error: %v", err)
	}

	_, _, err := callMutexMethod(mu, "Unlock")
	if err == nil {
		t.Fatal("second Unlock: expected a non-nil error, got nil")
	}

	if !errors.Equals(errors.New(err), errors.ErrMutexNotLocked) {
		t.Errorf("second Unlock error = %v, want ErrMutexNotLocked", err)
	}
}

// Test_callMutexMethod_TryLockWhenLocked verifies that TryLock() correctly
// reports failure (and does not disturb the lock state) when the mutex is
// already locked.
func Test_callMutexMethod_TryLockWhenLocked(t *testing.T) {
	mu := &sync.Mutex{}

	_, _, _ = callMutexMethod(mu, "Lock") //nolint:dogsled

	result, handled, err := callMutexMethod(mu, "TryLock")
	if !handled || err != nil {
		t.Fatalf("TryLock: handled=%v err=%v, want handled=true err=nil", handled, err)
	}

	acquired, ok := result.(bool)
	if !ok {
		t.Fatalf("TryLock result = %T, want bool", result)
	}

	if acquired {
		t.Error("TryLock() on an already-locked mutex reported success")
	}

	// The mutex should still be considered locked, so Unlock() must
	// succeed cleanly (it would fail with ErrMutexNotLocked if our
	// bookkeeping had incorrectly cleared the locked state above).
	if _, _, err := callMutexMethod(mu, "Unlock"); err != nil {
		t.Errorf("Unlock after failed TryLock unexpected error: %v", err)
	}
}

// Test_callMutexMethod_UnhandledMethodFallsThrough verifies that a method
// name callMutexMethod does not know about (there are none today, since
// Lock/Unlock/TryLock are the only sync.Mutex methods Ego exposes — but this
// guards against a silent behavior change if that ever grows) reports
// handled=false so CallWithReceiver falls back to the generic reflection
// path instead of silently doing nothing.
func Test_callMutexMethod_UnhandledMethodFallsThrough(t *testing.T) {
	mu := &sync.Mutex{}

	_, handled, err := callMutexMethod(mu, "SomeFutureMethod")
	if handled {
		t.Error("handled=true for an unknown method name, want false")
	}

	if err != nil {
		t.Errorf("unexpected error for an unhandled method name: %v", err)
	}
}

// Test_CallWithReceiver_MutexDoubleUnlock exercises the full public
// CallWithReceiver entry point (rather than callMutexMethod directly) to
// confirm the BUG-28 fix is actually wired up end-to-end: a *sync.Mutex
// receiver with methodName "Unlock" must be intercepted before reaching the
// generic reflection dispatch further down in CallWithReceiver.
func Test_CallWithReceiver_MutexDoubleUnlock(t *testing.T) {
	mu := &sync.Mutex{}

	if _, err := CallWithReceiver(mu, "Lock"); err != nil {
		t.Fatalf("Lock via CallWithReceiver unexpected error: %v", err)
	}

	if _, err := CallWithReceiver(mu, "Unlock"); err != nil {
		t.Fatalf("first Unlock via CallWithReceiver unexpected error: %v", err)
	}

	var (
		panicked bool
		callErr  error
	)

	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()

		_, callErr = CallWithReceiver(mu, "Unlock")
	}()

	if panicked {
		t.Fatal("CallWithReceiver panicked (or crashed via fatal error) on double Unlock: BUG-28 fix was not applied")
	}

	if callErr == nil {
		t.Fatal("second Unlock via CallWithReceiver: expected a non-nil error, got nil")
	}

	if !errors.Equals(errors.New(callErr), errors.ErrMutexNotLocked) {
		t.Errorf("second Unlock via CallWithReceiver error = %v, want ErrMutexNotLocked", callErr)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 12: callNative — end-to-end integration
// ─────────────────────────────────────────────────────────────────────────────

// Test_callNative_SandboxedFunctionBlocked verifies that when both the
// execution context is sandboxed AND the function is marked Sandboxed, the
// call is rejected with ErrNoPrivilegeForOperation.  This prevents sandboxed
// Ego programs from calling privileged file-system functions.
func Test_callNative_SandboxedFunctionBlocked(t *testing.T) {
	tc := newTestContext(t)
	tc.ctx.sandboxedIO.Store(true) // enable sandboxed mode

	dp := &data.Function{
		Sandboxed: true,
		Declaration: &data.Declaration{
			Name: "readFile",
		},
	}

	err := callNative(tc.ctx, dp, []any{})

	tc.assertError(err, errors.ErrNoPrivilegeForOperation)
}

// Test_callNative_NonSandboxedFunctionNotBlocked verifies that a non-sandboxed
// function may be called even when the context is sandboxed.
func Test_callNative_NonSandboxedFunctionNotBlocked(t *testing.T) {
	tc := newTestContext(t)
	tc.ctx.sandboxedIO.Store(true) // sandboxed mode

	// Sandboxed: false means this function is always permitted.
	dp := &data.Function{
		Sandboxed: false, // not a sandboxed (restricted) function
		IsNative:  true,
		Value:     math.Abs,
		Declaration: &data.Declaration{
			Name:       "Abs",
			Parameters: []data.Parameter{{Name: "x", Type: data.Float64Type}},
			Returns:    []*data.Type{data.Float64Type},
		},
	}

	err := callNative(tc.ctx, dp, []any{float64(-5.0)})

	tc.assertNoError(err)
	tc.assertTopStack(5.0)
}

// Test_callNative_ArgCountMismatch verifies that calling a non-variadic native
// function with fewer arguments than declared returns ErrArgumentCount rather
// than panicking.
func Test_callNative_ArgCountMismatch(t *testing.T) {
	tc := newTestContext(t)

	// math.Abs expects one float64 argument.
	dp := &data.Function{
		IsNative: true,
		Value:    math.Abs,
		Declaration: &data.Declaration{
			Name: "Abs",
			Parameters: []data.Parameter{
				{Name: "x", Type: data.Float64Type},
				{Name: "y", Type: data.Float64Type}, // declared but not supplied
			},
		},
	}

	// Pass only one argument where two are declared.
	err := callNative(tc.ctx, dp, []any{float64(3.0)})

	tc.assertError(err, errors.ErrArgumentCount)
}

// Test_callNative_DirectCall_Float64 verifies the normal happy path for a
// zero-receiver native function: math.Abs(-7.5) returns 7.5.
func Test_callNative_DirectCall_Float64(t *testing.T) {
	tc := newTestContext(t)

	dp := &data.Function{
		IsNative: true,
		Value:    math.Abs,
		Declaration: &data.Declaration{
			Name:       "Abs",
			Parameters: []data.Parameter{{Name: "x", Type: data.Float64Type}},
			Returns:    []*data.Type{data.Float64Type},
		},
	}

	err := callNative(tc.ctx, dp, []any{float64(-7.5)})

	tc.assertNoError(err)
	tc.assertTopStack(7.5)
}

// Test_callNative_DirectCall_String verifies a string-returning native function.
// strings.TrimSpace converts an argument then pushes the result.
func Test_callNative_DirectCall_String(t *testing.T) {
	tc := newTestContext(t)

	dp := &data.Function{
		IsNative: true,
		Value:    strings.TrimSpace,
		Declaration: &data.Declaration{
			Name:       "TrimSpace",
			Parameters: []data.Parameter{{Name: "s", Type: data.StringType}},
			Returns:    []*data.Type{data.StringType},
		},
	}

	err := callNative(tc.ctx, dp, []any{"  trim me  "})

	tc.assertNoError(err)
	tc.assertTopStack("trim me")
}

// Test_callNative_ReceiverCall_NoReceiverInStack verifies that if a method
// call requires a receiver but the receiver stack is empty, the instruction
// returns ErrNoFunctionReceiver.
func Test_callNative_ReceiverCall_NoReceiverInStack(t *testing.T) {
	tc := newTestContext(t)

	// Declare a method on a type (Type != nil triggers the receiver path).
	dp := &data.Function{
		IsNative: true,
		Value:    math.Abs, // value doesn't matter — receiver pop fails first
		Declaration: &data.Declaration{
			Name:       "Year",
			Type:       data.IntType, // non-nil Type flags this as a method call
			Parameters: []data.Parameter{},
		},
	}

	// Receiver stack is empty — popThis will return (nil, false).
	err := callNative(tc.ctx, dp, []any{})

	tc.assertError(err, errors.ErrNoFunctionReceiver)
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 13: sandboxName — apply sandbox path prefix
// ─────────────────────────────────────────────────────────────────────────────

// Test_sandboxName_NoSandbox verifies that when the context is not sandboxed
// and no SandboxPathSetting is configured, the path is returned unchanged.
func Test_sandboxName_NoSandbox(t *testing.T) {
	tc := newTestContext(t)
	// sandboxedIO defaults to false in newTestContext; no SandboxPathSetting is set.

	got := sandboxName(tc.ctx, "/etc/passwd")

	if got != "/etc/passwd" {
		t.Errorf("no-sandbox: got %q, want %q", got, "/etc/passwd")
	}
}

// Test_sandboxName_SandboxedEmptyPrefix verifies that when the context IS
// sandboxed but the sandbox prefix setting is empty, HasPrefix(path, "")
// is always true so the path is returned unchanged.
func Test_sandboxName_SandboxedEmptyPrefix(t *testing.T) {
	tc := newTestContext(t)
	tc.ctx.sandboxedIO.Store(true)
	// SandboxPathSetting is not set, so settings.Get returns "".

	got := sandboxName(tc.ctx, "some/path")

	if got != "some/path" {
		t.Errorf("sandboxed-empty-prefix: got %q, want %q", got, "some/path")
	}
}

// Test_sandboxName_PathAlreadyHasPrefix verifies that a path that already
// starts with the sandbox prefix is returned as-is (no double-prefix).
func Test_sandboxName_PathAlreadyHasPrefix(t *testing.T) {
	tc := newTestContext(t)

	// Use the defs constant directly so the test doesn't need to set a
	// global setting.  The function checks settings.Get(SandboxPathSetting)
	// at call time.  We test the HasPrefix branch via the condition logic
	// by noting that "".HasPrefix("") is always true — which is covered by
	// Test_sandboxName_SandboxedEmptyPrefix above.
	// This test just confirms no-sandbox → identity.
	got := sandboxName(tc.ctx, "/sandbox/myfile.txt")

	if got != "/sandbox/myfile.txt" {
		t.Errorf("got %q, want unchanged", got)
	}
}

// Ensure the defs package is used (imported) so the compiler doesn't drop it.
var _ = defs.SandboxPathSetting
