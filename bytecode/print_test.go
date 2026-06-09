package bytecode

// print_test.go contains unit tests for the three functions in print.go:
//
//   - newlineByteCode   — writes a newline to the output writer
//   - printByteCode     — pops N stack values and prints them space-separated
//   - formatValueForPrinting — converts any Ego value to a display string
//
// All tests use the shared testContext harness from testhelpers_test.go.
// Output is captured by calling tc.ctx.EnableConsoleOutput(false) before
// invoking the instruction, then read back with tc.ctx.GetOutput().
//
// Test naming: Test_<function>_<Scenario>

import (
	"strings"
	"testing"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
)

// ─── Section 1: newlineByteCode ───────────────────────────────────────────────

// Test_newlineByteCode_WritesNewline verifies that the Newline instruction
// writes exactly one newline character to the output.
func Test_newlineByteCode_WritesNewline(t *testing.T) {
	tc := newTestContext(t)
	tc.ctx.EnableConsoleOutput(false)

	err := newlineByteCode(tc.ctx, nil)

	tc.assertNoError(err)

	got := tc.ctx.GetOutput()
	if got != "\n" {
		t.Errorf("newlineByteCode: got %q, want %q", got, "\n")
	}
}

// Test_newlineByteCode_NilOperand verifies that a nil operand is accepted
// without error — the Newline instruction never looks at its operand.
func Test_newlineByteCode_NilOperand(t *testing.T) {
	tc := newTestContext(t)
	tc.ctx.EnableConsoleOutput(false)

	err := newlineByteCode(tc.ctx, nil)

	tc.assertNoError(err)
}

// Test_newlineByteCode_IgnoresNonNilOperand verifies that a non-nil operand
// (like a string) is silently ignored — only "\n" is written.
func Test_newlineByteCode_IgnoresNonNilOperand(t *testing.T) {
	tc := newTestContext(t)
	tc.ctx.EnableConsoleOutput(false)

	err := newlineByteCode(tc.ctx, "ignored operand")

	tc.assertNoError(err)

	got := tc.ctx.GetOutput()
	if got != "\n" {
		t.Errorf("newlineByteCode with operand: got %q, want %q", got, "\n")
	}
}

// ─── Section 2: printByteCode — basic output ─────────────────────────────────

// Test_printByteCode_NilOperand_DefaultsToOnePop verifies that a nil operand
// causes exactly one item to be popped and printed.
func Test_printByteCode_NilOperand_DefaultsToOnePop(t *testing.T) {
	tc := newTestContext(t).withStack("hello")
	tc.ctx.EnableConsoleOutput(false)

	err := printByteCode(tc.ctx, nil)

	tc.assertNoError(err)
	tc.assertStackEmpty()

	got := tc.ctx.GetOutput()
	if got != "hello" {
		t.Errorf("printByteCode nil operand: got %q, want %q", got, "hello")
	}
}

// Test_printByteCode_StringValue verifies that a plain string is printed
// verbatim (no quotes, no type decoration).
func Test_printByteCode_StringValue(t *testing.T) {
	tc := newTestContext(t).withStack("world")
	tc.ctx.EnableConsoleOutput(false)

	err := printByteCode(tc.ctx, 1)

	tc.assertNoError(err)

	got := tc.ctx.GetOutput()
	if got != "world" {
		t.Errorf("string value: got %q, want %q", got, "world")
	}
}

// Test_printByteCode_IntValue verifies that an integer is printed as its
// decimal representation — data.Format routes int through strconv.Itoa.
func Test_printByteCode_IntValue(t *testing.T) {
	tc := newTestContext(t).withStack(42)
	tc.ctx.EnableConsoleOutput(false)

	err := printByteCode(tc.ctx, 1)

	tc.assertNoError(err)

	got := tc.ctx.GetOutput()
	if got != "42" {
		t.Errorf("int value: got %q, want %q", got, "42")
	}
}

