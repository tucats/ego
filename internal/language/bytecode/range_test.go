package bytecode

// range_test.go — comprehensive unit tests for bytecode/range.go.
//
// # What is tested
//
// rangeInitByteCode  — sets up a for-range loop by popping the value to iterate
//                      from the stack, creating loop variables in the symbol
//                      table, and pushing a rangeDefinition onto the context's
//                      rangeStack.
//
// rangeNextByteCode  — advances one step through a for-range loop.  If the
//                      iteration is complete it branches to the destination
//                      address and pops the rangeDefinition; otherwise it
//                      updates the loop variables and continues.
//
// The five rangeNext* helpers are tested indirectly through rangeNextByteCode
// and also directly where their specific behavior needs isolation.
//
// # Bugs fixed in this session (RANGE-1 through RANGE-5)
//
// All five bugs discovered during the initial audit have been resolved.
// Each affected test was updated to assert the corrected behavior.
// See docs/BYTECODE_ISSUES.md for the full fix descriptions.
//
// # Testing conventions
//
// - One flat test function per scenario so every case is independently runnable
//   with -run.
// - Each test uses newTestContext(t) from testhelpers_test.go to create a
//   correctly initialized Context, ByteCode, and two-level symbol table.
// - Tests that exercise "exhaustion" (loop ends) call withBytecodeSize(N) to
//   make destination address N a valid program counter value.
// - Direct access to tc.ctx.rangeStack is permitted because this file is in
//   the same package (bytecode).

import (
	"testing"

	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
)

// ─────────────────────────────────────────────────────────────────────────────
// Section 1 — rangeInitByteCode: valid value types
// ─────────────────────────────────────────────────────────────────────────────

// Test_rangeInitByteCode_String verifies that ranging over a string builds the
// correct keySet (byte offsets) and runes slice.
//
// Ego's for-range over a string follows Go semantics: the index variable
// receives the byte offset of each rune, not the sequential rune number.
// For ASCII text those are identical (0, 1, 2 ...), but for multi-byte UTF-8
// characters the offsets can jump (0, 3, 6 ...).
func Test_rangeInitByteCode_String(t *testing.T) {
	tc := newTestContext(t).withStack("abc")

	// The operand is a two-element slice: [indexVarName, valueVarName].
	err := rangeInitByteCode(tc.ctx, []any{"idx", "val"})
	tc.assertNoError(err)
	tc.assertStackEmpty()

	// rangeStack should now contain exactly one entry.
	if len(tc.ctx.rangeStack) != 1 {
		t.Fatalf("rangeStack length = %d, want 1", len(tc.ctx.rangeStack))
	}

	r := tc.ctx.rangeStack[0]

	// keySet must contain the byte offset of each rune.  For pure-ASCII "abc"
	// that is 0, 1, 2.
	wantKeys := []any{0, 1, 2}
	if len(r.keySet) != len(wantKeys) {
		t.Fatalf("keySet length = %d, want %d", len(r.keySet), len(wantKeys))
	}

	for i, k := range wantKeys {
		if r.keySet[i] != k {
			t.Errorf("keySet[%d] = %v, want %v", i, r.keySet[i], k)
		}
	}

	// runes slice must contain the corresponding rune values.
	wantRunes := []rune{'a', 'b', 'c'}
	if len(r.runes) != len(wantRunes) {
		t.Fatalf("runes length = %d, want %d", len(r.runes), len(wantRunes))
	}

	for i, ch := range wantRunes {
		if r.runes[i] != ch {
			t.Errorf("runes[%d] = %q, want %q", i, r.runes[i], ch)
		}
	}

	// The loop variable symbols must have been created in the symbol table.
	if _, ok := tc.ctx.symbols.Get("idx"); !ok {
		t.Error("symbol 'idx' was not created by rangeInitByteCode")
	}

	if _, ok := tc.ctx.symbols.Get("val"); !ok {
		t.Error("symbol 'val' was not created by rangeInitByteCode")
	}
}

// Test_rangeInitByteCode_String_MultiByteUTF8 verifies that the keySet for a
// multi-byte UTF-8 string contains byte offsets, not sequential integers.
// The string "日本語" has three runes each encoded as three bytes, so the byte
// offsets are 0, 3, 6.
func Test_rangeInitByteCode_String_MultiByteUTF8(t *testing.T) {
	tc := newTestContext(t).withStack("日本語")

	err := rangeInitByteCode(tc.ctx, []any{"i", "v"})
	tc.assertNoError(err)

	r := tc.ctx.rangeStack[0]

	// Each Japanese character is 3 bytes in UTF-8.
	wantKeys := []any{0, 3, 6}
	if len(r.keySet) != len(wantKeys) {
		t.Fatalf("keySet length = %d, want %d (byte offsets, not rune indices)",
			len(r.keySet), len(wantKeys))
	}

	for idx, k := range wantKeys {
		if r.keySet[idx] != k {
			t.Errorf("keySet[%d] = %v, want %v", idx, r.keySet[idx], k)
		}
	}
}

// Test_rangeInitByteCode_Array verifies that ranging over a *data.Array stores
// the array reference in the range definition and creates the loop variables.
func Test_rangeInitByteCode_Array(t *testing.T) {
	arr := data.NewArrayFromInterfaces(data.IntType, 10, 20, 30)
	tc := newTestContext(t).withStack(arr)

	err := rangeInitByteCode(tc.ctx, []any{"i", "v"})
	tc.assertNoError(err)

	if len(tc.ctx.rangeStack) != 1 {
		t.Fatalf("rangeStack length = %d, want 1", len(tc.ctx.rangeStack))
	}

	r := tc.ctx.rangeStack[0]
	if r.value != arr {
		t.Error("rangeDefinition.value should be the array pointer")
	}

	if r.index != 0 {
		t.Errorf("initial index = %d, want 0", r.index)
	}
}

