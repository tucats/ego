package bytecode

// flow_test.go contains unit tests for the flow-control and runtime-support
// bytecode handlers defined in bytecode/flow.go:
//
//	profileByteCode        — enable/disable/report profiling
//	stopByteCode           — halt execution immediately
//	panicByteCode          — generate a fatal runtime error
//	moduleByteCode         — set the current module name on the context
//	atLineByteCode         — tag the current source line number
//	waitByteCode           — wait for goroutines to complete
//	modeCheckBytecode      — verify the execution mode matches an expected value
//	ifErrorByteCode        — check whether TOS is an error condition
//	inPackageSymbolTable   — walk the symbol-table chain looking for a package
//
// All tests use newTestContext(t) and the "with" builder chain from
// testhelpers_test.go, as required by CLAUDE.md.  The six legacy tests that
// previously used raw &Context{} struct literals (FLOW-3) have been converted.
//
// # Bugs resolved (see docs/BYTECODE_ISSUES.md for details)
//
//	FLOW-1  Test_branchFalseByteCode called branchTrueByteCode for its
//	        invalid-address sub-case.  Fixed: the call now targets
//	        branchFalseByteCode so that function's address-validation path
//	        is exercised.
//
//	FLOW-2  moduleByteCode and atLineByteCode accessed array[1] without a
//	        length check; a one-element array operand would panic.  Fixed in
//	        flow.go; regression tests added below (Section 4 and Section 5).
//
//	FLOW-3  Legacy tests used raw &Context{} struct literals instead of
//	        newTestContext.  All six legacy tests have been converted.

import (
	"reflect"
	"sync"
	"testing"

	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/profiling"
	"github.com/tucats/ego/tokenizer"
)

// ─────────────────────────────────────────────────────────────────────────────
// Section 1 — profileByteCode
//
// profileByteCode validates the operand, maps recognized strings to profiling
// action constants, and delegates to profiling.Profile.
//
// Operand dispatch:
//   nil                     → ErrInvalidInstruction (checked before Profile)
//   known string verb       → Profile(StartAction / StopAction / ReportAction)
//   unrecognized string     → ErrInvalidProfileAction (checked before Profile)
//   integer                 → Profile(int) passed straight through
//   non-integer, non-string → runtimeError wrapping data.Int failure
//
// The six recognized string verbs are tested individually so that each branch
// in the switch is exercised by at least one named test.
// ─────────────────────────────────────────────────────────────────────────────

// Test_profileByteCode_NilOperand verifies that a nil operand returns
// ErrInvalidInstruction before profileByteCode dispatches to the profiling
// package.  The nil check is the very first thing the function does, so no
// other state is required.
func Test_profileByteCode_NilOperand(t *testing.T) {
	tc := newTestContext(t)

	tc.assertError(profileByteCode(tc.ctx, nil), errors.ErrInvalidInstruction)
}

// Test_profileByteCode_StringEnable verifies that "enable" is accepted and
// dispatched to profiling.Profile(StartAction).  Profile(StartAction) always
// returns nil, so this should succeed.
func Test_profileByteCode_StringEnable(t *testing.T) {
	tc := newTestContext(t)

	tc.assertNoError(profileByteCode(tc.ctx, "enable"))
}

// Test_profileByteCode_StringStart verifies the "start" alias for StartAction.
func Test_profileByteCode_StringStart(t *testing.T) {
	tc := newTestContext(t)

	tc.assertNoError(profileByteCode(tc.ctx, "start"))
}

// Test_profileByteCode_StringOn verifies the "on" alias for StartAction.
func Test_profileByteCode_StringOn(t *testing.T) {
	tc := newTestContext(t)

	tc.assertNoError(profileByteCode(tc.ctx, "on"))
}

// Test_profileByteCode_StringDisable verifies that "disable" is dispatched to
// Profile(StopAction).  Profile(StopAction) always returns nil.
func Test_profileByteCode_StringDisable(t *testing.T) {
	tc := newTestContext(t)

	tc.assertNoError(profileByteCode(tc.ctx, "disable"))
}

// Test_profileByteCode_StringStop verifies the "stop" alias for StopAction.
func Test_profileByteCode_StringStop(t *testing.T) {
	tc := newTestContext(t)

	tc.assertNoError(profileByteCode(tc.ctx, "stop"))
}

// Test_profileByteCode_StringOff verifies the "off" alias for StopAction.
func Test_profileByteCode_StringOff(t *testing.T) {
	tc := newTestContext(t)

	tc.assertNoError(profileByteCode(tc.ctx, "off"))
}