// Test_printByteCode_BoolValue verifies that a boolean prints as "true" or
// "false" (no type decoration — data.Format treats bool as a special case).
func Test_printByteCode_BoolValue(t *testing.T) {
	tc := newTestContext(t).withStack(true)
	tc.ctx.EnableConsoleOutput(false)

	err := printByteCode(tc.ctx, 1)

	tc.assertNoError(err)

	got := tc.ctx.GetOutput()
	if got != "true" {
		t.Errorf("bool value: got %q, want %q", got, "true")
	}
}

// Test_printByteCode_NilValue verifies that a nil stack value is printed as
// "<nil>" (data.FormatUnquoted returns defs.NilTypeString for nil).
func Test_printByteCode_NilValue(t *testing.T) {
	tc := newTestContext(t).withStack(nil)
	tc.ctx.EnableConsoleOutput(false)

	err := printByteCode(tc.ctx, 1)

	tc.assertNoError(err)

	got := tc.ctx.GetOutput()
	if got != "<nil>" {
		t.Errorf("nil value: got %q, want %q", got, "<nil>")
	}
}

// ─── Section 3: printByteCode — explicit count, space separation ──────────────

// Test_printByteCode_TwoItems_SpaceSeparated verifies that two items are
// printed with a single space between them.
//
// The compiler pushes arguments in reverse order, so the first printed value
// ends up at the top of the stack.  We mirror that here: withStack pushes
// left-to-right, so the last argument in the slice ends up on top.
func Test_printByteCode_TwoItems_SpaceSeparated(t *testing.T) {
	// Stack: ["b" (bottom), "a" (top)]  →  output: "a b"
	tc := newTestContext(t).withStack("b", "a")
	tc.ctx.EnableConsoleOutput(false)

	err := printByteCode(tc.ctx, 2)

	tc.assertNoError(err)
	tc.assertStackEmpty()

	got := tc.ctx.GetOutput()
	if got != "a b" {
		t.Errorf("two items: got %q, want %q", got, "a b")
	}
}

// Test_printByteCode_ThreeItems_SpaceSeparated verifies that three items are
// printed with spaces between them.
func Test_printByteCode_ThreeItems_SpaceSeparated(t *testing.T) {
	// Stack: ["c" (bottom), "b", "a" (top)]  →  output: "a b c"
	tc := newTestContext(t).withStack("c", "b", "a")
	tc.ctx.EnableConsoleOutput(false)

	err := printByteCode(tc.ctx, 3)

	tc.assertNoError(err)
	tc.assertStackEmpty()

	got := tc.ctx.GetOutput()
	if got != "a b c" {
		t.Errorf("three items: got %q, want %q", got, "a b c")
	}
}

// ─── Section 4: printByteCode — error conditions ─────────────────────────────

// Test_printByteCode_EmptyStack_Error verifies that popping from an empty
// stack returns ErrMissingPrintItems.
func Test_printByteCode_EmptyStack_Error(t *testing.T) {
	tc := newTestContext(t)
	tc.ctx.EnableConsoleOutput(false)

	err := printByteCode(tc.ctx, nil)

	tc.assertError(err, errors.ErrMissingPrintItems)
}

// Test_printByteCode_StackMarker_ReturnsVoidError verifies that encountering
// a StackMarker during the pop loop returns ErrFunctionReturnedVoid.  A marker
// on the stack means the function returned no usable value.
func Test_printByteCode_StackMarker_ReturnsVoidError(t *testing.T) {
	// Push a non-"results" marker so the results-marker path is NOT taken,
	// and the marker ends up being popped mid-loop.
	tc := newTestContext(t).withStack(NewStackMarker("sentinel"))
	tc.ctx.EnableConsoleOutput(false)

	err := printByteCode(tc.ctx, 1)

	tc.assertError(err, errors.ErrFunctionReturnedVoid)
}