// Test_rangeInitByteCode_Map verifies that ranging over a *data.Map stores the
// keyset, locks the map as readonly, and creates the loop variables.
//
// Maps are locked readonly during iteration so the loop body cannot add or
// remove keys from underneath the iterator.
func Test_rangeInitByteCode_Map(t *testing.T) {
	m := data.NewMap(data.StringType, data.IntType)
	m.Set("a", 1) //nolint:errcheck
	m.Set("b", 2) //nolint:errcheck

	tc := newTestContext(t).withStack(m)

	err := rangeInitByteCode(tc.ctx, []any{"k", "v"})
	tc.assertNoError(err)

	r := tc.ctx.rangeStack[0]

	// keySet must hold the map's keys (order is unspecified in maps).
	if len(r.keySet) != 2 {
		t.Fatalf("keySet length = %d, want 2", len(r.keySet))
	}

	// The map must now be readonly — trying to Set a value should fail.
	_, setErr := m.Set("c", 3)
	if setErr == nil {
		t.Error("map should be readonly during iteration, but Set succeeded")
	}

	if !errors.Equal(errors.New(setErr), errors.ErrImmutableMap) {
		t.Errorf("expected ErrImmutableMap, got %v", setErr)
	}
}

// Test_rangeInitByteCode_Channel verifies that ranging over a *data.Channel
// pushes a range definition without pre-building a keyset (channels are read
// on-demand by rangeNextChannel).
func Test_rangeInitByteCode_Channel(t *testing.T) {
	ch := data.NewChannel(3)
	tc := newTestContext(t).withStack(ch)

	err := rangeInitByteCode(tc.ctx, []any{"i", "v"})
	tc.assertNoError(err)

	if len(tc.ctx.rangeStack) != 1 {
		t.Fatalf("rangeStack length = %d, want 1", len(tc.ctx.rangeStack))
	}

	r := tc.ctx.rangeStack[0]
	if r.value != ch {
		t.Error("rangeDefinition.value should be the channel pointer")
	}
}

// Test_rangeInitByteCode_Integer verifies that ranging over an integer stores
// the integer as the upper bound in the range definition (as a plain int).
// Integer ranges repeat exactly n times (indices 0 through n-1).
func Test_rangeInitByteCode_Integer(t *testing.T) {
	tc := newTestContext(t).withStack(5)

	err := rangeInitByteCode(tc.ctx, []any{"i", ""})
	tc.assertNoError(err)

	r := tc.ctx.rangeStack[0]

	// data.Int converts all integer widths to plain int; the stored value must
	// be int(5), not the original operand type.
	if r.value != 5 {
		t.Errorf("rangeDefinition.value = %v (%T), want int(5)", r.value, r.value)
	}

	if r.index != 0 {
		t.Errorf("initial index = %d, want 0", r.index)
	}
}