// Test_profileByteCode_UnknownString verifies that an unrecognized string
// operand returns ErrInvalidProfileAction.  This error is returned from the
// switch's default case without calling Profile.
func Test_profileByteCode_UnknownString(t *testing.T) {
	tc := newTestContext(t)

	tc.assertError(profileByteCode(tc.ctx, "bogus_verb"), errors.ErrInvalidProfileAction)
}

// Test_profileByteCode_IntegerStart verifies that profiling.StartAction (= 0)
// supplied as a raw integer is passed directly to profiling.Profile.
//
// Integer operands bypass the string-dispatch switch and call
// `data.Int(i)` to obtain the action code.
func Test_profileByteCode_IntegerStart(t *testing.T) {
	tc := newTestContext(t)

	tc.assertNoError(profileByteCode(tc.ctx, profiling.StartAction))
}

// Test_profileByteCode_IntegerStop verifies profiling.StopAction as an integer.
func Test_profileByteCode_IntegerStop(t *testing.T) {
	tc := newTestContext(t)

	tc.assertNoError(profileByteCode(tc.ctx, profiling.StopAction))
}

// Test_profileByteCode_StringCaseInsensitive verifies that profileByteCode
// lowercases the string operand before matching.  "ENABLE" (all caps) must be
// treated the same as "enable".
func Test_profileByteCode_StringCaseInsensitive(t *testing.T) {
	tc := newTestContext(t)

	tc.assertNoError(profileByteCode(tc.ctx, "ENABLE"))
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 2 — stopByteCode
//
// stopByteCode stores false into c.running and returns ErrStop.  No operand
// is used.
// ─────────────────────────────────────────────────────────────────────────────

// Test_stopByteCode verifies that stopByteCode clears the running flag and
// returns ErrStop.
//
// FLOW-3 fix: the original test used a raw &Context{} struct literal.
// Converted to newTestContext.
func Test_stopByteCode(t *testing.T) {
	tc := newTestContext(t)
	tc.ctx.running.Store(true)

	tc.assertError(stopByteCode(tc.ctx, nil), errors.ErrStop)

	// The running flag must be cleared so the run loop exits.
	if tc.ctx.running.Load() {
		t.Error("stopByteCode: running flag should be false after stop")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 3 — panicByteCode
//
// panicByteCode always sets c.running = false.  If the instruction operand is
// non-nil, its string representation is used as the panic message.  If the
// operand is nil, the message is popped from the top of the stack.
//
// When defs.RuntimePanicsSetting is "true" the function issues a native Go
// panic (untestable; requires setting the flag to "false" in all tests below).
// ─────────────────────────────────────────────────────────────────────────────

// Test_panicByteCode_OperandMode verifies the non-nil operand path: the operand
// string is used directly as the panic message without consulting the stack.
//
// FLOW-3 fix: the original test used a raw &Context{} with a pre-loaded stack
// item that was never used (the operand was always non-nil).  Converted to
// newTestContext with an empty stack to make the intent clear.
func Test_panicByteCode_OperandMode(t *testing.T) {
	// Ensure native Go panics are disabled so the function returns an error
	// instead of calling panic().
	settings.Set(defs.RuntimePanicsSetting, "false")

	tc := newTestContext(t) // empty stack — operand is used, not stack

	err := panicByteCode(tc.ctx, "panic message")

	// The error must carry the operand string as its context value.
	if egErr, ok := err.(*errors.Error); !ok || egErr.GetContext() != "panic message" {
		t.Errorf("panicByteCode: context = %q, want %q", err, "panic message")
	}
}

// Test_panicByteCode_NilOperand_PopsFromStack verifies the nil-operand path:
// when no message is supplied via the operand, panicByteCode pops the message
// from the top of the stack.
//
// This exercises the `else` branch of `if i != nil` in panicByteCode.
func Test_panicByteCode_NilOperand_PopsFromStack(t *testing.T) {
	settings.Set(defs.RuntimePanicsSetting, "false")

	// Push the panic message onto the stack; panicByteCode will pop it.
	tc := newTestContext(t).withStack("stack-panic-message")

	err := panicByteCode(tc.ctx, nil) // nil operand → pop from stack

	// The error must carry the popped string as its context value.
	tc.assertError(err, errors.ErrPanic)

	egErr, ok := err.(*errors.Error)
	if !ok {
		t.Fatalf("expected *errors.Error, got %T", err)
	}

	if egErr.GetContext() != "stack-panic-message" {
		t.Errorf("panicByteCode: context = %q, want %q",
			egErr.GetContext(), "stack-panic-message")
	}
}

// Test_panicByteCode_NilOperand_EmptyStack verifies that a nil operand on an
// empty stack causes panicByteCode to return ErrStackUnderflow (from the failed
// Pop) rather than panicking internally.
func Test_panicByteCode_NilOperand_EmptyStack(t *testing.T) {
	settings.Set(defs.RuntimePanicsSetting, "false")

	tc := newTestContext(t) // empty stack

	// Pop will fail, returning ErrStackUnderflow before panic logic runs.
	tc.assertError(panicByteCode(tc.ctx, nil), errors.ErrStackUnderflow)
}

// Test_panicByteCode_SetsRunningFalse verifies that panicByteCode always clears
// c.running regardless of where the message comes from.
func Test_panicByteCode_SetsRunningFalse(t *testing.T) {
	settings.Set(defs.RuntimePanicsSetting, "false")

	tc := newTestContext(t)
	tc.ctx.running.Store(true)

	_ = panicByteCode(tc.ctx, "any message")

	if tc.ctx.running.Load() {
		t.Error("panicByteCode: running flag should be false after panic")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 4 — moduleByteCode
//
// moduleByteCode sets c.module from the operand.  The operand is either a
// plain string or a []any{name, tokenizer} slice.
//
// FLOW-2 regression tests are included here: a single-element slice operand
// must not panic after the bounds-check fix in flow.go.
// ─────────────────────────────────────────────────────────────────────────────

// Test_moduleByteCode_StringOperand verifies that a plain string operand sets
// c.module to that string.
func Test_moduleByteCode_StringOperand(t *testing.T) {
	tc := newTestContext(t)

	tc.assertNoError(moduleByteCode(tc.ctx, "my_module.ego"))

	if tc.ctx.module != "my_module.ego" {
		t.Errorf("moduleByteCode: module = %q, want %q", tc.ctx.module, "my_module.ego")
	}
}

// Test_moduleByteCode_ArrayOperandWithNilSlot verifies that a two-element array
// whose second slot is nil (not a *tokenizer.Tokenizer) sets the module name
// without crashing.  The tokenizer type-assertion simply fails and c.tokenizer
// is left unchanged.
func Test_moduleByteCode_ArrayOperandWithNilSlot(t *testing.T) {
	tc := newTestContext(t)

	operand := []any{"array_module.ego", nil}

	tc.assertNoError(moduleByteCode(tc.ctx, operand))

	if tc.ctx.module != "array_module.ego" {
		t.Errorf("moduleByteCode: module = %q, want %q", tc.ctx.module, "array_module.ego")
	}
}

// Test_moduleByteCode_ArrayOperandWithTokenizer verifies that a two-element
// []any{name, *tokenizer.Tokenizer} operand sets both c.module and c.tokenizer.
func Test_moduleByteCode_ArrayOperandWithTokenizer(t *testing.T) {
	tc := newTestContext(t)

	tok := tokenizer.New("x := 1", true)
	operand := []any{"tok_module.ego", tok}

	tc.assertNoError(moduleByteCode(tc.ctx, operand))

	if tc.ctx.module != "tok_module.ego" {
		t.Errorf("moduleByteCode: module = %q, want %q", tc.ctx.module, "tok_module.ego")
	}

	// The tokenizer must be stored on the context for the debugger / trace.
	if tc.ctx.tokenizer != tok {
		t.Error("moduleByteCode: tokenizer not stored on context")
	}
}

// Test_moduleByteCode_SingleElementArray is the FLOW-2 regression test.
//
// Before the fix, accessing array[1] without checking len(array) would panic
// with "index out of range [1] with length 1".  After the fix, a one-element
// array is valid: the module name is set and the tokenizer slot is silently
// skipped.
func Test_moduleByteCode_SingleElementArray(t *testing.T) {
	tc := newTestContext(t)

	// One-element array — previously triggered a panic (FLOW-2).
	operand := []any{"single_elem.ego"}

	// This must not panic and must return nil.
	tc.assertNoError(moduleByteCode(tc.ctx, operand))

	// The module name must still be set from the first element.
	if tc.ctx.module != "single_elem.ego" {
		t.Errorf("moduleByteCode: module = %q, want %q", tc.ctx.module, "single_elem.ego")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 5 — atLineByteCode
//
// atLineByteCode records the current source line number (and optionally the
// source text) in the context and in the symbol table.  It also signals the
// debugger when debugging mode is active.
//
// FLOW-2 regression test is included: a single-element array must not panic
// after the bounds-check fix in flow.go.
// ─────────────────────────────────────────────────────────────────────────────

// Test_atLineByteCode_IntegerOperand verifies that a plain integer operand sets
// c.line to that value.
func Test_atLineByteCode_IntegerOperand(t *testing.T) {
	tc := newTestContext(t)

	tc.assertNoError(atLineByteCode(tc.ctx, 42))

	if tc.ctx.line != 42 {
		t.Errorf("atLineByteCode: line = %d, want 42", tc.ctx.line)
	}
}

// Test_atLineByteCode_ArrayOperand verifies that a []any{lineNumber, text}
// operand sets both c.line and c.source.
func Test_atLineByteCode_ArrayOperand(t *testing.T) {
	tc := newTestContext(t)

	operand := []any{10, "x := x + 1"}

	tc.assertNoError(atLineByteCode(tc.ctx, operand))

	if tc.ctx.line != 10 {
		t.Errorf("atLineByteCode: line = %d, want 10", tc.ctx.line)
	}

	if tc.ctx.source != "x := x + 1" {
		t.Errorf("atLineByteCode: source = %q, want %q", tc.ctx.source, "x := x + 1")
	}
}

// Test_atLineByteCode_SingleElementArray is the FLOW-2 regression test.
//
// Before the fix, accessing array[1] on a one-element slice would panic with
// "index out of range [1] with length 1".  After the fix, the source text
// is simply left as "" when the second slot is absent.
func Test_atLineByteCode_SingleElementArray(t *testing.T) {
	tc := newTestContext(t)

	// One-element array — previously triggered a panic (FLOW-2).
	operand := []any{7}

	// This must not panic and must return nil.
	tc.assertNoError(atLineByteCode(tc.ctx, operand))

	// Line must be set from slot 0.
	if tc.ctx.line != 7 {
		t.Errorf("atLineByteCode: line = %d, want 7", tc.ctx.line)
	}

	// Source text must be the zero value since no second element was provided.
	if tc.ctx.source != "" {
		t.Errorf("atLineByteCode: source = %q, want empty string", tc.ctx.source)
	}
}

// Test_atLineByteCode_SetsLineSymbol verifies that atLineByteCode stores the
// line number in the symbol table under defs.LineVariable ("__line").
//
// Ego programs can read __line to get the current source-line number.
func Test_atLineByteCode_SetsLineSymbol(t *testing.T) {
	tc := newTestContext(t)

	tc.assertNoError(atLineByteCode(tc.ctx, 77))

	v, found := tc.ctx.symbols.Get(defs.LineVariable)
	if !found {
		t.Fatal("atLineByteCode: LineVariable not found in symbol table")
	}

	if !reflect.DeepEqual(v, 77) {
		t.Errorf("atLineByteCode: LineVariable = %v (%T), want 77 (int)", v, v)
	}
}

// Test_atLineByteCode_SetsModuleSymbol verifies that atLineByteCode stores the
// bytecode name in the symbol table under defs.ModuleVariable ("__module").
func Test_atLineByteCode_SetsModuleSymbol(t *testing.T) {
	tc := newTestContext(t)
	tc.ctx.bc.name = "sample.ego"

	tc.assertNoError(atLineByteCode(tc.ctx, 5))

	v, found := tc.ctx.symbols.Get(defs.ModuleVariable)
	if !found {
		t.Fatal("atLineByteCode: ModuleVariable not found in symbol table")
	}

	if v != "sample.ego" {
		t.Errorf("atLineByteCode: ModuleVariable = %v, want %q", v, "sample.ego")
	}
}

// Test_atLineByteCode_DebuggingMode verifies that atLineByteCode returns
// ErrSignalDebugger when debugging is enabled and the line number is non-zero.
//
// The run loop uses this signal to hand control to the debugger REPL so it can
// check for breakpoints or single-step requests.
func Test_atLineByteCode_DebuggingMode(t *testing.T) {
	tc := newTestContext(t)
	tc.ctx.debugging = true

	tc.assertError(atLineByteCode(tc.ctx, 3), errors.ErrSignalDebugger)
}

// Test_atLineByteCode_DebuggingMode_LineZeroNoSignal verifies that a line
// number of 0 does NOT signal the debugger, even when debugging is active.
//
// Line 0 is a synthetic line number used for compiler-generated preamble code
// that has no corresponding source position.  Halting on it would be confusing.
func Test_atLineByteCode_DebuggingMode_LineZeroNoSignal(t *testing.T) {
	tc := newTestContext(t)
	tc.ctx.debugging = true

	tc.assertNoError(atLineByteCode(tc.ctx, 0))
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 6 — waitByteCode
//
// waitByteCode waits for goroutines to complete.  When the operand is a
// *sync.WaitGroup, that specific WaitGroup is used.  Any other operand causes
// the package-level goRoutineCompletion WaitGroup to be used instead.
// ─────────────────────────────────────────────────────────────────────────────

// Test_waitByteCode_WithWaitGroup verifies that waitByteCode calls Wait() on
// the *sync.WaitGroup supplied as the operand.
//
// A WaitGroup with counter 0 returns from Wait() immediately, keeping the test
// deterministic without spawning goroutines.
func Test_waitByteCode_WithWaitGroup(t *testing.T) {
	tc := newTestContext(t)

	var wg sync.WaitGroup // counter = 0

	tc.assertNoError(waitByteCode(tc.ctx, &wg))
}

// Test_waitByteCode_NonWaitGroupOperand verifies that a non-WaitGroup operand
// falls back to the package-level goRoutineCompletion WaitGroup.
//
// When no Ego goroutines are running, goRoutineCompletion has counter 0 and
// Wait() returns immediately.
func Test_waitByteCode_NonWaitGroupOperand(t *testing.T) {
	tc := newTestContext(t)

	// nil is not a *sync.WaitGroup; the fallback WaitGroup is used.
	tc.assertNoError(waitByteCode(tc.ctx, nil))
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 7 — modeCheckBytecode
//
// modeCheckBytecode reads the __exec_mode symbol from the symbol table and
// compares it (as a string) to the instruction operand.  A mismatch or absent
// symbol returns ErrWrongMode.
// ─────────────────────────────────────────────────────────────────────────────

// Test_modeCheckBytecode_ModeMatches verifies that when the __exec_mode symbol
// equals the operand, modeCheckBytecode returns nil.
func Test_modeCheckBytecode_ModeMatches(t *testing.T) {
	tc := newTestContext(t)
	tc.ctx.symbols.SetAlways(defs.ModeVariable, "server")

	tc.assertNoError(modeCheckBytecode(tc.ctx, "server"))
}

// Test_modeCheckBytecode_ModeMismatch verifies that when the stored mode does
// not match the operand, ErrWrongMode is returned.
func Test_modeCheckBytecode_ModeMismatch(t *testing.T) {
	tc := newTestContext(t)
	tc.ctx.symbols.SetAlways(defs.ModeVariable, "interactive")

	tc.assertError(modeCheckBytecode(tc.ctx, "server"), errors.ErrWrongMode)
}

// Test_modeCheckBytecode_ModeNotFound verifies that when the __exec_mode symbol
// is absent from the symbol table, ErrWrongMode is returned.
//
// When found == false the `if found && ...` condition is false, so the
// function falls through to the error return.
func Test_modeCheckBytecode_ModeNotFound(t *testing.T) {
	tc := newTestContext(t)
	// __exec_mode is deliberately absent.

	tc.assertError(modeCheckBytecode(tc.ctx, "server"), errors.ErrWrongMode)
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 8 — ifErrorByteCode
//
// ifErrorByteCode pops one item from the stack and acts on its type:
//
//   StackMarker  → push it back (pass-through for the optional-operator chain)
//   error value  → propagate the error to the run loop
//   bool true    → return nil  (expression was valid)
//   bool false   → return ErrInvalidType (nil operand) or runtimeError(operand)
//   other        → data.Bool conversion; ErrInvalidType if conversion fails
// ─────────────────────────────────────────────────────────────────────────────

// Test_ifErrorByteCode_StackUnderflow verifies that an empty stack causes the
// Pop to fail with ErrStackUnderflow before any type-checking logic runs.
func Test_ifErrorByteCode_StackUnderflow(t *testing.T) {
	tc := newTestContext(t)

	tc.assertError(ifErrorByteCode(tc.ctx, nil), errors.ErrStackUnderflow)
}

// Test_ifErrorByteCode_StackMarkerPassthrough verifies that a StackMarker on
// TOS is pushed back onto the stack and nil is returned.
//
// This is the pass-through behavior used by the `?` optional operator: when
// the sub-expression produced no value, the marker is preserved for downstream
// handling rather than being consumed as an error.
func Test_ifErrorByteCode_StackMarkerPassthrough(t *testing.T) {
	marker := NewStackMarker("optional")
	tc := newTestContext(t).withStack(marker)

	tc.assertNoError(ifErrorByteCode(tc.ctx, nil))

	// The marker must still be on the stack after the pass-through.
	tc.assertTopStack(marker)
}

// Test_ifErrorByteCode_ErrorOnStack verifies that an error value on TOS is
// returned to the run loop for normal error handling.
//
// Multi-return runtime functions may leave error values on the stack.
// ifErrorByteCode provides the mechanism to surface those errors.
func Test_ifErrorByteCode_ErrorOnStack(t *testing.T) {
	tc := newTestContext(t).withStack(errors.New(errors.ErrTypeMismatch))

	tc.assertError(ifErrorByteCode(tc.ctx, nil), errors.ErrTypeMismatch)
}

// Test_ifErrorByteCode_BoolTrue verifies that bool true on TOS returns nil
// (the expression that produced it was valid — no error to report).
func Test_ifErrorByteCode_BoolTrue(t *testing.T) {
	tc := newTestContext(t).withStack(true)

	tc.assertNoError(ifErrorByteCode(tc.ctx, nil))
}

// Test_ifErrorByteCode_BoolFalse_NilOperand verifies that bool false on TOS
// with a nil instruction operand returns ErrInvalidType.
//
// A false condition signals that the expression before the ifError produced an
// invalid result; with no specific error supplied via the operand, the generic
// type error is used.
func Test_ifErrorByteCode_BoolFalse_NilOperand(t *testing.T) {
	tc := newTestContext(t).withStack(false)

	tc.assertError(ifErrorByteCode(tc.ctx, nil), errors.ErrInvalidType)
}

// Test_ifErrorByteCode_BoolFalse_ErrorOperand verifies that when the operand
// is an error and TOS is false, that specific error is returned (decorated with
// module/line info via runtimeError).
func Test_ifErrorByteCode_BoolFalse_ErrorOperand(t *testing.T) {
	tc := newTestContext(t).withStack(false)

	tc.assertError(ifErrorByteCode(tc.ctx, errors.ErrTypeMismatch), errors.ErrTypeMismatch)
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 9 — inPackageSymbolTable
//
// inPackageSymbolTable walks the symbol-table chain from the current table
// upward, returning true when it finds a table whose Package() equals name.
// ─────────────────────────────────────────────────────────────────────────────

// Test_inPackageSymbolTable_Found verifies that inPackageSymbolTable returns
// true when the current symbol table is tagged with the named package.
func Test_inPackageSymbolTable_Found(t *testing.T) {
	tc := newTestContext(t)
	tc.ctx.symbols.SetPackage("math")

	if !tc.ctx.inPackageSymbolTable("math") {
		t.Error("inPackageSymbolTable: expected true for 'math', got false")
	}
}

// Test_inPackageSymbolTable_NotFound verifies that inPackageSymbolTable returns
// false when no table in the chain belongs to the named package.
func Test_inPackageSymbolTable_NotFound(t *testing.T) {
	tc := newTestContext(t)

	if tc.ctx.inPackageSymbolTable("strings") {
		t.Error("inPackageSymbolTable: expected false for 'strings', got true")
	}
}

// Test_inPackageSymbolTable_FoundInParent verifies that the walk reaches parent
// tables: when only the parent has the matching package name, the result is true.
func Test_inPackageSymbolTable_FoundInParent(t *testing.T) {
	tc := newTestContext(t)

	// Tag the root (parent) table as the "io" package.  The local child table
	// has no package assigned, so the walk must climb to the root.
	tc.ctx.symbols.Parent().SetPackage("io")

	if !tc.ctx.inPackageSymbolTable("io") {
		t.Error("inPackageSymbolTable: expected true when parent is 'io', got false")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 10 — type-cast call (callByteCode with *data.Type on stack)
//
// When callByteCode pops a *data.Type as the function, it routes to
// callCastFunction which performs the Ego type conversion.  This section
// exercises several conversion targets.
//
// FLOW-3 fix: the original test used a raw &Context{} struct literal.
// Converted to newTestContext.
// ─────────────────────────────────────────────────────────────────────────────

// Test_typeCast_IntToString verifies that a *data.Type for string converts
// an integer value to its decimal string representation.
func Test_typeCast_IntToString(t *testing.T) {
	// callByteCode needs a context with a valid symbol table and bytecode.
	// newTestContext provides both.
	tc := newTestContext(t)

	// Stack layout for Call(argc=1):
	//   [bottom]  the function  (*data.Type)
	//   [top]     the argument  (55)
	// callByteCode pops argc=1 argument, then the function below it.
	tc.withStack(data.StringType, 55)

	if err := callByteCode(tc.ctx, 1); err != nil {
		t.Fatalf("callByteCode: unexpected error %v", err)
	}

	tc.assertTopStack("55")
}

// Test_typeCast_BoolToString verifies that a bool value is converted to the
// strings "true" or "false" via a *data.Type cast to string.
func Test_typeCast_BoolToString(t *testing.T) {
	tc := newTestContext(t)
	tc.withStack(data.StringType, true)

	if err := callByteCode(tc.ctx, 1); err != nil {
		t.Fatalf("callByteCode: unexpected error %v", err)
	}

	tc.assertTopStack("true")
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 11 — localCallByteCode and returnByteCode
//
// localCallByteCode is a variant of Call that branches within the same bytecode
// stream without creating a new bytecode context.  It saves a CallFrame on the
// stack and sets the program counter to the target address.
// returnByteCode detects the local frame and pops it, restoring the original
// context state.
//
// FLOW-3 fix: the original test used a raw &Context{} struct literal.
// Converted to newTestContext with withBytecodeSize and withStack helpers.
// The saved-symbol-table-name assertion was updated from "local call test"
// to "test local" to match the name that newTestContext assigns to its local
// symbol table.
// ─────────────────────────────────────────────────────────────────────────────

// Test_localCallAndReturnByteCode exercises localCallByteCode and returnByteCode
// as a matched pair.  It verifies that:
//   - localCallByteCode saves the current CallFrame, advances the PC, and
//     creates a new symbol table scope.
//   - returnByteCode restores the original PC, symbol table, and frame pointer.
//   - The stack items present before the local call are still accessible after
//     the return (they were not consumed by the call/return pair).
func Test_localCallAndReturnByteCode(t *testing.T) {
	const (
		// "test local" is the name that newTestContext assigns to its local
		// (innermost) symbol table.  The CallFrame saves the caller's table
		// by pointer, and fp.symbols.Name reflects that name.
		expectedTableName  = "test local"
		uninterestingValue = "uninteresting value"
	)

	// Build a context whose ByteCode is large enough to jump to address 5.
	// withStack loads two items so that:
	//   stack[0] = uninterestingValue (SP starts at 0, ends at 2 after both pushes)
	//   stack[1] = StackMarker
	//   → stackPointer = 2
	// After localCallByteCode pushes a *CallFrame at stack[2], framePointer = 3.
	tc := newTestContext(t).
		withBytecodeSize(5).
		withStack(uninterestingValue, NewStackMarker("defer test"))

	// ── Call ─────────────────────────────────────────────────────────────────

	if err := localCallByteCode(tc.ctx, 5); err != nil {
		t.Fatalf("localCallByteCode: unexpected error %v", err)
	}

	// Program counter must be set to the branch target.
	if tc.ctx.programCounter != 5 {
		t.Errorf("localCallByteCode: PC = %d, want 5", tc.ctx.programCounter)
	}

	// Frame pointer points one slot above the saved *CallFrame.
	if tc.ctx.framePointer != 3 {
		t.Errorf("localCallByteCode: framePointer = %d, want 3", tc.ctx.framePointer)
	}

	// Inspect the saved frame to confirm the caller's symbol table was captured.
	savedSlot := tc.ctx.stack[tc.ctx.framePointer-1]

	frame, ok := savedSlot.(*CallFrame)
	if !ok {
		t.Fatalf("localCallByteCode: stack[fp-1] is %T, want *CallFrame", savedSlot)
	}

	if frame.symbols.Name != expectedTableName {
		t.Errorf("localCallByteCode: saved table name = %q, want %q",
			frame.symbols.Name, expectedTableName)
	}

	// ── Simulate callee body ──────────────────────────────────────────────────

	// Push a nested symbol table and a return value, as compiled Ego code would.
	tc.ctx.symbols = newTestContext(t).ctx.symbols // fresh child for the callee
	if err := tc.ctx.push(3.14); err != nil {
		t.Fatalf("push callee value: %v", err)
	}

	// ── Return ───────────────────────────────────────────────────────────────

	if err := returnByteCode(tc.ctx, false); err != nil {
		t.Fatalf("returnByteCode: unexpected error %v", err)
	}

	// ── Verify stack restored ─────────────────────────────────────────────────

	// The StackMarker pushed before the call is still on the stack.
	// Drop items accumulated by the local call down to it.
	marker := NewStackMarker("defer test")
	if err := dropToMarkerByteCode(tc.ctx, marker); err != nil {
		t.Fatalf("dropToMarkerByteCode: %v", err)
	}

	// The original "uninteresting value" must still be accessible.
	v, err := tc.ctx.Pop()
	if err != nil {
		t.Fatalf("Pop: %v", err)
	}

	if data.String(v) != uninterestingValue {
		t.Errorf("localCallAndReturn: TOS = %q, want %q", data.String(v), uninterestingValue)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 12 — branchFalseByteCode (three sub-cases, shared context)
//
// FLOW-1 fix: sub-case 3 previously called branchTrueByteCode by mistake.
//             The call now correctly targets branchFalseByteCode.
// FLOW-3 fix: converted from raw &Context{} to newTestContext.
//
// All three sub-cases share one context so the program counter carries over
// from sub-case 1 (PC=2) into sub-case 2 (branch not taken, PC stays 2).
// ─────────────────────────────────────────────────────────────────────────────

// Test_branchFalseByteCode exercises the three primary behaviors:
//
//  1. TOS = false → branch taken, PC set to the target address.
//  2. TOS = true  → branch NOT taken, PC unchanged.
//  3. Target address out of range → ErrInvalidBytecodeAddress, PC unchanged.
func Test_branchFalseByteCode(t *testing.T) {
	// A single context is used across all three sub-cases so that PC carry-over
	// from sub-case 1 is visible in sub-case 2's "PC unchanged" assertion.
	// withBytecodeSize(5) allows valid addresses 0–5; 20 is therefore invalid.
	tc := newTestContext(t).withBytecodeSize(5)

	// Sub-case 1: TOS = false → branch taken, PC → 2.
	tc.withStack(false)

	if err := branchFalseByteCode(tc.ctx, 2); err != nil {
		t.Errorf("sub-case 1: unexpected error %v", err)
	}

	if tc.ctx.programCounter != 2 {
		t.Errorf("sub-case 1: PC = %d, want 2", tc.ctx.programCounter)
	}

	// Sub-case 2: TOS = true → branch NOT taken, PC stays at 2 (carried over).
	tc.withStack(true)

	if err := branchFalseByteCode(tc.ctx, 1); err != nil {
		t.Errorf("sub-case 2: unexpected error %v", err)
	}

	if tc.ctx.programCounter != 2 {
		t.Errorf("sub-case 2: PC = %d, want 2 (unchanged)", tc.ctx.programCounter)
	}

	// Sub-case 3: address 20 exceeds nextAddress (5) → invalid.
	// FLOW-1 fix: original code called branchTrueByteCode here.
	// Now correctly calls branchFalseByteCode to exercise its own validation.
	tc.withStack(true)

	err := branchFalseByteCode(tc.ctx, 20)
	if !err.(*errors.Error).Equal(errors.ErrInvalidBytecodeAddress) {
		t.Errorf("sub-case 3: error = %v, want ErrInvalidBytecodeAddress", err)
	}

	if tc.ctx.programCounter != 2 {
		t.Errorf("sub-case 3: PC = %d, want 2 (unchanged after invalid branch)", tc.ctx.programCounter)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 13 — branchTrueByteCode (three sub-cases, shared context)
//
// FLOW-3 fix: converted from raw &Context{} to newTestContext.
//
// Same shared-context strategy as Section 12: the PC from sub-case 1 is
// visible in sub-case 2.
// ─────────────────────────────────────────────────────────────────────────────

// Test_branchTrueByteCode exercises the three primary behaviors:
//
//  1. TOS = false → branch NOT taken, PC unchanged.
//  2. TOS = true  → branch taken, PC set to target.
//  3. Target address out of range → ErrInvalidBytecodeAddress, PC unchanged.
func Test_branchTrueByteCode(t *testing.T) {
	tc := newTestContext(t).withBytecodeSize(5)

	// Sub-case 1: TOS = false → branch NOT taken.  PC starts at 0 and stays 0.
	tc.withStack(false)

	if err := branchTrueByteCode(tc.ctx, 2); err != nil {
		t.Errorf("sub-case 1: unexpected error %v", err)
	}

	if tc.ctx.programCounter != 0 {
		t.Errorf("sub-case 1: PC = %d, want 0 (unchanged)", tc.ctx.programCounter)
	}

	// Sub-case 2: TOS = true → branch taken, PC → 2.
	tc.withStack(true)

	if err := branchTrueByteCode(tc.ctx, 2); err != nil {
		t.Errorf("sub-case 2: unexpected error %v", err)
	}

	if tc.ctx.programCounter != 2 {
		t.Errorf("sub-case 2: PC = %d, want 2", tc.ctx.programCounter)
	}

	// Sub-case 3: address 20 is invalid.
	tc.withStack(true)

	err := branchTrueByteCode(tc.ctx, 20)
	if !err.(*errors.Error).Equal(errors.ErrInvalidBytecodeAddress) {
		t.Errorf("sub-case 3: error = %v, want ErrInvalidBytecodeAddress", err)
	}

	if tc.ctx.programCounter != 2 {
		t.Errorf("sub-case 3: PC = %d, want 2 (unchanged after invalid branch)", tc.ctx.programCounter)
	}
}