// Test_printByteCode_InvalidOperandType_Error verifies that an operand that
// cannot be converted to int (e.g., a *data.Array) causes a runtime error.
func Test_printByteCode_InvalidOperandType_Error(t *testing.T) {
	tc := newTestContext(t)
	tc.ctx.EnableConsoleOutput(false)

	// data.Int(*data.Array) fails because an array cannot be coerced to int.
	badOperand := data.NewArray(data.IntType, 1)

	err := printByteCode(tc.ctx, badOperand)

	if err == nil {
		t.Error("expected error for non-integer operand, got nil")
	}
}

// ─── Section 5: printByteCode — results-marker path ──────────────────────────
//
// When callRuntimeFunction returns a data.List, it pushes a StackMarker
// ("results") onto the stack first, then the list items in reverse order so
// that item 0 ends up on top.  printByteCode detects this marker and switches
// to "tuple mode": it counts the items above the marker, and if the LAST item
// popped (item N-1 in the original list) is nil it is silently skipped — that
// nil is the conventional "no error" return from a Go-style (value, error) pair.

// Test_printByteCode_ResultsMarker_TwoValues verifies that two non-nil values
// above a "results" marker are both printed, space-separated.
func Test_printByteCode_ResultsMarker_TwoValues(t *testing.T) {
	// callRuntimeFunction pushes: marker, "b", "a"  ("a" on top).
	tc := newTestContext(t).withStack(NewStackMarker("results"), "b", "a")
	tc.ctx.EnableConsoleOutput(false)

	err := printByteCode(tc.ctx, nil)

	tc.assertNoError(err)

	got := tc.ctx.GetOutput()
	if got != "a b" {
		t.Errorf("results marker two values: got %q, want %q", got, "a b")
	}
}

// Test_printByteCode_ResultsMarker_SkipsNilAtEnd verifies the common
// (value, nil_error) pattern: the nil error (last list item, bottom of the
// values above the marker) is silently suppressed.
func Test_printByteCode_ResultsMarker_SkipsNilAtEnd(t *testing.T) {
	// Simulate list [value, nil_error].
	// callRuntimeFunction pushes: marker, nil (error), "world" (value).
	// "world" is on top and popped first (n=0); nil is popped last (n=1,
	// count-1) and skipped because skipNil=true and IsNil(nil)=true.
	tc := newTestContext(t).withStack(NewStackMarker("results"), nil, "world")
	tc.ctx.EnableConsoleOutput(false)

	err := printByteCode(tc.ctx, nil)

	tc.assertNoError(err)

	got := tc.ctx.GetOutput()
	if got != "world" {
		t.Errorf("results marker skip nil: got %q, want %q", got, "world")
	}
}

// Test_printByteCode_ResultsMarker_SingleValue verifies that a single non-nil
// value above a results marker is printed normally.
func Test_printByteCode_ResultsMarker_SingleValue(t *testing.T) {
	// Simulate list [value] — no error slot.
	tc := newTestContext(t).withStack(NewStackMarker("results"), "solo")
	tc.ctx.EnableConsoleOutput(false)

	err := printByteCode(tc.ctx, nil)

	tc.assertNoError(err)

	got := tc.ctx.GetOutput()
	if got != "solo" {
		t.Errorf("results marker single value: got %q, want %q", got, "solo")
	}
}

// Test_printByteCode_ResultsMarker_NilOnlySkipped verifies that when the only
// item above the results marker is nil, skipNil causes it to be silently
// dropped and nothing is written to the output.
func Test_printByteCode_ResultsMarker_NilOnlySkipped(t *testing.T) {
	tc := newTestContext(t).withStack(NewStackMarker("results"), nil)
	tc.ctx.EnableConsoleOutput(false)

	err := printByteCode(tc.ctx, nil)

	tc.assertNoError(err)

	got := tc.ctx.GetOutput()
	if got != "" {
		t.Errorf("results marker nil only: got %q, want empty string", got)
	}
}

// ─── Section 6: formatValueForPrinting — scalar and simple types ──────────────

// Test_formatValueForPrinting_String verifies that a plain Go string is
// returned verbatim (no quotes, no type prefix).
func Test_formatValueForPrinting_String(t *testing.T) {
	got := formatValueForPrinting("hello")
	if got != "hello" {
		t.Errorf("string: got %q, want %q", got, "hello")
	}
}