// Test_rangeInitByteCode_IntegerTypes verifies that all integer subtypes
// (int8, int16, int32, int64, uint16, uint32, byte) are converted to int.
func Test_rangeInitByteCode_IntegerTypes(t *testing.T) {
	cases := []any{
		int8(3), int16(3), int32(3), int64(3),
		uint16(3), uint32(3), byte(3),
	}

	for _, v := range cases {
		tc := newTestContext(t).withStack(v)

		err := rangeInitByteCode(tc.ctx, []any{"i", ""})
		if err != nil {
			t.Errorf("rangeInitByteCode(%T) returned error: %v", v, err)

			continue
		}

		r := tc.ctx.rangeStack[0]
		if r.value != 3 {
			t.Errorf("value for %T = %v (%T), want int(3)", v, r.value, r.value)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 2 — rangeInitByteCode: variable-name edge cases
// ─────────────────────────────────────────────────────────────────────────────

// Test_rangeInitByteCode_DiscardedIndex verifies that when the index variable
// name is "_" (the Ego discard symbol), no symbol is created in the symbol
// table.  The discard symbol must never be stored to.
func Test_rangeInitByteCode_DiscardedIndex(t *testing.T) {
	tc := newTestContext(t).withStack("hi")

	err := rangeInitByteCode(tc.ctx, []any{defs.DiscardedVariable, "val"})
	tc.assertNoError(err)

	// Symbol "_" must NOT exist; the discard sentinel is never written to.
	if _, ok := tc.ctx.symbols.Get(defs.DiscardedVariable); ok {
		t.Errorf("symbol %q should NOT be created for discarded index", defs.DiscardedVariable)
	}

	// But the value variable must be created.
	if _, ok := tc.ctx.symbols.Get("val"); !ok {
		t.Error("symbol 'val' should have been created")
	}
}

// Test_rangeInitByteCode_DiscardedBoth verifies that when both names are "_",
// no symbols at all are created.  This is the `for range x {}` pattern.
func Test_rangeInitByteCode_DiscardedBoth(t *testing.T) {
	tc := newTestContext(t).withStack("hi")

	err := rangeInitByteCode(tc.ctx, []any{defs.DiscardedVariable, defs.DiscardedVariable})
	tc.assertNoError(err)

	if _, ok := tc.ctx.symbols.Get(defs.DiscardedVariable); ok {
		t.Errorf("symbol %q should NOT be created when both names are discarded", defs.DiscardedVariable)
	}
}

// Test_rangeInitByteCode_EmptyValueName verifies that an empty value-variable
// name ("") is treated as "no value variable" and no symbol is created.
// This represents `for i := range x {}` where the value is not captured.
func Test_rangeInitByteCode_EmptyValueName(t *testing.T) {
	tc := newTestContext(t).withStack(42)

	err := rangeInitByteCode(tc.ctx, []any{"i", ""})
	tc.assertNoError(err)

	// Symbol "i" must exist, but no empty-name symbol should have been created.
	if _, ok := tc.ctx.symbols.Get("i"); !ok {
		t.Error("symbol 'i' should have been created")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 3 — rangeInitByteCode: error cases
// ─────────────────────────────────────────────────────────────────────────────

// Test_rangeInitByteCode_EmptyStack verifies that popping from an empty stack
// returns an error.
func Test_rangeInitByteCode_EmptyStack(t *testing.T) {
	tc := newTestContext(t)

	err := rangeInitByteCode(tc.ctx, []any{"i", "v"})
	if err == nil {
		t.Fatal("expected an error when stack is empty, got nil")
	}
}

// Test_rangeInitByteCode_StackMarker verifies that a StackMarker on the stack
// is rejected with ErrFunctionReturnedVoid.  Stack markers are sentinel values
// used internally and must never be iterated over.
func Test_rangeInitByteCode_StackMarker(t *testing.T) {
	tc := newTestContext(t).withStack(NewStackMarker("test"))

	err := rangeInitByteCode(tc.ctx, []any{"i", "v"})
	tc.assertError(err, errors.ErrFunctionReturnedVoid)
}

// Test_rangeInitByteCode_UnsupportedType verifies that passing a type that the
// range loop cannot iterate over (here: a bool) returns ErrInvalidType and does
// NOT push a stale entry onto the rangeStack.
//
// RANGE-5 fix: rangeInitByteCode now guards the rangeStack push with
// `if err == nil { ... }`, so the push is skipped when the type-switch default
// case sets an error.  Previously the push always ran, leaving a range entry
// whose value field pointed at an invalid type.
func Test_rangeInitByteCode_UnsupportedType(t *testing.T) {
	tc := newTestContext(t).withStack(true) // bool is not iterable

	err := rangeInitByteCode(tc.ctx, []any{"i", "v"})

	if err == nil {
		t.Fatal("expected ErrInvalidType for bool operand, got nil")
	}

	if !errors.Equal(errors.New(err), errors.ErrInvalidType) {
		t.Errorf("expected ErrInvalidType, got %v", err)
	}

	// RANGE-5 fix: rangeStack must be empty — no stale entry was pushed.
	if len(tc.ctx.rangeStack) != 0 {
		t.Errorf("RANGE-5: rangeStack has %d entries after error, want 0",
			len(tc.ctx.rangeStack))
	}
}

// Test_rangeInitByteCode_NilOperand verifies graceful handling when the
// operand is nil (no variable names provided).  The range should still be
// pushed with empty variable names; no symbols should be created.
func Test_rangeInitByteCode_NilOperand(t *testing.T) {
	tc := newTestContext(t).withStack("hello")

	err := rangeInitByteCode(tc.ctx, nil) // nil operand = no variable names
	tc.assertNoError(err)

	// One entry on the range stack, both names empty.
	if len(tc.ctx.rangeStack) != 1 {
		t.Fatalf("rangeStack length = %d, want 1", len(tc.ctx.rangeStack))
	}

	r := tc.ctx.rangeStack[0]
	if r.indexName != "" || r.valueName != "" {
		t.Errorf("with nil operand, both names should be empty; got %q / %q",
			r.indexName, r.valueName)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 4 — rangeNextByteCode: dispatch and edge cases
// ─────────────────────────────────────────────────────────────────────────────

// Test_rangeNextByteCode_EmptyRangeStack verifies that when the rangeStack is
// empty (no active for-range loop), the program counter is set to the
// destination address and no error is returned.  This is a defensive guard for
// malformed bytecode.
func Test_rangeNextByteCode_EmptyRangeStack(t *testing.T) {
	const dest = 42
	tc := newTestContext(t).withBytecodeSize(dest + 1)

	err := rangeNextByteCode(tc.ctx, dest)
	tc.assertNoError(err)
	tc.assertProgramCounter(dest)

	// rangeStack must still be empty.
	if len(tc.ctx.rangeStack) != 0 {
		t.Errorf("rangeStack length = %d, want 0", len(tc.ctx.rangeStack))
	}
}

// Test_rangeNextByteCode_InvalidOperand verifies that a non-integer operand
// (the destination address must be an integer) causes rangeNextByteCode to
// return an error rather than panic.
func Test_rangeNextByteCode_InvalidOperand(t *testing.T) {
	tc := newTestContext(t)

	err := rangeNextByteCode(tc.ctx, "not-an-int")
	if err == nil {
		t.Fatal("expected error for non-integer operand, got nil")
	}
}

// Test_rangeNextByteCode_SliceAnyValue verifies that if (through some means)
// a []any value ends up as the range target, rangeNextByteCode returns ErrInvalidType.
//
// RANGE-4 fix: the error is now returned via c.runtimeError() so it carries module
// and source-line context, consistent with every other error return in this package.
// The test still asserts ErrInvalidType (the error key is unchanged); the fix is
// invisible to callers who only inspect the error key.
func Test_rangeNextByteCode_SliceAnyValue_RANGE4(t *testing.T) {
	tc := newTestContext(t).withBytecodeSize(10)

	// Bypass rangeInitByteCode (which would reject []any) and push the entry
	// directly to exercise the defensive case in rangeNextByteCode.
	tc.ctx.rangeStack = append(tc.ctx.rangeStack, &rangeDefinition{
		indexName: "i",
		valueName: "v",
		value:     []any{1, 2, 3},
	})

	err := rangeNextByteCode(tc.ctx, 10)

	if err == nil {
		t.Fatal("expected ErrInvalidType for []any value, got nil")
	}

	if !errors.Equal(errors.New(err), errors.ErrInvalidType) {
		t.Errorf("expected ErrInvalidType, got %v", err)
	}
}

// Test_rangeNextByteCode_DefaultCase_PopsRangeStack verifies RANGE-2 fix:
// when the switch's default case fires for an unknown value type, the program
// counter is set to the destination AND the rangeStack entry is popped.
//
// RANGE-2 fix: the original default case set c.programCounter but did NOT trim
// c.rangeStack, leaving a stale entry that could corrupt an outer loop's state on
// its next RangeNext call.  The fix adds the same stack-trim that every other
// exhaustion path already had.
func Test_rangeNextByteCode_DefaultCase_PopsRangeStack_RANGE2(t *testing.T) {
	const dest = 5
	tc := newTestContext(t).withBytecodeSize(dest + 1)

	// Push a range entry with a type that reaches the default case.
	// Bypass rangeInitByteCode (which would reject unknown types) and push
	// directly.  A struct pointer is something completely unknown to the range
	// dispatcher, so it will reliably land in the default case.
	type unknownStruct struct{ x int }

	tc.ctx.rangeStack = append(tc.ctx.rangeStack, &rangeDefinition{
		value: &unknownStruct{x: 1},
	})

	err := rangeNextByteCode(tc.ctx, dest)
	tc.assertNoError(err)         // default case does not return an error
	tc.assertProgramCounter(dest) // program counter jumps to destination

	// RANGE-2 fix: the stale entry must now be removed.
	if len(tc.ctx.rangeStack) != 0 {
		t.Errorf("RANGE-2: rangeStack has %d entries after default case, want 0",
			len(tc.ctx.rangeStack))
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 5 — rangeNextString: per-step verification
// ─────────────────────────────────────────────────────────────────────────────

// Test_rangeNextString_FullIteration verifies a complete for-range loop over a
// 3-character ASCII string.
//
// Expected per-step behavior:
//
//	step 0: index var = 0 (byte offset), value var = "a", PC unchanged
//	step 1: index var = 1,               value var = "b", PC unchanged
//	step 2: index var = 2,               value var = "c", PC unchanged
//	step 3: exhausted — PC = dest,       rangeStack trimmed to 0 entries
func Test_rangeNextString_FullIteration(t *testing.T) {
	const dest = 99
	tc := newTestContext(t).
		withBytecodeSize(dest + 1).
		withStack("abc")

	// initialize the range (creates symbols, populates rangeStack).
	if err := rangeInitByteCode(tc.ctx, []any{"idx", "val"}); err != nil {
		t.Fatalf("rangeInitByteCode: %v", err)
	}

	type want struct {
		idx any
		val string
	}

	steps := []want{
		{0, "a"},
		{1, "b"},
		{2, "c"},
	}

	for step, w := range steps {
		// Save program counter before the call; it must NOT change on a
		// non-exhausted step.
		pcBefore := tc.ctx.programCounter

		if err := rangeNextByteCode(tc.ctx, dest); err != nil {
			t.Fatalf("step %d: rangeNextByteCode error: %v", step, err)
		}

		if tc.ctx.programCounter != pcBefore {
			t.Errorf("step %d: program counter changed to %d before exhaustion",
				step, tc.ctx.programCounter)
		}

		tc.assertSymbolValue("idx", w.idx)
		tc.assertSymbolValue("val", w.val)
	}

	// Step 3 — exhaustion.
	if err := rangeNextByteCode(tc.ctx, dest); err != nil {
		t.Fatalf("exhaustion step: rangeNextByteCode error: %v", err)
	}

	tc.assertProgramCounter(dest)

	if len(tc.ctx.rangeStack) != 0 {
		t.Errorf("after exhaustion, rangeStack length = %d, want 0",
			len(tc.ctx.rangeStack))
	}
}

// Test_rangeNextString_EmptyString verifies that ranging over an empty string
// immediately exhausts (no iterations).
func Test_rangeNextString_EmptyString(t *testing.T) {
	const dest = 50
	tc := newTestContext(t).
		withBytecodeSize(dest + 1).
		withStack("")

	if err := rangeInitByteCode(tc.ctx, []any{"i", "v"}); err != nil {
		t.Fatalf("rangeInitByteCode: %v", err)
	}

	if err := rangeNextByteCode(tc.ctx, dest); err != nil {
		t.Fatalf("rangeNextByteCode: %v", err)
	}

	tc.assertProgramCounter(dest)
}

// Test_rangeNextString_DiscardedVariables verifies that when both loop variable
// names are discarded ("_"), no symbols are set and no error is raised.
func Test_rangeNextString_DiscardedVariables(t *testing.T) {
	const dest = 30
	tc := newTestContext(t).
		withBytecodeSize(dest + 1).
		withStack("x")

	if err := rangeInitByteCode(tc.ctx, []any{defs.DiscardedVariable, defs.DiscardedVariable}); err != nil {
		t.Fatalf("rangeInitByteCode: %v", err)
	}

	// Should advance without error and without touching any symbol.
	if err := rangeNextByteCode(tc.ctx, dest); err != nil {
		t.Fatalf("rangeNextByteCode: %v", err)
	}
}

// Test_rangeNextString_MultiByteUTF8 verifies that the index variable receives
// the byte offset (not the sequential rune count) for multi-byte characters.
func Test_rangeNextString_MultiByteUTF8(t *testing.T) {
	const dest = 99
	tc := newTestContext(t).
		withBytecodeSize(dest + 1).
		withStack("日本") // each character is 3 UTF-8 bytes

	if err := rangeInitByteCode(tc.ctx, []any{"i", "v"}); err != nil {
		t.Fatalf("rangeInitByteCode: %v", err)
	}

	// First step: rune '日' at byte offset 0.
	if err := rangeNextByteCode(tc.ctx, dest); err != nil {
		t.Fatalf("step 0: %v", err)
	}

	tc.assertSymbolValue("i", 0) // byte offset 0
	tc.assertSymbolValue("v", "日")

	// Second step: rune '本' at byte offset 3.
	if err := rangeNextByteCode(tc.ctx, dest); err != nil {
		t.Fatalf("step 1: %v", err)
	}

	tc.assertSymbolValue("i", 3) // byte offset 3 (not 1!)
	tc.assertSymbolValue("v", "本")
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 6 — rangeNextArray: per-step verification
// ─────────────────────────────────────────────────────────────────────────────

// Test_rangeNextArray_FullIteration verifies a complete for-range loop over a
// 3-element integer array.
func Test_rangeNextArray_FullIteration(t *testing.T) {
	const dest = 99

	arr := data.NewArrayFromInterfaces(data.IntType, 10, 20, 30)
	tc := newTestContext(t).
		withBytecodeSize(dest + 1).
		withStack(arr)

	if err := rangeInitByteCode(tc.ctx, []any{"i", "v"}); err != nil {
		t.Fatalf("rangeInitByteCode: %v", err)
	}

	type want struct {
		idx int
		val int
	}

	steps := []want{{0, 10}, {1, 20}, {2, 30}}

	for step, w := range steps {
		if err := rangeNextByteCode(tc.ctx, dest); err != nil {
			t.Fatalf("step %d: rangeNextByteCode error: %v", step, err)
		}

		tc.assertSymbolValue("i", w.idx)
		tc.assertSymbolValue("v", w.val)
	}

	// Exhaustion.
	if err := rangeNextByteCode(tc.ctx, dest); err != nil {
		t.Fatalf("exhaustion step: %v", err)
	}

	tc.assertProgramCounter(dest)

	if len(tc.ctx.rangeStack) != 0 {
		t.Errorf("rangeStack should be empty after exhaustion, got %d entries",
			len(tc.ctx.rangeStack))
	}
}

// Test_rangeNextArray_EmptyArray verifies that ranging over an empty array
// exhausts immediately.
func Test_rangeNextArray_EmptyArray(t *testing.T) {
	const dest = 20

	arr := data.NewArray(data.IntType, 0)
	tc := newTestContext(t).
		withBytecodeSize(dest + 1).
		withStack(arr)

	if err := rangeInitByteCode(tc.ctx, []any{"i", "v"}); err != nil {
		t.Fatalf("rangeInitByteCode: %v", err)
	}

	if err := rangeNextByteCode(tc.ctx, dest); err != nil {
		t.Fatalf("rangeNextByteCode: %v", err)
	}

	tc.assertProgramCounter(dest)
}

// Test_rangeNextArray_DiscardedIndex verifies that when the index variable is
// "_", the symbol is not set and no error results.
func Test_rangeNextArray_DiscardedIndex(t *testing.T) {
	const dest = 20

	arr := data.NewArrayFromInterfaces(data.IntType, 42)
	tc := newTestContext(t).
		withBytecodeSize(dest + 1).
		withStack(arr)

	if err := rangeInitByteCode(tc.ctx, []any{defs.DiscardedVariable, "v"}); err != nil {
		t.Fatalf("rangeInitByteCode: %v", err)
	}

	if err := rangeNextByteCode(tc.ctx, dest); err != nil {
		t.Fatalf("rangeNextByteCode: %v", err)
	}

	// Value variable "v" must be set to the element.
	tc.assertSymbolValue("v", 42)
}

// Test_rangeNextArray_DiscardedValue verifies that when the value variable is
// "_", only the index is set.
func Test_rangeNextArray_DiscardedValue(t *testing.T) {
	const dest = 20

	arr := data.NewArrayFromInterfaces(data.IntType, 99)
	tc := newTestContext(t).
		withBytecodeSize(dest + 1).
		withStack(arr)

	if err := rangeInitByteCode(tc.ctx, []any{"i", defs.DiscardedVariable}); err != nil {
		t.Fatalf("rangeInitByteCode: %v", err)
	}

	if err := rangeNextByteCode(tc.ctx, dest); err != nil {
		t.Fatalf("rangeNextByteCode: %v", err)
	}

	tc.assertSymbolValue("i", 0)
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 7 — rangeNextMap: per-step verification
// ─────────────────────────────────────────────────────────────────────────────

// Test_rangeNextMap_FullIteration verifies a complete for-range over a two-entry
// map.  Because map iteration order is undefined, the test collects all produced
// key/value pairs and checks them as a set.
func Test_rangeNextMap_FullIteration(t *testing.T) {
	const dest = 99

	m := data.NewMap(data.StringType, data.IntType)
	m.Set("x", 10) //nolint:errcheck
	m.Set("y", 20) //nolint:errcheck

	tc := newTestContext(t).
		withBytecodeSize(dest + 1).
		withStack(m)

	if err := rangeInitByteCode(tc.ctx, []any{"k", "v"}); err != nil {
		t.Fatalf("rangeInitByteCode: %v", err)
	}

	// Collect all key→value pairs produced by the loop.
	got := map[string]int{}

	for {
		if err := rangeNextByteCode(tc.ctx, dest); err != nil {
			t.Fatalf("rangeNextByteCode: %v", err)
		}

		if tc.ctx.programCounter == dest {
			break // exhausted
		}

		kRaw, _ := tc.ctx.symbols.Get("k")
		vRaw, _ := tc.ctx.symbols.Get("v")

		k, _ := kRaw.(string)
		v, _ := vRaw.(int)
		got[k] = v
	}

	// Verify we collected both entries.
	if len(got) != 2 {
		t.Fatalf("collected %d entries, want 2; got %v", len(got), got)
	}

	if got["x"] != 10 {
		t.Errorf("got[\"x\"] = %d, want 10", got["x"])
	}

	if got["y"] != 20 {
		t.Errorf("got[\"y\"] = %d, want 20", got["y"])
	}

	// After exhaustion the map must be unlocked (immutable counter back to 0).
	_, setErr := m.Set("z", 30)
	if setErr != nil {
		t.Errorf("map should be writable after range exhaustion, got error: %v", setErr)
	}

	// rangeStack must be empty.
	if len(tc.ctx.rangeStack) != 0 {
		t.Errorf("rangeStack should be empty, got %d entries", len(tc.ctx.rangeStack))
	}
}

// Test_rangeNextMap_EmptyMap verifies that ranging over an empty map exhausts
// immediately AND clears the readonly lock.
func Test_rangeNextMap_EmptyMap(t *testing.T) {
	const dest = 20

	m := data.NewMap(data.StringType, data.IntType)
	tc := newTestContext(t).
		withBytecodeSize(dest + 1).
		withStack(m)

	if err := rangeInitByteCode(tc.ctx, []any{"k", "v"}); err != nil {
		t.Fatalf("rangeInitByteCode: %v", err)
	}

	if err := rangeNextByteCode(tc.ctx, dest); err != nil {
		t.Fatalf("rangeNextByteCode: %v", err)
	}

	tc.assertProgramCounter(dest)

	// Map must be writable again.
	if _, setErr := m.Set("a", 1); setErr != nil {
		t.Errorf("map not writable after exhaustion: %v", setErr)
	}
}

// Test_rangeNextMap_ReadonlyDuringIteration verifies that the map remains
// locked while iteration is in progress (before exhaustion).
func Test_rangeNextMap_ReadonlyDuringIteration(t *testing.T) {
	const dest = 99

	m := data.NewMap(data.StringType, data.IntType)
	m.Set("a", 1) //nolint:errcheck

	tc := newTestContext(t).
		withBytecodeSize(dest + 1).
		withStack(m)

	if err := rangeInitByteCode(tc.ctx, []any{"k", "v"}); err != nil {
		t.Fatalf("rangeInitByteCode: %v", err)
	}

	// Execute one step (does not exhaust the one-entry map).
	if err := rangeNextByteCode(tc.ctx, dest); err != nil {
		t.Fatalf("rangeNextByteCode: %v", err)
	}

	// Map must still be readonly: we have consumed the only entry but
	// rangeNextByteCode's exhaustion check fires on the NEXT call.
	// (The first call advances r.index to 1; the second call sees
	//  r.index >= len(keySet) and branches.  Between those two calls
	//  the map is still locked.)
	_, setErr := m.Set("b", 2)
	if setErr == nil {
		t.Error("map should still be readonly during iteration, but Set succeeded")
	}
}

// Test_rangeNextMap_EarlyExitReleasesMap verifies the RANGE-3 fix:
// when a for-range loop over a map exits early (without reaching exhaustion),
// the map's readonly lock is released so the map can be modified afterward.
//
// RANGE-3 fix: rangeInitByteCode now stores a cleanup closure on the
// rangeDefinition (r.cleanup = func() { actual.SetReadonly(false) }).
// popScopeByteCode was updated to call r.release() on any range entries whose
// scopeDepth matches the scope being popped, which covers the early-exit path.
// The rangeNextMap exhaustion path was also updated to call r.release() instead
// of SetReadonly(false) directly, so double-release is impossible.
func Test_rangeNextMap_EarlyExitReleasesMap_RANGE3(t *testing.T) {
	m := data.NewMap(data.StringType, data.IntType)
	m.Set("a", 1) //nolint:errcheck
	m.Set("b", 2) //nolint:errcheck

	// Use a real for-scope so that popScopeByteCode can detect and release the
	// range entry.  withBytecodeSize makes the destination address valid.
	tc := newTestContext(t).withBytecodeSize(10).withStack(m)

	// rangeInitByteCode locks the map and records r.scopeDepth = c.blockDepth.
	if err := rangeInitByteCode(tc.ctx, []any{"k", "v"}); err != nil {
		t.Fatalf("rangeInitByteCode: %v", err)
	}

	// The map is now locked.
	if _, setErr := m.Set("x", 99); setErr == nil {
		t.Fatal("expected map to be locked after rangeInitByteCode")
	}

	// Simulate one loop iteration (early break — we stop before exhaustion).
	if err := rangeNextByteCode(tc.ctx, 10); err != nil {
		t.Fatalf("rangeNextByteCode: %v", err)
	}

	// Simulate popScopeByteCode: it decrements blockDepth and releases any
	// range entries whose scopeDepth > new blockDepth.  Call it directly rather
	// than running a full bytecode program so the test stays self-contained.
	if err := popScopeByteCode(tc.ctx, nil); err != nil {
		t.Fatalf("popScopeByteCode: %v", err)
	}

	// RANGE-3 fix: map must now be writable.
	if _, setErr := m.Set("c", 3); setErr != nil {
		t.Errorf("RANGE-3: map still locked after scope pop: %v", setErr)
	}

	// The range entry must have been removed from the stack.
	if len(tc.ctx.rangeStack) != 0 {
		t.Errorf("rangeStack has %d entries after scope pop, want 0", len(tc.ctx.rangeStack))
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 8 — rangeNextChannel: per-step verification
// ─────────────────────────────────────────────────────────────────────────────

// Test_rangeNextChannel_FullIteration verifies that a buffered channel with
// three values is consumed correctly and then exhausts.
func Test_rangeNextChannel_FullIteration(t *testing.T) {
	const dest = 99

	ch := data.NewChannel(3)

	ch.Send(100) //nolint:errcheck
	ch.Send(200) //nolint:errcheck
	ch.Send(300) //nolint:errcheck
	ch.Close()   // closing signals the range to stop after draining

	tc := newTestContext(t).
		withBytecodeSize(dest + 1).
		withStack(ch)

	if err := rangeInitByteCode(tc.ctx, []any{"i", "v"}); err != nil {
		t.Fatalf("rangeInitByteCode: %v", err)
	}

	wantVals := []any{100, 200, 300}

	for step, want := range wantVals {
		pcBefore := tc.ctx.programCounter

		if err := rangeNextByteCode(tc.ctx, dest); err != nil {
			t.Fatalf("step %d: rangeNextByteCode error: %v", step, err)
		}

		if tc.ctx.programCounter != pcBefore {
			t.Errorf("step %d: PC changed prematurely", step)
		}

		tc.assertSymbolValue("i", step)
		tc.assertSymbolValue("v", want)
	}

	// Exhaustion: closed channel with no remaining items.
	if err := rangeNextByteCode(tc.ctx, dest); err != nil {
		t.Fatalf("exhaustion step: %v", err)
	}

	tc.assertProgramCounter(dest)

	if len(tc.ctx.rangeStack) != 0 {
		t.Errorf("rangeStack not empty after channel exhaustion")
	}
}

// Test_rangeNextChannel_ClosedEmptyChannel verifies that ranging over a channel
// that is already closed and empty exhausts immediately.
func Test_rangeNextChannel_ClosedEmptyChannel(t *testing.T) {
	const dest = 15

	ch := data.NewChannel(1)
	ch.Close()

	tc := newTestContext(t).
		withBytecodeSize(dest + 1).
		withStack(ch)

	if err := rangeInitByteCode(tc.ctx, []any{"i", "v"}); err != nil {
		t.Fatalf("rangeInitByteCode: %v", err)
	}

	if err := rangeNextByteCode(tc.ctx, dest); err != nil {
		t.Fatalf("rangeNextByteCode: %v", err)
	}

	tc.assertProgramCounter(dest)
}

// Test_rangeNextChannel_DiscardedVariables verifies that the channel iterator
// works correctly when both loop variables are discarded ("_").
func Test_rangeNextChannel_DiscardedVariables(t *testing.T) {
	const dest = 10

	ch := data.NewChannel(1)
	ch.Send(42) //nolint:errcheck
	ch.Close()

	tc := newTestContext(t).
		withBytecodeSize(dest + 1).
		withStack(ch)

	if err := rangeInitByteCode(tc.ctx, []any{defs.DiscardedVariable, defs.DiscardedVariable}); err != nil {
		t.Fatalf("rangeInitByteCode: %v", err)
	}

	// Should advance without error.
	if err := rangeNextByteCode(tc.ctx, dest); err != nil {
		t.Fatalf("first step: %v", err)
	}

	// Exhaustion.
	if err := rangeNextByteCode(tc.ctx, dest); err != nil {
		t.Fatalf("exhaustion step: %v", err)
	}

	tc.assertProgramCounter(dest)
}

// Test_rangeNextChannel_SingleVarReceivesValue is the regression test for BUG-01.
//
// # Root cause
//
// When the Ego compiler compiles "for v := range ch { ... }" it emits
//
//	RangeInit ["v", ""]
//
// placing the sole loop variable "v" in the indexName slot and leaving
// valueName empty.  Before the fix, rangeNextChannel treated indexName as an
// array-style index and unconditionally wrote the loop counter (r.index = 0, 1,
// 2, …) into it, discarding the received channel value entirely.  The caller
// therefore saw 0, 1, 2 instead of the actual channel values.
//
// # Fix
//
// rangeNextChannel now checks whether valueName is present:
//
//   - Two-variable form (indexName + valueName both set): indexName gets the
//     loop counter and valueName gets the received value — unchanged behavior.
//
//   - Single-variable form (valueName empty or "_"): indexName gets the received
//     value, because channels carry no meaningful positional index.
func Test_rangeNextChannel_SingleVarReceivesValue(t *testing.T) {
	const dest = 99

	ch := data.NewChannel(3)

	ch.Send(100) //nolint:errcheck
	ch.Send(200) //nolint:errcheck
	ch.Send(300) //nolint:errcheck
	ch.Close()

	// Simulate "for v := range ch": compiler puts "v" in indexName, valueName = "".
	tc := newTestContext(t).
		withBytecodeSize(dest + 1).
		withStack(ch)

	if err := rangeInitByteCode(tc.ctx, []any{"v", ""}); err != nil {
		t.Fatalf("rangeInitByteCode: %v", err)
	}

	wantVals := []any{100, 200, 300}

	for step, want := range wantVals {
		pcBefore := tc.ctx.programCounter

		if err := rangeNextByteCode(tc.ctx, dest); err != nil {
			t.Fatalf("step %d: rangeNextByteCode error: %v", step, err)
		}

		if tc.ctx.programCounter != pcBefore {
			t.Errorf("step %d: PC changed prematurely", step)
		}

		tc.assertSymbolValue("v", want)
	}

	// Exhaustion.
	if err := rangeNextByteCode(tc.ctx, dest); err != nil {
		t.Fatalf("exhaustion step: %v", err)
	}

	tc.assertProgramCounter(dest)

	if len(tc.ctx.rangeStack) != 0 {
		t.Errorf("rangeStack not empty after channel exhaustion")
	}
}

// Test_rangeNextChannel_SingleVarDiscarded verifies that the no-variable form
// "for range ch" (indexName = "_", valueName = "") consumes channel values
// without writing to any symbol.
func Test_rangeNextChannel_SingleVarDiscarded(t *testing.T) {
	const dest = 10

	ch := data.NewChannel(2)
	ch.Send(11) //nolint:errcheck
	ch.Send(22) //nolint:errcheck
	ch.Close()

	// Simulate "for _ := range ch": compiler emits indexName="_", valueName="".
	tc := newTestContext(t).
		withBytecodeSize(dest + 1).
		withStack(ch)

	if err := rangeInitByteCode(tc.ctx, []any{defs.DiscardedVariable, ""}); err != nil {
		t.Fatalf("rangeInitByteCode: %v", err)
	}

	// Both values must be consumed without error and without symbol writes.
	for step := 0; step < 2; step++ {
		if err := rangeNextByteCode(tc.ctx, dest); err != nil {
			t.Fatalf("step %d: %v", step, err)
		}
	}

	// Exhaustion.
	if err := rangeNextByteCode(tc.ctx, dest); err != nil {
		t.Fatalf("exhaustion step: %v", err)
	}

	tc.assertProgramCounter(dest)
}

// Test_rangeNextChannel_TwoVarCounterAndValue verifies that the two-variable
// form (for i, v := range ch) continues to work after the BUG-01 fix:
// the first variable still receives the loop counter and the second still
// receives the channel value.
func Test_rangeNextChannel_TwoVarCounterAndValue(t *testing.T) {
	const dest = 99

	ch := data.NewChannel(2)
	ch.Send("alpha") //nolint:errcheck
	ch.Send("beta")  //nolint:errcheck
	ch.Close()

	tc := newTestContext(t).
		withBytecodeSize(dest + 1).
		withStack(ch)

	if err := rangeInitByteCode(tc.ctx, []any{"i", "v"}); err != nil {
		t.Fatalf("rangeInitByteCode: %v", err)
	}

	type want struct {
		counter int
		value   string
	}

	steps := []want{
		{0, "alpha"},
		{1, "beta"},
	}

	for _, w := range steps {
		if err := rangeNextByteCode(tc.ctx, dest); err != nil {
			t.Fatalf("step %d: %v", w.counter, err)
		}

		// i must still be the loop counter, not the value.
		tc.assertSymbolValue("i", w.counter)
		// v must be the received channel value.
		tc.assertSymbolValue("v", w.value)
	}

	// Exhaustion.
	if err := rangeNextByteCode(tc.ctx, dest); err != nil {
		t.Fatalf("exhaustion: %v", err)
	}

	tc.assertProgramCounter(dest)
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 9 — rangeNextInteger: per-step verification
// ─────────────────────────────────────────────────────────────────────────────

// Test_rangeNextInteger_FullIteration verifies a complete for-range loop over
// an integer count of 3 (indices 0, 1, 2).
func Test_rangeNextInteger_FullIteration(t *testing.T) {
	const dest = 99

	tc := newTestContext(t).
		withBytecodeSize(dest + 1).
		withStack(3)

	if err := rangeInitByteCode(tc.ctx, []any{"i", ""}); err != nil {
		t.Fatalf("rangeInitByteCode: %v", err)
	}

	for want := 0; want < 3; want++ {
		pcBefore := tc.ctx.programCounter

		if err := rangeNextByteCode(tc.ctx, dest); err != nil {
			t.Fatalf("step %d: rangeNextByteCode error: %v", want, err)
		}

		if tc.ctx.programCounter != pcBefore {
			t.Errorf("step %d: PC changed before exhaustion", want)
		}

		tc.assertSymbolValue("i", want)
	}

	// Exhaustion (4th call).
	if err := rangeNextByteCode(tc.ctx, dest); err != nil {
		t.Fatalf("exhaustion step: %v", err)
	}

	tc.assertProgramCounter(dest)

	if len(tc.ctx.rangeStack) != 0 {
		t.Errorf("rangeStack not empty after integer exhaustion")
	}
}

// Test_rangeNextInteger_ZeroRange verifies that a range of 0 exhausts
// immediately without executing any iterations.
func Test_rangeNextInteger_ZeroRange(t *testing.T) {
	const dest = 20

	tc := newTestContext(t).
		withBytecodeSize(dest + 1).
		withStack(0)

	if err := rangeInitByteCode(tc.ctx, []any{"i", ""}); err != nil {
		t.Fatalf("rangeInitByteCode: %v", err)
	}

	if err := rangeNextByteCode(tc.ctx, dest); err != nil {
		t.Fatalf("rangeNextByteCode: %v", err)
	}

	tc.assertProgramCounter(dest)
}

// Test_rangeNextInteger_NegativeRange verifies that a negative count exhausts
// immediately (no iterations).
func Test_rangeNextInteger_NegativeRange(t *testing.T) {
	const dest = 20

	tc := newTestContext(t).
		withBytecodeSize(dest + 1).
		withStack(-5)

	if err := rangeInitByteCode(tc.ctx, []any{"i", ""}); err != nil {
		t.Fatalf("rangeInitByteCode: %v", err)
	}

	if err := rangeNextByteCode(tc.ctx, dest); err != nil {
		t.Fatalf("rangeNextByteCode: %v", err)
	}

	tc.assertProgramCounter(dest)
}

// Test_rangeNextInteger_DiscardedIndex verifies the RANGE-1 fix: when the index
// variable is discarded ("_"), rangeNextInteger must skip the symbol write rather
// than failing with ErrUnknownSymbol.
//
// RANGE-1 fix: added the same guard present in all other rangeNext* helpers:
//
//	if r.indexName != "" && r.indexName != defs.DiscardedVariable {
//	    err = c.symbols.Set(r.indexName, r.index)
//	}
//
// rangeInitByteCode already skips creating a symbol when the name is "_", so the
// unconditional Set call was always going to fail for discarded index names.
func Test_rangeNextInteger_DiscardedIndex_RANGE1(t *testing.T) {
	const dest = 20

	tc := newTestContext(t).
		withBytecodeSize(dest + 1).
		withStack(2) // two iterations

	// Use the discarded variable for the index, no value variable.
	if err := rangeInitByteCode(tc.ctx, []any{defs.DiscardedVariable, ""}); err != nil {
		t.Fatalf("rangeInitByteCode: %v", err)
	}

	// RANGE-1 fix: both steps must succeed; "_" is silently skipped.
	for step := 0; step < 2; step++ {
		if err := rangeNextByteCode(tc.ctx, dest); err != nil {
			t.Fatalf("RANGE-1: step %d failed with: %v", step, err)
		}
	}

	// Third call: exhaustion — PC must jump to destination.
	if err := rangeNextByteCode(tc.ctx, dest); err != nil {
		t.Fatalf("exhaustion step: %v", err)
	}

	tc.assertProgramCounter(dest)
}

// Test_rangeNextInteger_EmptyIndexName verifies the RANGE-1 fix for the empty
// index name ("") case.  An empty name means "no index variable declared"; the
// iterator should advance silently without attempting any symbol write.
func Test_rangeNextInteger_EmptyIndexName_RANGE1(t *testing.T) {
	const dest = 20

	tc := newTestContext(t).withBytecodeSize(dest + 1)

	// Push a range entry with empty index name directly (bypassing
	// rangeInitByteCode, which parses its own operand).
	tc.ctx.rangeStack = append(tc.ctx.rangeStack, &rangeDefinition{
		indexName:  "",
		valueName:  "",
		value:      1, // integer range of 1: one iteration then exhaust
		scopeDepth: tc.ctx.blockDepth,
	})

	// RANGE-1 fix: must succeed (no symbol Set attempted).
	if err := rangeNextByteCode(tc.ctx, dest); err != nil {
		t.Fatalf("RANGE-1 (empty name): step 0 failed: %v", err)
	}

	// Exhaustion.
	if err := rangeNextByteCode(tc.ctx, dest); err != nil {
		t.Fatalf("RANGE-1 (empty name): exhaustion step failed: %v", err)
	}

	tc.assertProgramCounter(dest)
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 10 — Nested ranges
// ─────────────────────────────────────────────────────────────────────────────

// Test_rangeNested verifies that two for-range loops can be active
// simultaneously (one outer, one inner) by checking that the rangeStack grows
// and shrinks correctly, and that each loop's variables are independent.
func Test_rangeNested(t *testing.T) {
	const outerDest = 90

	const innerDest = 80

	tc := newTestContext(t).withBytecodeSize(100)

	// ── Outer loop: string "AB" ───────────────────────────────────────────
	tc.ctx.stack[tc.ctx.stackPointer] = "AB"
	tc.ctx.stackPointer++

	if err := rangeInitByteCode(tc.ctx, []any{"outer", "outerVal"}); err != nil {
		t.Fatalf("outer rangeInitByteCode: %v", err)
	}

	if len(tc.ctx.rangeStack) != 1 {
		t.Fatalf("rangeStack = %d after outer init, want 1", len(tc.ctx.rangeStack))
	}

	// ── First outer step: outer = 0 ──────────────────────────────────────
	if err := rangeNextByteCode(tc.ctx, outerDest); err != nil {
		t.Fatalf("outer step 0: %v", err)
	}

	tc.assertSymbolValue("outer", 0)

	// ── Inner loop: array [10, 20] ────────────────────────────────────────
	innerArr := data.NewArrayFromInterfaces(data.IntType, 10, 20)
	tc.ctx.stack[tc.ctx.stackPointer] = innerArr
	tc.ctx.stackPointer++

	if err := rangeInitByteCode(tc.ctx, []any{"inner", "innerVal"}); err != nil {
		t.Fatalf("inner rangeInitByteCode: %v", err)
	}

	if len(tc.ctx.rangeStack) != 2 {
		t.Fatalf("rangeStack = %d after inner init, want 2", len(tc.ctx.rangeStack))
	}

	// ── Inner step 0: inner = 0, innerVal = 10 ───────────────────────────
	if err := rangeNextByteCode(tc.ctx, innerDest); err != nil {
		t.Fatalf("inner step 0: %v", err)
	}

	tc.assertSymbolValue("inner", 0)
	tc.assertSymbolValue("innerVal", 10)

	// ── Inner step 1: inner = 1, innerVal = 20 ───────────────────────────
	if err := rangeNextByteCode(tc.ctx, innerDest); err != nil {
		t.Fatalf("inner step 1: %v", err)
	}

	tc.assertSymbolValue("inner", 1)
	tc.assertSymbolValue("innerVal", 20)

	// ── Inner exhaustion ─────────────────────────────────────────────────
	if err := rangeNextByteCode(tc.ctx, innerDest); err != nil {
		t.Fatalf("inner exhaustion: %v", err)
	}

	tc.assertProgramCounter(innerDest)

	if len(tc.ctx.rangeStack) != 1 {
		t.Fatalf("rangeStack = %d after inner exhaustion, want 1", len(tc.ctx.rangeStack))
	}

	// Outer variable must be unaffected.
	tc.assertSymbolValue("outer", 0)
}
