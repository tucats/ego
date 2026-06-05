package bytecode

// create_test.go tests the bytecode instructions in create.go:
//
//   makeArrayByteCode      — MakeArray opcode (make() or typed array literals)
//   arrayByteCode          — Array opcode (anonymous array constants)
//   structByteCode         — Struct opcode (struct literals)
//   makeMapByteCode        — MakeMap opcode (map literals)
//
// Helper functions also tested here:
//   coerceConstantArrayInitializer — type-coercion helper for makeArrayByteCode
//   reverse                        — slice-reversal utility used by structByteCode
//
// The file keeps the original table-driven tests unchanged at the top, then
// adds flat-style (one-function-per-case) tests below to fill coverage gaps.
//
// Bugs documented in this file:
//
//   CREATE-1  makeArrayByteCode called result.Set twice per element (fixed).
//   CREATE-2  addMissingFields had an inverted error check that prevented
//             coerced field values from being written back to structMap (fixed).
//   CREATE-3  makeArrayByteCode's element-pop loop swallowed Pop errors silently;
//             a stack underflow during element collection produced a zeroed
//             element instead of ErrStackUnderflow (fixed).

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// ─────────────────────────────────────────────────────────────────────────────
// Original table-driven tests (preserved; only the string-array `want` value
// has been corrected — it previously copied the int-array expected value, so
// the output was never actually validated for that case).
// ─────────────────────────────────────────────────────────────────────────────

func Test_makeArrayByteCode(t *testing.T) {
	type args struct {
		stack []any
		i     int
	}

	tests := []struct {
		name string
		args args
		want *data.Array
	}{
		{
			name: "[]int{5,3}",
			args: args{
				stack: []any{3, 5, data.IntType},
				i:     2,
			},
			want: data.NewArrayFromList(data.IntType, data.NewList(3, 5)),
		},
		{
			// The original test had the wrong want value here (it copied the
			// int-array expected value).  Corrected to the actual string array.
			name: "[]string{\"Tom\", \"Cole\"}",
			args: args{
				stack: []any{"Cole", "Tom", data.StringType},
				i:     2,
			},
			want: data.NewArrayFromList(data.StringType, data.NewList("Tom", "Cole")),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &Context{stack: tt.args.stack, stackPointer: len(tt.args.stack)}

			e := makeArrayByteCode(ctx, tt.args.i)
			if e != nil {
				t.Errorf("Unexpected error %v", e)
			}
		})
	}
}

func Test_arrayByteCode(t *testing.T) {
	target := arrayByteCode
	name := "arrayByteCode"

	tests := []struct {
		name   string
		arg    any
		stack  []any
		want   any
		err    error
		static int
		debug  bool
	}{
		{
			name:  "untyped array",
			arg:   2,
			stack: []any{3, "test", float64(3.5)},
			err:   nil,
			want:  data.NewArrayFromList(data.InterfaceType, data.NewList("test", float64(3.5))),
		},
		{
			name:   "typed array",
			arg:    []any{3, data.Int32Type},
			stack:  []any{byte(3), "55", float64(3.5)},
			err:    nil,
			static: 2,
			want:   data.NewArrayFromList(data.Int32Type, data.NewList(int32(3), int32(55), int32(3))),
		},
		{
			name:   "untyped static (valid) array",
			arg:    3,
			stack:  []any{int32(10), int32(11), int32(12)},
			static: 0,
			want:   data.NewArrayFromList(data.InterfaceType, data.NewList(int32(10), int32(11), int32(12))),
		},
		{
			name:   "stack underflow",
			arg:    3,
			stack:  []any{"test", float64(3.5)},
			err:    errors.ErrStackUnderflow,
			static: 2,
			want:   data.NewArrayFromList(data.InterfaceType, data.NewList("test", float64(3.5))),
		},
	}

	for _, tt := range tests {
		syms := symbols.NewSymbolTable("testing")
		bc := ByteCode{}

		c := NewContext(syms, &bc)
		c.typeStrictness = tt.static

		for _, item := range tt.stack {
			_ = c.push(item)
		}

		t.Run(tt.name, func(t *testing.T) {
			if tt.debug {
				fmt.Println("DEBUG")
			}

			err := target(c, tt.arg)

			if err != nil {
				e1 := nilError

				if tt.err != nil {
					e1 = tt.err.Error()
				}

				e2 := err.Error()

				if e1 == e2 {
					return
				}

				t.Errorf("%s() error %v", name, err)
			} else if tt.err != nil {
				t.Errorf("%s() expected error not reported: %v", name, tt.err)
			}

			v, err := c.Pop()

			if err != nil {
				t.Errorf("%s() stack error %v", name, err)
			}

			if !reflect.DeepEqual(v, tt.want) {
				t.Errorf("%s() got %v, want %v", name, v, tt.want)
			}
		})
	}
}