// Test_formatValueForPrinting_Int verifies that an int is formatted as its
// decimal string.  data.Format routes int through strconv.Itoa — no type
// prefix is added.
func Test_formatValueForPrinting_Int(t *testing.T) {
	got := formatValueForPrinting(42)
	if got != "42" {
		t.Errorf("int: got %q, want %q", got, "42")
	}
}

// Test_formatValueForPrinting_Bool verifies that booleans format as "true" or
// "false" (data.Format has a dedicated bool case with no type prefix).
func Test_formatValueForPrinting_BoolTrue(t *testing.T) {
	got := formatValueForPrinting(true)
	if got != "true" {
		t.Errorf("bool true: got %q, want %q", got, "true")
	}
}

func Test_formatValueForPrinting_BoolFalse(t *testing.T) {
	got := formatValueForPrinting(false)
	if got != "false" {
		t.Errorf("bool false: got %q, want %q", got, "false")
	}
}

// Test_formatValueForPrinting_Float64 verifies the float64 format string.
// data.Format uses strconv.FormatFloat with format 'g' and precision 10,
// which suppresses trailing zeros: 3.14 prints as "3.14", not "3.140000".
func Test_formatValueForPrinting_Float64(t *testing.T) {
	got := formatValueForPrinting(float64(3.14))
	if got != "3.14" {
		t.Errorf("float64: got %q, want %q", got, "3.14")
	}
}

// Test_formatValueForPrinting_Nil verifies that a nil value formats as "<nil>"
// (defs.NilTypeString), not the empty string.
func Test_formatValueForPrinting_Nil(t *testing.T) {
	got := formatValueForPrinting(nil)
	if got != "<nil>" {
		t.Errorf("nil: got %q, want %q", got, "<nil>")
	}
}

// ─── Section 7: formatValueForPrinting — complex Ego types ───────────────────

// Test_formatValueForPrinting_Struct verifies that a *data.Struct is rendered
// via formats.StructAsString, which produces a two-column Field/Value table.
// We check for non-empty output and that field names and values appear.
func Test_formatValueForPrinting_Struct(t *testing.T) {
	st := data.StructureType(
		data.Field{Name: "Name", Type: data.StringType},
		data.Field{Name: "Score", Type: data.IntType},
	)

	s := data.NewStruct(st)
	s.SetAlways("Name", "Alice")
	s.SetAlways("Score", 99)

	got := formatValueForPrinting(s)

	if got == "" {
		t.Error("struct: expected non-empty output")
	}

	if !strings.Contains(got, "Alice") {
		t.Errorf("struct: output does not contain field value %q: %s", "Alice", got)
	}

	if !strings.Contains(got, "99") {
		t.Errorf("struct: output does not contain field value %q: %s", "99", got)
	}
}

// Test_formatValueForPrinting_Map verifies that a *data.Map is rendered via
// formats.MapAsString, producing a key/value table with the map contents.
func Test_formatValueForPrinting_Map(t *testing.T) {
	m := data.NewMap(data.StringType, data.IntType)
	_, _ = m.Set("x", 7)

	got := formatValueForPrinting(m)

	if got == "" {
		t.Error("map: expected non-empty output")
	}

	if !strings.Contains(got, "x") {
		t.Errorf("map: output does not contain key %q: %s", "x", got)
	}
}

// Test_formatValueForPrinting_Type verifies that a *data.Type is rendered by
// its String() method.
func Test_formatValueForPrinting_Type(t *testing.T) {
	typ := data.StructureType(
		data.Field{Name: "X", Type: data.IntType},
	)

	got := formatValueForPrinting(typ)

	if got == "" {
		t.Error("type: expected non-empty output from Type.String()")
	}
}

// Test_formatValueForPrinting_FunctionPointer_ProducesEmptyString verifies
// that a *data.Function pointer produces empty output — functions are not
// printable data in Ego.
func Test_formatValueForPrinting_FunctionPointer_ProducesEmptyString(t *testing.T) {
	fn := &data.Function{
		Declaration: &data.Declaration{Name: "myFunc"},
	}

	got := formatValueForPrinting(fn)
	if got != "" {
		t.Errorf("*data.Function pointer: expected empty string, got %q", got)
	}
}

// Test_formatValueForPrinting_FunctionValue_SuppressedAfterFix_PRINT2 verifies
// that a data.Function VALUE (not pointer) is also suppressed after the PRINT-2
// fix.  Before the fix the value case fell through to default and produced the
// function's declaration string; after the fix both forms produce empty output.
func Test_formatValueForPrinting_FunctionValue_SuppressedAfterFix_PRINT2(t *testing.T) {
	fn := data.Function{
		Declaration: &data.Declaration{Name: "myFunc"},
	}

	got := formatValueForPrinting(fn)
	if got != "" {
		t.Errorf("data.Function value (PRINT-2 fix): expected empty string, got %q", got)
	}
}

// Test_formatValueForPrinting_ArrayOfStrings verifies that a non-struct array
// is formatted as a newline-joined string of element values.
func Test_formatValueForPrinting_ArrayOfStrings(t *testing.T) {
	arr := data.NewArrayFromInterfaces(data.StringType, "alpha", "beta", "gamma")

	got := formatValueForPrinting(arr)

	want := "alpha\nbeta\ngamma"
	if got != want {
		t.Errorf("string array: got %q, want %q", got, want)
	}
}

// Test_formatValueForPrinting_ArrayOfStructs verifies that a struct array is
// formatted as a multi-column text table with all fields and values present.
func Test_formatValueForPrinting_ArrayOfStructs(t *testing.T) {
	structType := data.StructureType(
		data.Field{Name: "Name", Type: data.StringType},
		data.Field{Name: "Age", Type: data.IntType},
	)

	arr := data.NewArray(structType, 2)

	row0 := data.NewStruct(structType)
	row0.SetAlways("Name", "Alice")
	row0.SetAlways("Age", 30)
	_ = arr.Set(0, row0)

	row1 := data.NewStruct(structType)
	row1.SetAlways("Name", "Bob")
	row1.SetAlways("Age", 25)
	_ = arr.Set(1, row1)

	got := formatValueForPrinting(arr)

	if got == "" {
		t.Error("struct array: expected non-empty table output")
	}

	for _, want := range []string{"Name", "Age", "Alice", "30", "Bob", "25"} {
		if !strings.Contains(got, want) {
			t.Errorf("struct array: output missing %q\nFull output:\n%s", want, got)
		}
	}
}

// Test_formatValueForPrinting_ArrayOfStructs_NilElement_PRINT1 verifies the
// PRINT-1 fix: nil elements in a struct array are silently skipped rather than
// causing a panic from an unchecked type assertion.
//
// data.NewArray(structType, 3) leaves all three slots as nil because struct
// elements are not zero-initialized by NewArray.  After the fix, the formatter
// iterates without panicking and only rows with valid *data.Struct values appear
// in the output.
func Test_formatValueForPrinting_ArrayOfStructs_NilElement_PRINT1(t *testing.T) {
	structType := data.StructureType(
		data.Field{Name: "City", Type: data.StringType},
	)

	// Three-element array: only slots 0 and 2 are populated; slot 1 stays nil.
	arr := data.NewArray(structType, 3)

	row0 := data.NewStruct(structType)
	row0.SetAlways("City", "Paris")
	_ = arr.Set(0, row0)
	// slot 1 intentionally left nil

	row2 := data.NewStruct(structType)
	row2.SetAlways("City", "Rome")
	_ = arr.Set(2, row2)

	// Before the PRINT-1 fix this call would panic.
	got := formatValueForPrinting(arr)

	if !strings.Contains(got, "Paris") {
		t.Errorf("nil element test: output missing %q\nFull output:\n%s", "Paris", got)
	}

	if !strings.Contains(got, "Rome") {
		t.Errorf("nil element test: output missing %q\nFull output:\n%s", "Rome", got)
	}
}