func Test_makeMapByteCode(t *testing.T) {
	target := makeMapByteCode
	name := "makeMapByteCode"

	tests := []struct {
		name   string
		arg    any
		stack  []any
		want   any
		err    error
		static int
		debug  bool
	}{
		{
			name: "map[string]int",
			arg:  4,
			stack: []any{
				"tom", 63, // Key/value pair
				"mary", 47, // Key/value pair
				"chelsea", 10, // Key/value pair
				"sarah", 31, // Key/value pair
				data.IntType,    // Value type
				data.StringType, // Key type
			},
			static: 2,
			err:    nil,
			want:   data.NewMapFromMap(map[string]int{"tom": 63, "mary": 47, "chelsea": 10, "sarah": 31}),
		},
		{
			name:   "Missing key type",
			arg:    4,
			static: 2,
			stack:  []any{},
			err:    errors.ErrStackUnderflow,
			want:   data.NewMapFromMap(map[string]int{"tom": 63, "mary": 47, "chelsea": 10, "sarah": 31}),
		},
		{
			name:   "Missing value type",
			arg:    4,
			static: 2,
			stack: []any{
				data.StringType, // Key type
			},
			err:  errors.ErrStackUnderflow,
			want: data.NewMapFromMap(map[string]int{"tom": 63, "mary": 47, "chelsea": 10, "sarah": 31}),
		},
		{
			name:   "Missing key",
			arg:    4,
			static: 2,
			stack: []any{
				"mary", 47, // Key/value pair
				"chelsea", 10, // Key/value pair
				"sarah", 31, // Key/value pair
				data.IntType,    // Value type
				data.StringType, // Key type
			},
			err:  errors.ErrStackUnderflow,
			want: data.NewMapFromMap(map[string]int{"tom": 63, "mary": 47, "chelsea": 10, "sarah": 31}),
		},
		{
			name:   "missing value",
			arg:    4,
			static: 2,
			stack: []any{
				"tom",         // key without its value
				"mary", 47,    // Key/value pair
				"chelsea", 10, // Key/value pair
				"sarah", 31,   // Key/value pair
				data.IntType,    // Value type
				data.StringType, // Key type
			},
			err:  errors.ErrStackUnderflow,
			want: data.NewMapFromMap(map[string]int{"tom": 63, "mary": 47, "chelsea": 10, "sarah": 31}),
		},
	}

	for _, tt := range tests {
		syms := symbols.NewSymbolTable("testing")
		bc := ByteCode{}

		c := NewContext(syms, &bc)
		c.typeStrictness = tt.static

		for _, item := range tt.stack {
			_ = c.push(item)
		}

		t.Run(tt.name, func(t *testing.T) {
			if tt.debug {
				fmt.Println("DEBUG")
			}

			err := target(c, tt.arg)

			if err != nil {
				e1 := nilError
				e2 := err.Error()

				if tt.err != nil {
					e1 = tt.err.Error()
				}

				if e1 == e2 {
					return
				}

				t.Errorf("%s() error %v", name, err)
			} else if tt.err != nil {
				t.Errorf("%s() expected error not reported: %v", name, tt.err)
			}

			v, err := c.Pop()

			if err != nil {
				t.Errorf("%s() stack error %v", name, err)
			}

			if !reflect.DeepEqual(v, tt.want) {
				t.Errorf("%s() got %v, want %v", name, v, tt.want)
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Flat-style tests for makeArrayByteCode
//
// Stack layout for makeArrayByteCode (top → bottom):
//
//	top:    baseType  (*data.Type)
//	...     element[count-1]   (last element pushed, popped first)
//	...     element[1]
//	bottom: element[0]
//
// The operand is the count of elements (not counting the type pop).
// withStack pushes arguments left-to-right so the rightmost arg ends up on top:
//
//	withStack(elem0, elem1, ..., elemN-1, baseType)   operand=N
//
// Each test covers one distinct scenario and is independently runnable via
// -run Test_makeArrayByteCode_<name>.
// ─────────────────────────────────────────────────────────────────────────────

// Test_makeArrayByteCode_IntArray verifies that a two-element int array is
// assembled correctly and leaves exactly one item (the array) on the stack.
func Test_makeArrayByteCode_IntArray(t *testing.T) {
	// Push: elem[0]=3 (bottom), elem[1]=5, baseType=IntType (top).
	// makeArrayByteCode pops IntType first (base type), then 5, then 3.
	// Indices are filled as result[count-i-1]: result[1]=5, result[0]=3.
	tc := newTestContext(t).withStack(3, 5, data.IntType)

	tc.assertNoError(makeArrayByteCode(tc.ctx, 2))

	want := data.NewArrayFromList(data.IntType, data.NewList(3, 5))
	tc.assertTopStack(want)
	tc.assertStackEmpty()
}

// Test_makeArrayByteCode_StringArray verifies that a two-element string array
// is assembled correctly and holds the right string values in order.
func Test_makeArrayByteCode_StringArray(t *testing.T) {
	tc := newTestContext(t).withStack("Tom", "Cole", data.StringType)

	tc.assertNoError(makeArrayByteCode(tc.ctx, 2))

	want := data.NewArrayFromList(data.StringType, data.NewList("Tom", "Cole"))
	tc.assertTopStack(want)
	tc.assertStackEmpty()
}

// Test_makeArrayByteCode_Float64Array verifies three-element float64 assembly
// with exact value preservation.
func Test_makeArrayByteCode_Float64Array(t *testing.T) {
	tc := newTestContext(t).withStack(1.5, 2.5, 3.5, data.Float64Type)

	tc.assertNoError(makeArrayByteCode(tc.ctx, 3))

	want := data.NewArrayFromList(data.Float64Type, data.NewList(1.5, 2.5, 3.5))
	tc.assertTopStack(want)
	tc.assertStackEmpty()
}

// Test_makeArrayByteCode_EmptyArray verifies that count=0 with only a type on
// the stack produces a valid zero-length array of the correct element type.
func Test_makeArrayByteCode_EmptyArray(t *testing.T) {
	// Only the base type is on the stack; no element values are needed.
	tc := newTestContext(t).withStack(data.IntType)

	tc.assertNoError(makeArrayByteCode(tc.ctx, 0))

	want := data.NewArray(data.IntType, 0)
	tc.assertTopStack(want)
	tc.assertStackEmpty()
}

// Test_makeArrayByteCode_StackUnderflow_TypePop verifies that an empty stack
// returns ErrStackUnderflow immediately — the first Pop (for the base type)
// fails and the error is propagated via the else-branch of the type-pop block.
func Test_makeArrayByteCode_StackUnderflow_TypePop(t *testing.T) {
	tc := newTestContext(t) // completely empty stack

	tc.assertError(makeArrayByteCode(tc.ctx, 2), errors.ErrStackUnderflow)
}

// Test_makeArrayByteCode_StackMarkerAsType verifies that a StackMarker in the
// type position (top of stack when the base type is popped) causes
// ErrFunctionReturnedVoid.  This happens when the expression that was supposed
// to supply the type was a void call.
func Test_makeArrayByteCode_StackMarkerAsType(t *testing.T) {
	tc := newTestContext(t).withStack(NewStackMarker("void-result"))

	tc.assertError(makeArrayByteCode(tc.ctx, 1), errors.ErrFunctionReturnedVoid)
}

// Test_makeArrayByteCode_StackMarkerAsElement verifies that a StackMarker in
// the element position causes ErrFunctionReturnedVoid.  The marker IS caught
// because Pop() returns it successfully (it is a real stack value, not an
// error), and coerceConstantArrayInitializer then detects it with isStackMarker.
// This is the correct, non-silent path.
func Test_makeArrayByteCode_StackMarkerAsElement(t *testing.T) {
	// Stack: StackMarker (bottom, element slot), IntType (top, type slot).
	tc := newTestContext(t).withStack(NewStackMarker("bad"), data.IntType)

	tc.assertError(makeArrayByteCode(tc.ctx, 1), errors.ErrFunctionReturnedVoid)
}

// Test_makeArrayByteCode_ElementStackUnderflow verifies that a stack underflow
// during element collection returns ErrStackUnderflow (CREATE-3 fix).
//
// Before the fix the loop used `if value, err := c.Pop(); err == nil { ... }`,
// which swallowed Pop errors silently and produced a zero-filled array with no
// indication that something had gone wrong.
func Test_makeArrayByteCode_ElementStackUnderflow(t *testing.T) {
	// Only the base type is on the stack.  count=1 requires one element pop, but
	// after consuming the type the stack is empty — the element pop must fail.
	tc := newTestContext(t).withStack(data.IntType)

	tc.assertError(makeArrayByteCode(tc.ctx, 1), errors.ErrStackUnderflow)
}

// Test_makeArrayByteCode_InvalidOperand verifies that a non-integer operand
// (cannot be converted to int for the element count) causes a runtime error
// before any stack operations are attempted.
func Test_makeArrayByteCode_InvalidOperand(t *testing.T) {
	// Push a type so the operand error fires first, not a stack underflow.
	tc := newTestContext(t).withStack(data.IntType)

	err := makeArrayByteCode(tc.ctx, "not-a-number")
	if err == nil {
		t.Error("expected error for non-integer operand, got nil")
	}
}

// Test_makeArrayByteCode_IntCoercion_DynamicMode verifies that in dynamic
// (no-type-enforcement) mode a narrower integer (int32) is coerced to match
// the int base type declared for the array.  The coercion is applied by
// coerceConstantArrayInitializer when the value and base type are both integer
// kinds.
func Test_makeArrayByteCode_IntCoercion_DynamicMode(t *testing.T) {
	// int32(7) pushed into a []int array; dynamic mode coerces compatible types.
	tc := newTestContext(t).withStack(int32(7), data.IntType)

	tc.assertNoError(makeArrayByteCode(tc.ctx, 1))

	got, popErr := tc.ctx.Pop()
	if popErr != nil {
		t.Fatalf("Pop failed: %v", popErr)
	}

	arr, ok := got.(*data.Array)
	if !ok {
		t.Fatalf("expected *data.Array, got %T", got)
	}

	elem, _ := arr.Get(0)

	// Dynamic mode coerces int32 → int to match the declared base type.
	if _, isInt := elem.(int); !isInt {
		t.Errorf("dynamic coercion: element got %T(%v), want int", elem, elem)
	}
}

// Test_makeArrayByteCode_IntCoercion_StrictMode verifies that in strict
// type-enforcement mode a narrower integer (int32) placed into an int-typed
// array is still coerced to int, because both types are integer kinds and the
// strict path in coerceConstantArrayInitializer coerces compatible integers.
func Test_makeArrayByteCode_IntCoercion_StrictMode(t *testing.T) {
	tc := newTestContext(t).
		withTypeStrictness(defs.StrictTypeEnforcement).
		withStack(int32(7), data.IntType)

	tc.assertNoError(makeArrayByteCode(tc.ctx, 1))

	got, popErr := tc.ctx.Pop()
	if popErr != nil {
		t.Fatalf("Pop failed: %v", popErr)
	}

	arr, ok := got.(*data.Array)
	if !ok {
		t.Fatalf("expected *data.Array, got %T", got)
	}

	elem, _ := arr.Get(0)

	// Strict mode still coerces compatible integer widths.
	if _, isInt := elem.(int); !isInt {
		t.Errorf("strict coercion: element got %T(%v), want int", elem, elem)
	}
}

// Test_makeArrayByteCode_IncompatibleType_StrictMode verifies that in strict
// mode a string value for an int-typed array causes ErrWrongArrayValueType,
// because string is not an integer type and strict mode rejects the mismatch
// rather than coercing.
func Test_makeArrayByteCode_IncompatibleType_StrictMode(t *testing.T) {
	tc := newTestContext(t).
		withTypeStrictness(defs.StrictTypeEnforcement).
		withStack("hello", data.IntType)

	tc.assertError(makeArrayByteCode(tc.ctx, 1), errors.ErrWrongArrayValueType)
}

// ─────────────────────────────────────────────────────────────────────────────
// Flat-style tests for arrayByteCode
//
// Stack layout for arrayByteCode with an integer operand N:
//
//	top:    element[0]  (popped first → stored at result[0])
//	...     element[N-1]
//
// When the operand is []any{count, type}, elements are coerced to that type.
// Strict-mode tests pass a withTypeStrictness option and a typed operand.
// ─────────────────────────────────────────────────────────────────────────────

// Test_arrayByteCode_UntypedTwoElements verifies that an integer operand
// creates an interface-typed array containing the two popped values in order.
func Test_arrayByteCode_UntypedTwoElements(t *testing.T) {
	// withStack pushes left-to-right; rightmost value ends up on top (popped first).
	// Pop order: 42 (element[0]), "hello" (element[1]).
	// Result: []interface{"hello", 42} — first-pushed = first element.
	tc := newTestContext(t).withStack("hello", 42)

	tc.assertNoError(arrayByteCode(tc.ctx, 2))

	want := data.NewArrayFromList(data.InterfaceType, data.NewList("hello", 42))
	tc.assertTopStack(want)
	tc.assertStackEmpty()
}

// Test_arrayByteCode_TypedArray verifies that a []any{count, type} operand
// creates a strongly-typed array and coerces every element to that type.
func Test_arrayByteCode_TypedArray(t *testing.T) {
	// Push three int32 values; the operand declares a []int32 of length 3.
	tc := newTestContext(t).withStack(int32(10), int32(20), int32(30))

	tc.assertNoError(arrayByteCode(tc.ctx, []any{3, data.Int32Type}))

	want := data.NewArrayFromList(data.Int32Type, data.NewList(int32(10), int32(20), int32(30)))
	tc.assertTopStack(want)
	tc.assertStackEmpty()
}

// Test_arrayByteCode_EmptyArray verifies that count=0 (no elements to pop)
// produces a valid zero-length interface-typed array without error.
func Test_arrayByteCode_EmptyArray(t *testing.T) {
	tc := newTestContext(t) // nothing on the stack — zero pops needed

	tc.assertNoError(arrayByteCode(tc.ctx, 0))

	// An integer operand implies an interface-typed element type.
	want := data.NewArray(data.InterfaceType, 0)
	tc.assertTopStack(want)
	tc.assertStackEmpty()
}

// Test_arrayByteCode_StrictMode_HomogeneousArray verifies that a uniform
// sequence of int32 values passes strict type enforcement (all elements share
// the same concrete reflect type, so no error is raised).
func Test_arrayByteCode_StrictMode_HomogeneousArray(t *testing.T) {
	tc := newTestContext(t).
		withTypeStrictness(defs.StrictTypeEnforcement).
		withStack(int32(1), int32(2), int32(3))

	// An untyped-operand (integer 3) array in strict mode detects the element
	// type from the first element and requires all subsequent elements to match.
	tc.assertNoError(arrayByteCode(tc.ctx, 3))
}

// Test_arrayByteCode_StrictMode_HeterogeneousArray verifies that a mixed-type
// sequence inside a typed array literal in strict mode returns ErrInvalidType.
// The typed operand []any{2, Int32Type} is used so the array type is explicit.
func Test_arrayByteCode_StrictMode_HeterogeneousArray(t *testing.T) {
	// withStack("bad", int32(1)): int32(1) is on top (element[0] — coercion
	// to int32 succeeds), then "bad" is popped as element[1] and
	// reflect.TypeOf("bad") != reflect.TypeOf(int32(1)) → ErrInvalidType.
	tc := newTestContext(t).
		withTypeStrictness(defs.StrictTypeEnforcement).
		withStack("bad", int32(1))

	err := arrayByteCode(tc.ctx, []any{2, data.Int32Type})
	if err == nil {
		t.Error("strict mode: expected ErrInvalidType for mixed-type array, got nil")
	}
}

// Test_arrayByteCode_StackUnderflow verifies that requesting more elements
// than are available on the stack returns ErrStackUnderflow.
func Test_arrayByteCode_StackUnderflow(t *testing.T) {
	// Only one item on the stack, but the operand requests three.
	tc := newTestContext(t).withStack(42)

	tc.assertError(arrayByteCode(tc.ctx, 3), errors.ErrStackUnderflow)
}

// Test_arrayByteCode_StackMarkerAsElement verifies that a StackMarker in the
// element stream causes ErrFunctionReturnedVoid.  Unlike makeArrayByteCode,
// arrayByteCode propagates Pop errors immediately.
func Test_arrayByteCode_StackMarkerAsElement(t *testing.T) {
	// Bottom element is a StackMarker; it is encountered on the second pop.
	tc := newTestContext(t).withStack(NewStackMarker("void"), 99)

	tc.assertError(arrayByteCode(tc.ctx, 2), errors.ErrFunctionReturnedVoid)
}

// ─────────────────────────────────────────────────────────────────────────────
// Flat-style tests for makeMapByteCode
//
// Stack layout for makeMapByteCode (top → bottom):
//
//	top:    keyType   (*data.Type)  — popped first
//	        valueType (*data.Type)  — popped second
//	        key[N-1]               — last pair pushed (first pair consumed in loop)
//	        value[N-1]
//	        ...
//	bottom: value[0]
//
// The operand is the number of key/value pairs (not counting the two type pops).
// ─────────────────────────────────────────────────────────────────────────────

// Test_makeMapByteCode_TwoStringIntPairs verifies that a two-pair map[string]int
// is built with correct key-value contents.
func Test_makeMapByteCode_TwoStringIntPairs(t *testing.T) {
	// makeMapByteCode pops VALUE first, then KEY for each pair.
	// So each pair must be laid out key-first (lower), value-second (higher).
	//
	// Stack (bottom→top): "a", 1, "b", 2, IntType, StringType.
	// Pop order: StringType (keyType), IntType (valueType),
	//            2 (value), "b" (key), 1 (value), "a" (key).
	tc := newTestContext(t).withStack("a", 1, "b", 2, data.IntType, data.StringType)

	tc.assertNoError(makeMapByteCode(tc.ctx, 2))

	want := data.NewMap(data.StringType, data.IntType)
	_, _ = want.Set("a", 1)
	_, _ = want.Set("b", 2)

	tc.assertTopStack(want)
	tc.assertStackEmpty()
}

// Test_makeMapByteCode_EmptyMap verifies that count=0 produces an empty map
// with the correct declared key and value types.
func Test_makeMapByteCode_EmptyMap(t *testing.T) {
	// Only the type descriptors are on the stack; no k/v pairs follow.
	// Pop order: StringType (keyType), IntType (valueType).
	tc := newTestContext(t).withStack(data.IntType, data.StringType)

	tc.assertNoError(makeMapByteCode(tc.ctx, 0))

	// The resulting map should be an empty map[string]int.
	tc.assertTopStack(data.NewMap(data.StringType, data.IntType))
	tc.assertStackEmpty()
}

// Test_makeMapByteCode_StackUnderflow_KeyType verifies that an empty stack
// (nothing to pop for the key type) returns ErrStackUnderflow on the very first
// Pop call.
func Test_makeMapByteCode_StackUnderflow_KeyType(t *testing.T) {
	tc := newTestContext(t) // empty stack

	tc.assertError(makeMapByteCode(tc.ctx, 0), errors.ErrStackUnderflow)
}

// Test_makeMapByteCode_StackMarkerAsValue verifies that a StackMarker found in
// the value position of a key/value pair causes ErrFunctionReturnedVoid.
func Test_makeMapByteCode_StackMarkerAsValue(t *testing.T) {
	// Stack (bottom→top): StackMarker (value), "x" (key), IntType, StringType.
	// When the loop pops the pair it checks for markers in both slots.
	tc := newTestContext(t).withStack(
		NewStackMarker("void"), "x", // pair: value=marker, key="x"
		data.IntType,    // value type
		data.StringType, // key type
	)

	tc.assertError(makeMapByteCode(tc.ctx, 1), errors.ErrFunctionReturnedVoid)
}

// Test_makeMapByteCode_StackMarkerAsKey verifies that a StackMarker in the key
// position of a key/value pair also causes ErrFunctionReturnedVoid.
func Test_makeMapByteCode_StackMarkerAsKey(t *testing.T) {
	// Stack (bottom→top): 42 (value), StackMarker (key), IntType, StringType.
	tc := newTestContext(t).withStack(
		42, NewStackMarker("void"), // pair: value=42, key=marker
		data.IntType,    // value type
		data.StringType, // key type
	)

	tc.assertError(makeMapByteCode(tc.ctx, 1), errors.ErrFunctionReturnedVoid)
}

// ─────────────────────────────────────────────────────────────────────────────
// Tests for coerceConstantArrayInitializer
//
// This unexported helper is tested directly because it encodes its own multi-
// branch type-enforcement logic that is hard to exercise exhaustively through
// the makeArrayByteCode surface.  Tests cover all three enforcement modes
// (dynamic, strict, relaxed) as well as the StackMarker guard that fires before
// any type check.
// ─────────────────────────────────────────────────────────────────────────────

// Test_coerceConstantArrayInitializer_DynamicMode_IntToInt32 verifies that in
// dynamic (no-type-enforcement) mode a plain int value is coerced to int32 when
// the base type is Int32Type.  Dynamic mode takes the else-branch that calls
// baseType.Coerce unconditionally for non-interface types.
func Test_coerceConstantArrayInitializer_DynamicMode_IntToInt32(t *testing.T) {
	tc := newTestContext(t).withTypeStrictness(defs.NoTypeEnforcement)

	// isInt=true because Int32Type.IsIntegerType() returns true.
	got, err := coerceConstantArrayInitializer(tc.ctx, data.Int32Type, 42, true, false)

	tc.assertNoError(err)

	if _, ok := got.(int32); !ok {
		t.Errorf("expected int32, got %T(%v)", got, got)
	}
}

// Test_coerceConstantArrayInitializer_DynamicMode_IntToFloat64 verifies that
// in dynamic mode an int literal is coerced to float64 when the base type
// requires it.
func Test_coerceConstantArrayInitializer_DynamicMode_IntToFloat64(t *testing.T) {
	tc := newTestContext(t).withTypeStrictness(defs.NoTypeEnforcement)

	// isFloat=true because Float64Type.IsFloatType() returns true.
	got, err := coerceConstantArrayInitializer(tc.ctx, data.Float64Type, 7, false, true)

	tc.assertNoError(err)

	if _, ok := got.(float64); !ok {
		t.Errorf("expected float64, got %T(%v)", got, got)
	}
}

// Test_coerceConstantArrayInitializer_StrictMode_CompatibleIntegers verifies
// that in strict mode an int32 value is coerced (not rejected) for an int-typed
// array, because both types are integer kinds and the first branch fires.
func Test_coerceConstantArrayInitializer_StrictMode_CompatibleIntegers(t *testing.T) {
	tc := newTestContext(t).withTypeStrictness(defs.StrictTypeEnforcement)

	// isInt=true; int32 is an integer type → coercion fires, no error.
	got, err := coerceConstantArrayInitializer(tc.ctx, data.IntType, int32(5), true, false)

	tc.assertNoError(err)

	// The returned value must be plain int, not the original int32.
	if _, ok := got.(int); !ok {
		t.Errorf("expected int after coercion, got %T(%v)", got, got)
	}
}

// Test_coerceConstantArrayInitializer_StrictMode_IncompatibleType verifies that
// in strict mode a string value for an int-typed array causes
// ErrWrongArrayValueType, because string is not an integer kind and the outer
// else-branch rejects the type mismatch.
func Test_coerceConstantArrayInitializer_StrictMode_IncompatibleType(t *testing.T) {
	tc := newTestContext(t).withTypeStrictness(defs.StrictTypeEnforcement)

	// isInt=true but string is not an integer type → ErrWrongArrayValueType.
	_, err := coerceConstantArrayInitializer(tc.ctx, data.IntType, "hello", true, false)

	tc.assertError(err, errors.ErrWrongArrayValueType)
}

// Test_coerceConstantArrayInitializer_RelaxedMode_CoercesAll verifies that in
// relaxed type-enforcement mode any value is coerced to the base type regardless
// of its original kind.  When isInt and isFloat are both false the function
// falls through to the RelaxedTypeEnforcement branch that calls Coerce
// unconditionally.
func Test_coerceConstantArrayInitializer_RelaxedMode_CoercesAll(t *testing.T) {
	tc := newTestContext(t).withTypeStrictness(defs.RelaxedTypeEnforcement)

	// isInt=false, isFloat=false: the relaxed branch coerces 99 (int) → "99".
	got, err := coerceConstantArrayInitializer(tc.ctx, data.StringType, 99, false, false)

	tc.assertNoError(err)

	if s, ok := got.(string); !ok || s != "99" {
		t.Errorf("relaxed coercion: expected \"99\", got %T(%v)", got, got)
	}
}

// Test_coerceConstantArrayInitializer_StackMarker verifies that a StackMarker
// value (which signals a void function result in the element stream) causes
// ErrFunctionReturnedVoid before any type check is attempted.
func Test_coerceConstantArrayInitializer_StackMarker(t *testing.T) {
	tc := newTestContext(t).withTypeStrictness(defs.NoTypeEnforcement)

	_, err := coerceConstantArrayInitializer(
		tc.ctx, data.IntType, NewStackMarker("void"), true, false,
	)

	tc.assertError(err, errors.ErrFunctionReturnedVoid)
}

// ─────────────────────────────────────────────────────────────────────────────
// Tests for the reverse() utility function
//
// reverse is a thin in-place slice-reversal helper; tests focus on edge cases
// (empty, single-element, odd and even lengths).
// ─────────────────────────────────────────────────────────────────────────────

// Test_reverse_EvenLength verifies that a slice with an even element count is
// fully reversed (both the left and right halves swap completely).
func Test_reverse_EvenLength(t *testing.T) {
	got := reverse([]string{"a", "b", "c", "d"})
	want := []string{"d", "c", "b", "a"}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("reverse even: got %v, want %v", got, want)
	}
}

// Test_reverse_OddLength verifies that a slice with an odd element count is
// reversed correctly; the middle element stays in place.
func Test_reverse_OddLength(t *testing.T) {
	got := reverse([]string{"x", "y", "z"})
	want := []string{"z", "y", "x"}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("reverse odd: got %v, want %v", got, want)
	}
}

// Test_reverse_SingleElement verifies that a one-element slice is returned
// unchanged (no out-of-bounds access).
func Test_reverse_SingleElement(t *testing.T) {
	got := reverse([]string{"only"})
	want := []string{"only"}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("reverse single: got %v, want %v", got, want)
	}
}

// Test_reverse_EmptySlice verifies that an empty slice does not panic and is
// returned as an empty slice.
func Test_reverse_EmptySlice(t *testing.T) {
	got := reverse([]string{})

	if len(got) != 0 {
		t.Errorf("reverse empty: expected empty slice, got %v", got)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Regression test for CREATE-2: addMissingFields field-type coercion
//
// addMissingFields is called when assembling a typed struct.  When a field in
// structMap has a value whose type differs from the declared model type, the
// function must coerce the value and write it back to structMap.
//
// Before the fix the condition was `if err == nil { return err }`: a successful
// coercion caused the function to return nil early without updating structMap,
// leaving the field with its original pre-coercion type.  Corrected to
// `if err != nil`.
//
// IMPORTANT — the coercion branch is guarded by `ft.Kind() != UndefinedKind`.
// data.Type.Field() returns UndefinedType (kind = UndefinedKind) for any type
// that is not a raw StructKind (e.g., a TypeDefinition wrapper has TypeKind).
// The test therefore drives addMissingFields directly, using a raw
// data.StructureType model so that Field() returns the real IntType and the
// coercion branch is actually reached.
//
// float64 is used for the mismatched field value rather than int32 because
// data.TypeOf(int32).IsType(IntType) returns true in Ego's type system (both
// are integer kinds), which causes the condition
// `!data.TypeOf(existingValue).IsType(ft)` to be false and the coercion block
// to be skipped entirely.  Float64Kind and IntKind are distinct, so float64
// correctly triggers the path.
// ─────────────────────────────────────────────────────────────────────────────

// Test_addMissingFields_FieldTypeCoercion_CREATE2 is the direct regression test
// for the CREATE-2 fix.  It calls addMissingFields with a raw StructureType
// model (StructKind) so that Field() returns the declared IntType.  The
// structMap contains float64(3.0) for a field declared as int.  After the fix
// addMissingFields must coerce the value and store int(3) in structMap.
func Test_addMissingFields_FieldTypeCoercion_CREATE2(t *testing.T) {
	// data.StructureType creates a *data.Type with kind=StructKind.  Using this
	// directly — instead of wrapping in data.TypeDefinition (TypeKind) — ensures
	// model.Type().Field("x") returns IntType rather than UndefinedType.
	structType := data.StructureType(data.Field{Name: "x", Type: data.IntType})

	// Build the model: a *data.Struct whose type is the raw StructureType above.
	// The field is initialized to its zero value (int 0).
	model := data.NewStructFromMap(map[string]any{"x": 0})
	model.AsType(structType)

	// structMap holds a float64 value for the int-typed field.
	// data.TypeOf(float64(3.0)).IsType(IntType) returns false (Float64Kind ≠
	// IntKind), so the coercion block fires.
	structMap := map[string]any{"x": float64(3.0)}

	tc := newTestContext(t)

	err := addMissingFields(model, structMap, tc.ctx)
	if err != nil {
		t.Fatalf("addMissingFields returned unexpected error: %v", err)
	}

	// After the CREATE-2 fix, structMap["x"] must be int(3).
	// Without the fix the function returned nil before the assignment, leaving
	// structMap["x"] as float64(3.0).
	v := structMap["x"]
	if _, isInt := v.(int); !isInt {
		t.Errorf("field x: got %T(%v), want int — CREATE-2 fix not applied", v, v)
	}
}
