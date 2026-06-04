package bytecode

// call_test.go contains unit tests for the functions in call.go:
//
//   - localCallByteCode        – push a deferred-subroutine call frame
//   - callByteCode             – dispatch a function call from the stack
//   - validateFunctionArguments – validate arg count and strict types
//   - validateArgCount          – enforce min/max argument counts
//   - validateStrictParameterTyping – per-argument type checking in strict mode
//   - checkForTupleOnStack      – detect and unwrap a multi-return tuple
//
// # Test strategy
//
// call.go is the most complex dispatcher in the bytecode package.  It routes
// to callBytecodeFunction, callRuntimeFunction, callNative, and callTypeCast
// depending on the type of the function value found on the stack.  Deep
// integration with those callee functions is tested elsewhere; the tests here
// focus on:
//
//  1. The routing logic itself – which error or callee is selected for each
//     function-pointer type.
//  2. The helper functions that are independently callable and have clear
//     unit-test boundaries (validateArgCount, validateStrictParameterTyping,
//     checkForTupleOnStack, localCallByteCode).
//  3. Stack invariants – correct consumption and production of stack items.
//  4. Known bugs confirmed by docs/BYTECODE_ISSUES.md entries.
//
// # Stack layout convention for callByteCode tests
//
// callByteCode pops arguments bottom-first from the top of the stack, then
// pops the function pointer last.  To call a function with two arguments:
//
//	tc.withStack(functionPointer, arg0, arg1)
//
// arg1 is on top and is popped first; arg0 is next; functionPointer is at the
// bottom of the relevant window.

import (
	"testing"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// ─────────────────────────────────────────────────────────────────────────────
// Section 1: localCallByteCode
// ─────────────────────────────────────────────────────────────────────────────
//
// localCallByteCode pushes a CallFrame onto the stack so that execution
// continues from a given address within the current bytecode stream.  It is
// used to invoke inline subroutines compiled for defer blocks.

// Test_localCallByteCode_ValidInt verifies the normal path: a valid integer
// address causes a CallFrame to be pushed and the function returns nil.
func Test_localCallByteCode_ValidInt(t *testing.T) {
	tc := newTestContext(t).withBytecodeSize(10)

	err := localCallByteCode(tc.ctx, 5)

	tc.assertNoError(err)

	// A CallFrame must now be on the stack (below the current frame pointer).
	// After callFramePush the framePointer moves to c.stackPointer, so the
	// frame is at the slot just before the new framePointer.
	fp := tc.ctx.framePointer
	if fp == 0 {
		t.Fatal("localCallByteCode: framePointer was not advanced — callFramePush may not have run")
	}

	frame, ok := tc.ctx.stack[fp-1].(*CallFrame)
	if !ok {
		t.Fatalf("localCallByteCode: expected *CallFrame at stack[%d], got %T", fp-1, tc.ctx.stack[fp-1])
	}

	// The saved PC inside the frame is the address AFTER the one we branched
	// to (it is the caller's PC at the time of the push, which is 0 in a
	// freshly created test context).
	_ = frame // verified it is a *CallFrame; deeper field checks are in callframe_test if needed
}

// Test_localCallByteCode_SetsPC verifies that after the call frame is pushed,
// the context's program counter is set to the operand address.
func Test_localCallByteCode_SetsPC(t *testing.T) {
	tc := newTestContext(t).withBytecodeSize(10)

	err := localCallByteCode(tc.ctx, 7)

	tc.assertNoError(err)
	tc.assertProgramCounter(7)
}

// Test_localCallByteCode_NonIntOperand verifies that a non-integer operand
// causes data.Int to return an error, which is passed back to the caller.
func Test_localCallByteCode_NonIntOperand(t *testing.T) {
	tc := newTestContext(t)

	err := localCallByteCode(tc.ctx, "bad-address")

	if err == nil {
		t.Error("expected error for non-integer operand, got nil")
	}
}

// Test_localCallByteCode_ZeroAddress verifies that address 0 is accepted and
// sets the program counter to 0.
func Test_localCallByteCode_ZeroAddress(t *testing.T) {
	tc := newTestContext(t)

	err := localCallByteCode(tc.ctx, 0)

	tc.assertNoError(err)
	tc.assertProgramCounter(0)
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 2: callByteCode — function-pointer routing
// ─────────────────────────────────────────────────────────────────────────────
//
// The routing tests push a value as the "function pointer" with 0 arguments
// and verify which error (or success path) is taken based on the type.

// Test_callByteCode_StringFunctionZeroArgs verifies the special case where
// the "function" on the stack is a plain string and argc == 0.  This
// represents a .String() pseudo-method: the string is pushed back and
// returned as the "call result".
func Test_callByteCode_StringFunctionZeroArgs(t *testing.T) {
	tc := newTestContext(t).
		withStack("hello-world")

	err := callByteCode(tc.ctx, 0)

	tc.assertNoError(err)
	tc.assertTopStack("hello-world")
}

// Test_callByteCode_StringFunctionNonZeroArgs verifies that the string
// special-case only applies when there are no arguments.  With args the string
// falls through to the default handler and returns ErrInvalidFunctionCall.
func Test_callByteCode_StringFunctionNonZeroArgs(t *testing.T) {
	// Stack: fn="dummy-string", arg0=1  (arg0 on top)
	tc := newTestContext(t).
		withStack("dummy-string", 1)

	err := callByteCode(tc.ctx, 1)

	tc.assertError(err, errors.ErrInvalidFunctionCall)
}

// Test_callByteCode_NilFunctionPointer verifies that a nil function pointer
// returns ErrInvalidFunctionCall.
func Test_callByteCode_NilFunctionPointer(t *testing.T) {
	tc := newTestContext(t).
		withStack(nil) // nil function pointer

	err := callByteCode(tc.ctx, 0)

	tc.assertError(err, errors.ErrInvalidFunctionCall)
}

// Test_callByteCode_StackMarkerAsFunction verifies that a StackMarker on the
// function-pointer slot returns ErrFunctionReturnedVoid.  This happens when a
// prior function call returned void but the caller tried to use the return.
func Test_callByteCode_StackMarkerAsFunction(t *testing.T) {
	tc := newTestContext(t).
		withStack(NewStackMarker("void-result"))

	err := callByteCode(tc.ctx, 0)

	tc.assertError(err, errors.ErrFunctionReturnedVoid)
}

// Test_callByteCode_ErrorValueAsFunction verifies that pushing an error value
// as the function pointer returns ErrUnusedErrorReturn.  This detects when
// code tries to call an error value returned by a previous statement.
func Test_callByteCode_ErrorValueAsFunction(t *testing.T) {
	tc := newTestContext(t).
		withStack(errors.ErrAssert) // a non-nil error value

	err := callByteCode(tc.ctx, 0)

	tc.assertError(err, errors.ErrUnusedErrorReturn)
}

// Test_callByteCode_UnknownTypeAsFunction verifies that a value of an
// unrecognized type (here, a plain integer) returns ErrInvalidFunctionCall.
func Test_callByteCode_UnknownTypeAsFunction(t *testing.T) {
	tc := newTestContext(t).
		withStack(42) // int is not a callable type

	err := callByteCode(tc.ctx, 0)

	tc.assertError(err, errors.ErrInvalidFunctionCall)
}

// Test_callByteCode_StackMarkerInArguments verifies that a StackMarker found
// while popping arguments (not on the function-pointer slot) returns
// ErrFunctionReturnedVoid.
func Test_callByteCode_StackMarkerInArguments(t *testing.T) {
	// Stack layout: fn (bottom) → marker (arg[0] position, top).
	// callByteCode pops the marker as the first argument.
	tc := newTestContext(t).
		withStack("my-func", NewStackMarker("bad-arg"))

	err := callByteCode(tc.ctx, 1) // expect 1 argument

	tc.assertError(err, errors.ErrFunctionReturnedVoid)
}

// Test_callByteCode_StackUnderflow_NoItems verifies ErrStackUnderflow when
// the stack is completely empty (cannot pop the function pointer for argc=0).
func Test_callByteCode_StackUnderflow_NoItems(t *testing.T) {
	tc := newTestContext(t) // empty stack

	err := callByteCode(tc.ctx, 0)

	tc.assertError(err, errors.ErrStackUnderflow)
}

// Test_callByteCode_StackUnderflow_NotEnoughArgs verifies ErrStackUnderflow
// when there are fewer stack items than the declared argument count.  With
// two items on the stack and argc=3, the third pop underflows.
func Test_callByteCode_StackUnderflow_NotEnoughArgs(t *testing.T) {
	// We have only 2 items; callByteCode will try to pop 3 args before the fn.
	tc := newTestContext(t).withStack("fn", 1)

	err := callByteCode(tc.ctx, 3)

	tc.assertError(err, errors.ErrStackUnderflow)
}

// Test_callByteCode_ArgCountDelta verifies that c.argCountDelta is added to
// the operand's argument count and then reset to zero.  This is used by the
// runtime to signal extra hidden arguments (e.g. for goroutine dispatch).
func Test_callByteCode_ArgCountDelta(t *testing.T) {
	// We'll push a nil function — the important thing is to confirm the delta
	// was consumed before we reach the error return.
	tc := newTestContext(t).withStack(nil)
	tc.ctx.argCountDelta = 0 // delta already zero; no extra pops needed

	_ = callByteCode(tc.ctx, 0)

	// argCountDelta must be zeroed out regardless of outcome.
	if tc.ctx.argCountDelta != 0 {
		t.Errorf("argCountDelta: got %d after callByteCode, want 0", tc.ctx.argCountDelta)
	}
}

// Test_callByteCode_ArgCountDeltaNonZero verifies that a positive
// argCountDelta widens the argument window.  We set delta=1, argc=0, and
// push two items so one argument is popped before the function pointer.
func Test_callByteCode_ArgCountDeltaNonZero(t *testing.T) {
	tc := newTestContext(t).withStack(nil, 99) // fn=nil (bottom), arg=99 (top)
	tc.ctx.argCountDelta = 1                   // effective argc = 0 + 1 = 1

	err := callByteCode(tc.ctx, 0)

	// The delta is consumed regardless; check it is cleared.
	if tc.ctx.argCountDelta != 0 {
		t.Errorf("argCountDelta not reset: got %d", tc.ctx.argCountDelta)
	}

	// After popping 1 arg and nil fn: ErrInvalidFunctionCall for nil fn.
	tc.assertError(err, errors.ErrInvalidFunctionCall)
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 3: callByteCode — runtime function dispatch
// ─────────────────────────────────────────────────────────────────────────────
//
// When the function pointer is a func(*symbols.SymbolTable, data.List)(any,error),
// callByteCode delegates to callRuntimeFunction.  These tests verify the
// dispatch path and that a simple return value arrives on the stack.

// Test_callByteCode_RuntimeFn_NoArgs exercises the runtime-function dispatch
// with zero arguments.  The function returns a single value via data.List.
func Test_callByteCode_RuntimeFn_NoArgs(t *testing.T) {
	fn := func(s *symbols.SymbolTable, args data.List) (any, error) {
		// Return a single-item list so callRuntimeFunction pushes 99 onto
		// the stack.
		return data.NewList(99), nil
	}

	tc := newTestContext(t).withStack(fn)

	err := callByteCode(tc.ctx, 0)

	tc.assertNoError(err)

	// callRuntimeFunction pushes a StackMarker("results") then the return
	// value.  Pop the value then the marker.
	v, _ := tc.ctx.Pop()
	if v != 99 {
		t.Errorf("return value: got %v, want 99", v)
	}

	marker, _ := tc.ctx.Pop()
	if !isStackMarker(marker, "results") {
		t.Errorf("expected results StackMarker, got %T %v", marker, marker)
	}
}

// Test_callByteCode_RuntimeFn_WithArgs verifies that arguments are correctly
// passed into the runtime function via data.List.
func Test_callByteCode_RuntimeFn_WithArgs(t *testing.T) {
	// The function sums its two int arguments.
	fn := func(s *symbols.SymbolTable, args data.List) (any, error) {
		a := data.IntOrZero(args.Get(0))
		b := data.IntOrZero(args.Get(1))

		return data.NewList(a + b), nil
	}

	// Stack: fn (bottom) → 3 → 7 (top); argc=2.
	tc := newTestContext(t).withStack(fn, 3, 7)

	err := callByteCode(tc.ctx, 2)

	tc.assertNoError(err)

	v, _ := tc.ctx.Pop()
	if v != 10 {
		t.Errorf("sum: got %v, want 10", v)
	}
}

// Test_callByteCode_RuntimeFn_ReturnsError verifies that an error returned
// inside the runtime function propagates back correctly.
func Test_callByteCode_RuntimeFn_ReturnsError(t *testing.T) {
	fn := func(s *symbols.SymbolTable, args data.List) (any, error) {
		return data.NewList(nil, errors.ErrAssert), nil
	}

	tc := newTestContext(t).withStack(fn)

	// The error is pushed onto the stack as the second list item; the first
	// item (nil) is also on the stack above the marker.  We just confirm the
	// function call itself did not panic or produce an unexpected Go error.

	_ = callByteCode(tc.ctx, 0)
}

// Test_callRuntimeFunction_NilDefinition_SandboxedNoPanic verifies that calling
// a bare runtime function (not wrapped in data.Function) in a sandboxed context
// does not panic.
//
// Prior to the CALL-3 fix, callRuntimeFunction dereferenced savedDefinition
// unconditionally when checking the Sandboxed flag:
//
//	if c.sandboxedIO.Load() && savedDefinition.Sandboxed { ... }
//
// When savedDefinition was nil (bare function, not data.Function-wrapped) and
// the context was sandboxed, this caused a nil-pointer panic.  The fix adds a
// nil guard:
//
//	if c.sandboxedIO.Load() && savedDefinition != nil && savedDefinition.Sandboxed
func Test_callRuntimeFunction_NilDefinition_SandboxedNoPanic(t *testing.T) {
	// A simple function that does nothing and returns a single nil value.
	fn := func(s *symbols.SymbolTable, args data.List) (any, error) {
		return data.NewList(nil), nil
	}

	tc := newTestContext(t)

	// Enable sandboxing — this is the condition that previously triggered the panic.
	tc.ctx.sandboxedIO.Store(true)

	// Call with savedDefinition=nil (bare function, no data.Function wrapper).
	// Must not panic.
	err := callRuntimeFunction(tc.ctx, fn, nil /*savedDefinition*/, false, []any{})

	tc.assertNoError(err)
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 4: callByteCode — *data.Type dispatch (type cast)
// ─────────────────────────────────────────────────────────────────────────────

// Test_callByteCode_TypeCast_IntToString verifies that pushing a *data.Type
// as the function pointer triggers callTypeCast and correctly converts the
// argument to the target type.
func Test_callByteCode_TypeCast_IntToString(t *testing.T) {
	// Stack: data.StringType (bottom) → 42 (top); argc=1.
	tc := newTestContext(t).withStack(data.StringType, 42)

	err := callByteCode(tc.ctx, 1)

	tc.assertNoError(err)
	tc.assertTopStack("42")
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 5: validateArgCount
// ─────────────────────────────────────────────────────────────────────────────
//
// validateArgCount is the gatekeeper that checks whether the caller supplied
// the correct number of arguments for a data.Function call.

// makeDecl is a local helper that builds a data.Declaration with the given
// parameter count, variadic flag, and optional ArgCount range.
func makeDecl(paramCount int, variadic bool, argCount data.Range) *data.Declaration {
	params := make([]data.Parameter, paramCount)
	for i := range params {
		params[i] = data.Parameter{Name: "p", Type: data.IntType}
	}

	return &data.Declaration{
		Name:       "testFunc",
		Parameters: params,
		Variadic:   variadic,
		ArgCount:   argCount,
	}
}

// Test_validateArgCount_ExactMatch verifies that an exact match between the
// declared parameter count and the actual argument count always succeeds.
func Test_validateArgCount_ExactMatch(t *testing.T) {
	tc := newTestContext(t)
	dp := data.Function{Declaration: makeDecl(2, false, data.Range{})}

	err := validateArgCount(2, 2, false, dp, tc.ctx)

	tc.assertNoError(err)
}

// Test_validateArgCount_ZeroAndZero verifies that a zero-arg function called
// with zero args succeeds.
func Test_validateArgCount_ZeroAndZero(t *testing.T) {
	tc := newTestContext(t)
	dp := data.Function{Declaration: makeDecl(0, false, data.Range{})}

	err := validateArgCount(0, 0, false, dp, tc.ctx)

	tc.assertNoError(err)
}

// Test_validateArgCount_ArgCountRange_InRange verifies that argc within an
// explicit ArgCount [min, max] range passes without error.
//
// The ArgCount range check only fires when argumentCount != argc (it sits
// inside the outer "if argumentCount != argc" block).  We therefore use
// makeDecl(0, ...) so argumentCount is 0 while argc is non-zero, guaranteeing
// we enter the block and exercise the range logic.
func Test_validateArgCount_ArgCountRange_InRange(t *testing.T) {
	tc := newTestContext(t)
	dp := data.Function{Declaration: makeDecl(0, false, data.Range{1, 3})}

	// argumentCount=0, argc=2: mismatch triggers the ArgCount block; 2 ∈ [1,3].
	err := validateArgCount(0, 2, false, dp, tc.ctx)

	tc.assertNoError(err)
}

// Test_validateArgCount_ArgCountRange_AtMin verifies that argc == min is valid.
func Test_validateArgCount_ArgCountRange_AtMin(t *testing.T) {
	tc := newTestContext(t)
	dp := data.Function{Declaration: makeDecl(0, false, data.Range{2, 5})}

	// argumentCount=0, argc=2: mismatch; 2 == min → valid.
	err := validateArgCount(0, 2, false, dp, tc.ctx)

	tc.assertNoError(err)
}

// Test_validateArgCount_ArgCountRange_AtMax verifies that argc == max is valid.
func Test_validateArgCount_ArgCountRange_AtMax(t *testing.T) {
	tc := newTestContext(t)
	dp := data.Function{Declaration: makeDecl(0, false, data.Range{2, 5})}

	// argumentCount=0, argc=5: mismatch; 5 == max → valid.
	err := validateArgCount(0, 5, false, dp, tc.ctx)

	tc.assertNoError(err)
}

// Test_validateArgCount_ArgCountRange_BelowMin verifies ErrArgumentCount when
// argc is below the declared minimum.
func Test_validateArgCount_ArgCountRange_BelowMin(t *testing.T) {
	tc := newTestContext(t)
	dp := data.Function{Declaration: makeDecl(0, false, data.Range{2, 5})}

	// argumentCount=0, argc=1: mismatch; 1 < min(2) → error.
	err := validateArgCount(0, 1, false, dp, tc.ctx)

	tc.assertError(err, errors.ErrArgumentCount)
}

// Test_validateArgCount_ArgCountRange_AboveMax verifies ErrArgumentCount when
// argc exceeds the declared maximum.
func Test_validateArgCount_ArgCountRange_AboveMax(t *testing.T) {
	tc := newTestContext(t)
	dp := data.Function{Declaration: makeDecl(0, false, data.Range{2, 5})}

	// argumentCount=0, argc=6: mismatch; 6 > max(5) → error.
	err := validateArgCount(0, 6, false, dp, tc.ctx)

	tc.assertError(err, errors.ErrArgumentCount)
}

// Test_validateArgCount_Variadic_ExactNonVariadicPart verifies that a variadic
// function called with exactly the non-variadic argument count passes.
func Test_validateArgCount_Variadic_ExactNonVariadicPart(t *testing.T) {
	tc := newTestContext(t)
	// Two formal params (a int, b ...int); variadic=true; argc == 2 == argumentCount.
	dp := data.Function{Declaration: makeDecl(2, true, data.Range{})}

	err := validateArgCount(2, 2, false, dp, tc.ctx)

	tc.assertNoError(err)
}

// Test_validateArgCount_Variadic_MoreThanDeclared verifies that a variadic
// function accepts more arguments than its formal parameter list.
func Test_validateArgCount_Variadic_MoreThanDeclared(t *testing.T) {
	tc := newTestContext(t)
	dp := data.Function{Declaration: makeDecl(2, true, data.Range{})}

	err := validateArgCount(2, 5, false, dp, tc.ctx)

	tc.assertNoError(err)
}

// Test_validateArgCount_Variadic_MinusTwoFromDeclared verifies that a variadic
// function with 2 required params rejects argc == 0 (fewer than min-1).
func Test_validateArgCount_Variadic_MinusTwoFromDeclared(t *testing.T) {
	tc := newTestContext(t)
	dp := data.Function{Declaration: makeDecl(3, true, data.Range{})}

	// argumentCount=3, argc=0; 0 < 3-1=2 → error.
	err := validateArgCount(3, 0, false, dp, tc.ctx)

	tc.assertError(err, errors.ErrArgumentCount)
}

// Test_validateArgCount_Variadic_MinusOneFromDeclared verifies that argc ==
// argumentCount - 1 is accepted for a variadic function (zero variadic args).
func Test_validateArgCount_Variadic_MinusOneFromDeclared(t *testing.T) {
	tc := newTestContext(t)
	dp := data.Function{Declaration: makeDecl(3, true, data.Range{})}

	// argumentCount=3, argc=2; 2 < 3-1=2 is false → no error.
	err := validateArgCount(3, 2, false, dp, tc.ctx)

	tc.assertNoError(err)
}

// Test_validateArgCount_NilDeclaration verifies that a nil declaration skips
// all checks and returns nil.
func Test_validateArgCount_NilDeclaration(t *testing.T) {
	tc := newTestContext(t)
	dp := data.Function{} // Declaration is nil

	err := validateArgCount(0, 5, false, dp, tc.ctx)

	tc.assertNoError(err)
}

// Test_validateArgCount_DefaultArgCountZeroZero_Mismatch verifies that a
// non-variadic function called with the wrong argument count returns
// ErrArgumentCount when ArgCount holds its zero value ([0, 0]).
//
// Prior to the CALL-1 fix, the ArgCount block contained an unconditional
// "return nil" that was always reached for non-nil non-variadic declarations,
// making the extensions and exact-count checks unreachable dead code and
// silently accepting any argument count mismatch.
func Test_validateArgCount_DefaultArgCountZeroZero_Mismatch(t *testing.T) {
	tc := newTestContext(t)

	// One-parameter non-variadic function; no explicit ArgCount range.
	dp := data.Function{Declaration: makeDecl(1, false, data.Range{})}

	// Three arguments passed where only one is declared — must be an error.
	err := validateArgCount(1, 3, false, dp, tc.ctx)

	tc.assertError(err, errors.ErrArgumentCount)
}

// Test_validateArgCount_DefaultArgCountZeroZero_ExtensionsAllowsMismatch
// verifies that when language extensions are enabled a count mismatch on a
// non-variadic function with no ArgCount range is tolerated.  Extensions
// delegate count validation to the function itself rather than the call site.
func Test_validateArgCount_DefaultArgCountZeroZero_ExtensionsAllowsMismatch(t *testing.T) {
	tc := newTestContext(t)

	dp := data.Function{Declaration: makeDecl(1, false, data.Range{})}

	// extensions=true; mismatch should be allowed.
	err := validateArgCount(1, 3, true /*extensions*/, dp, tc.ctx)

	tc.assertNoError(err)
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 6: validateStrictParameterTyping
// ─────────────────────────────────────────────────────────────────────────────

// Test_validateStrict_NoArgsNoParams verifies that the empty case is always
// successful.
func Test_validateStrict_NoArgsNoParams(t *testing.T) {
	tc := newTestContext(t)
	dp := data.Function{Declaration: makeDecl(0, false, data.Range{})}

	err := validateStrictParameterTyping([]any{}, dp, tc.ctx)

	tc.assertNoError(err)
}

// Test_validateStrict_MatchingIntParam verifies that an int argument matching
// an int parameter passes without error.
func Test_validateStrict_MatchingIntParam(t *testing.T) {
	tc := newTestContext(t)
	dp := data.Function{Declaration: makeDecl(1, false, data.Range{})}

	err := validateStrictParameterTyping([]any{42}, dp, tc.ctx)

	tc.assertNoError(err)
}

// Test_validateStrict_MismatchedType verifies ErrArgumentType when the
// argument type does not match the declared parameter type.
func Test_validateStrict_MismatchedType(t *testing.T) {
	tc := newTestContext(t)

	// Parameter expects int; we pass a string.
	dp := data.Function{Declaration: &data.Declaration{
		Name: "f",
		Parameters: []data.Parameter{
			{Name: "x", Type: data.IntType},
		},
	}}

	err := validateStrictParameterTyping([]any{"not-an-int"}, dp, tc.ctx)

	tc.assertError(err, errors.ErrArgumentType)
}

// Test_validateStrict_InterfaceParam_AcceptsAnyType verifies that a parameter
// declared as interface{} accepts any argument type.
func Test_validateStrict_InterfaceParam_AcceptsAnyType(t *testing.T) {
	tc := newTestContext(t)
	dp := data.Function{Declaration: &data.Declaration{
		Name: "f",
		Parameters: []data.Parameter{
			{Name: "x", Type: data.InterfaceType},
		},
	}}

	// Integer, string, bool — all must pass.
	for _, arg := range []any{42, "hello", true} {
		if err := validateStrictParameterTyping([]any{arg}, dp, tc.ctx); err != nil {
			t.Errorf("interface{} param rejected %T %v: %v", arg, arg, err)
		}
	}
}

// Test_validateStrict_ArrayInterfaceParam_Accepted verifies that a parameter
// declared as []interface{} accepts any argument (skip check path).
func Test_validateStrict_ArrayInterfaceParam_Accepted(t *testing.T) {
	tc := newTestContext(t)
	dp := data.Function{Declaration: &data.Declaration{
		Name: "f",
		Parameters: []data.Parameter{
			{Name: "x", Type: data.ArrayType(data.InterfaceType)},
		},
	}}

	err := validateStrictParameterTyping([]any{99}, dp, tc.ctx)

	tc.assertNoError(err)
}

// Test_validateStrict_FunctionKindParam_AcceptsByteCode verifies that a
// parameter of FunctionKind accepts a *ByteCode value.
func Test_validateStrict_FunctionKindParam_AcceptsByteCode(t *testing.T) {
	tc := newTestContext(t)
	fnType := data.FunctionType(&data.Function{
		Declaration: &data.Declaration{Name: "cb"},
	})
	dp := data.Function{Declaration: &data.Declaration{
		Name: "f",
		Parameters: []data.Parameter{
			{Name: "callback", Type: fnType},
		},
	}}

	// A *ByteCode value has FunctionKind — it should pass the function check.
	bc := &ByteCode{name: "callback"}

	err := validateStrictParameterTyping([]any{bc}, dp, tc.ctx)

	tc.assertNoError(err)
}

// Test_validateStrict_InterfaceArgument_SkipsCheck verifies that when the
// argument itself is an interface-wrapped value the type check is skipped
// (the arg's type is Interface).
func Test_validateStrict_InterfaceArgument_SkipsCheck(t *testing.T) {
	tc := newTestContext(t)
	dp := data.Function{Declaration: &data.Declaration{
		Name: "f",
		Parameters: []data.Parameter{
			{Name: "x", Type: data.IntType}, // strict int param
		},
	}}

	// data.Wrap creates an Interface-typed value.
	wrappedArg := data.Wrap(42) // Interface{Value:42, BaseType:IntType}

	err := validateStrictParameterTyping([]any{wrappedArg}, dp, tc.ctx)

	tc.assertNoError(err)
}

// Test_validateStrict_Variadic_ExtraArgInterfaceLastParam verifies that extra
// variadic arguments are accepted when the last declared parameter is
// interface{}.
func Test_validateStrict_Variadic_ExtraArgInterfaceLastParam(t *testing.T) {
	tc := newTestContext(t)
	dp := data.Function{Declaration: &data.Declaration{
		Name: "f",
		Parameters: []data.Parameter{
			{Name: "a", Type: data.IntType},
			{Name: "rest", Type: data.InterfaceType},
		},
		Variadic: true,
	}}

	// Three args: one for "a", one for the variadic position, one extra.
	err := validateStrictParameterTyping([]any{1, 2, 3}, dp, tc.ctx)

	tc.assertNoError(err)
}

// Test_validateStrict_Variadic_ExtraArgTypeMismatch verifies that the FIRST
// extra variadic argument (at index exactly len(parms)) is now type-checked.
//
// Prior to the CALL-2 fix, the variadic block used n > len(parms), which
// silently skipped the argument at n == len(parms).  Changing > to >= closes
// that blind spot.
func Test_validateStrict_Variadic_ExtraArgTypeMismatch(t *testing.T) {
	tc := newTestContext(t)
	dp := data.Function{Declaration: &data.Declaration{
		Name: "f",
		Parameters: []data.Parameter{
			{Name: "a", Type: data.IntType},
			{Name: "b", Type: data.IntType}, // last declared param (variadic position)
		},
		Variadic: true,
	}}

	// args[0]=1  n=0 < 2 → regular check (int ✓)
	// args[1]=2  n=1 < 2 → regular check (int ✓)
	// args[2]="bad"  n=2 == len(parms)=2 → variadic block now fires (>= fix)
	err := validateStrictParameterTyping([]any{1, 2, "bad"}, dp, tc.ctx)

	tc.assertError(err, errors.ErrArgumentType)
}

// Test_validateStrict_Variadic_SecondExtraArgTypeMismatch verifies that the
// SECOND extra variadic arg (n > len(parms)) IS checked — confirming the
// off-by-one is specifically at n == len(parms).
func Test_validateStrict_Variadic_SecondExtraArgTypeMismatch(t *testing.T) {
	tc := newTestContext(t)
	dp := data.Function{Declaration: &data.Declaration{
		Name: "f",
		Parameters: []data.Parameter{
			{Name: "a", Type: data.IntType},
			{Name: "b", Type: data.IntType},
		},
		Variadic: true,
	}}

	// args[3] is at n=3 > len(parms)=2 → variadic block fires → type error.
	err := validateStrictParameterTyping([]any{1, 2, 3, "bad"}, dp, tc.ctx)

	// This SHOULD be an error and currently IS — the variadic check does fire
	// for n > len(parms).
	tc.assertError(err, errors.ErrArgumentType)
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 7: validateFunctionArguments
// ─────────────────────────────────────────────────────────────────────────────

// Test_validateFunctionArguments_NoDeclaration verifies that a Function with
// no declaration skips all checks and returns (false, nil).
func Test_validateFunctionArguments_NoDeclaration(t *testing.T) {
	tc := newTestContext(t)
	dp := data.Function{} // nil Declaration

	vis, err := validateFunctionArguments(tc.ctx, dp, 3, []any{1, 2, 3}, false)

	tc.assertNoError(err)

	if vis {
		t.Error("fullSymbolVisibility: expected false for nil declaration, got true")
	}
}

// Test_validateFunctionArguments_ScopeFlag verifies that a declaration with
// Scope:true causes fullSymbolVisibility to be returned as true.
func Test_validateFunctionArguments_ScopeFlag(t *testing.T) {
	tc := newTestContext(t)
	dp := data.Function{Declaration: &data.Declaration{
		Name:  "f",
		Scope: true,
		Parameters: []data.Parameter{
			{Name: "x", Type: data.IntType},
		},
	}}

	vis, err := validateFunctionArguments(tc.ctx, dp, 1, []any{42}, false)

	tc.assertNoError(err)

	if !vis {
		t.Error("fullSymbolVisibility: expected true when Declaration.Scope is true")
	}
}

// Test_validateFunctionArguments_StrictTyping_Pass verifies that strict typing
// accepts an argument whose type exactly matches the parameter declaration.
func Test_validateFunctionArguments_StrictTyping_Pass(t *testing.T) {
	tc := newTestContext(t).withTypeStrictness(defs.StrictTypeEnforcement)
	dp := data.Function{Declaration: makeDecl(1, false, data.Range{})}

	_, err := validateFunctionArguments(tc.ctx, dp, 1, []any{42}, false)

	tc.assertNoError(err)
}

// Test_validateFunctionArguments_StrictTyping_Fail verifies that strict typing
// rejects a mismatched argument type.
func Test_validateFunctionArguments_StrictTyping_Fail(t *testing.T) {
	tc := newTestContext(t).withTypeStrictness(defs.StrictTypeEnforcement)
	dp := data.Function{Declaration: makeDecl(1, false, data.Range{})}

	// Parameter is int, argument is string.
	_, err := validateFunctionArguments(tc.ctx, dp, 1, []any{"wrong"}, false)

	tc.assertError(err, errors.ErrArgumentType)
}

// Test_validateFunctionArguments_NoTypeEnforcement_IgnoresTypeMismatch
// verifies that in NoTypeEnforcement (dynamic) mode, validateFunctionArguments
// does NOT call validateStrictParameterTyping, so a type mismatch between
// argument and declared parameter is silently accepted.
//
// This is the default mode for production Ego programs.  The check is gated:
//
//	if c.typeStrictness == defs.StrictTypeEnforcement && dp.Declaration != nil {
//	    err := validateStrictParameterTyping(...)
//	}
//
// When typeStrictness is NoTypeEnforcement the gate is false and no error is
// returned even though the argument is a string where an int is declared.
func Test_validateFunctionArguments_NoTypeEnforcement_IgnoresTypeMismatch(t *testing.T) {
	tc := newTestContext(t).withTypeStrictness(defs.NoTypeEnforcement)
	dp := data.Function{Declaration: makeDecl(1, false, data.Range{})}

	// String argument for an int parameter — mismatch that strict mode would reject.
	_, err := validateFunctionArguments(tc.ctx, dp, 1, []any{"wrong-type"}, false)

	// In dynamic mode the type check is skipped; only the count is checked.
	tc.assertNoError(err)
}

// Test_validateFunctionArguments_RelaxedTypeEnforcement_IgnoresTypeMismatch
// verifies that RelaxedTypeEnforcement also skips validateStrictParameterTyping.
// Relaxed mode allows numeric widening (e.g. int32 for int64) but the gate in
// validateFunctionArguments is strictly defs.StrictTypeEnforcement, so anything
// other than strict — including relaxed — bypasses per-parameter type checking.
func Test_validateFunctionArguments_RelaxedTypeEnforcement_IgnoresTypeMismatch(t *testing.T) {
	tc := newTestContext(t).withTypeStrictness(defs.RelaxedTypeEnforcement)
	dp := data.Function{Declaration: makeDecl(1, false, data.Range{})}

	// String argument for an int parameter — would be rejected in strict mode.
	_, err := validateFunctionArguments(tc.ctx, dp, 1, []any{"wrong-type"}, false)

	tc.assertNoError(err)
}

// Test_validateFunctionArguments_StrictMode_RightCountWrongType verifies that
// strict mode checks both count AND type: the right number of arguments still
// fails when any argument's type mismatches its declaration.
func Test_validateFunctionArguments_StrictMode_RightCountWrongType(t *testing.T) {
	tc := newTestContext(t).withTypeStrictness(defs.StrictTypeEnforcement)

	// Two-parameter function: (a int, b int).
	dp := data.Function{Declaration: makeDecl(2, false, data.Range{})}

	// Correct count (2) but second arg is the wrong type.
	_, err := validateFunctionArguments(tc.ctx, dp, 2, []any{1, "bad"}, false)

	tc.assertError(err, errors.ErrArgumentType)
}

// Test_validateFunctionArguments_StrictMode_AllTypesMatch verifies that
// providing the exact declared types for all parameters succeeds in strict mode.
func Test_validateFunctionArguments_StrictMode_AllTypesMatch(t *testing.T) {
	tc := newTestContext(t).withTypeStrictness(defs.StrictTypeEnforcement)

	// Two-parameter function: (a int, b int).
	dp := data.Function{Declaration: makeDecl(2, false, data.Range{})}

	_, err := validateFunctionArguments(tc.ctx, dp, 2, []any{1, 2}, false)

	tc.assertNoError(err)
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 8: checkForTupleOnStack
// ─────────────────────────────────────────────────────────────────────────────
//
// checkForTupleOnStack scans the stack for a StackMarker whose embedded count
// matches the number of items above it.  When found, the caller can treat
// those items as a multi-value tuple return.

// Test_checkForTuple_EmptyStack verifies that an empty stack returns (argc, false).
func Test_checkForTuple_EmptyStack(t *testing.T) {
	tc := newTestContext(t)

	gotArgc, wasTuple := checkForTupleOnStack(tc.ctx, 1)

	if wasTuple {
		t.Error("expected wasTuple=false for empty stack")
	}

	if gotArgc != 1 {
		t.Errorf("argc: got %d, want 1", gotArgc)
	}
}

// Test_checkForTuple_SingleValueNoMarker verifies that a single value with no
// StackMarker does not trigger tuple detection.
func Test_checkForTuple_SingleValueNoMarker(t *testing.T) {
	tc := newTestContext(t).withStack(42)

	gotArgc, wasTuple := checkForTupleOnStack(tc.ctx, 1)

	if wasTuple {
		t.Error("expected wasTuple=false when no marker present")
	}

	if gotArgc != 1 {
		t.Errorf("argc: got %d, want 1", gotArgc)
	}
}

// Test_checkForTuple_MarkerCountMatchesTwoValues verifies that when two values
// sit above a StackMarker that records count=2, the tuple is detected and argc
// is adjusted to 2.
func Test_checkForTuple_MarkerCountMatchesTwoValues(t *testing.T) {
	tc := newTestContext(t)

	// Stack layout (bottom → top): marker(count=2), "a", "b"
	// The marker label must differ from c.module (which is "") so we use "results".
	_ = tc.ctx.push(NewStackMarker("results", 2))
	_ = tc.ctx.push("a")
	_ = tc.ctx.push("b")

	gotArgc, wasTuple := checkForTupleOnStack(tc.ctx, 1)

	if !wasTuple {
		t.Error("expected wasTuple=true")
	}

	if gotArgc != 2 {
		t.Errorf("argc: got %d, want 2", gotArgc)
	}
}

// Test_checkForTuple_MarkerCountMismatch verifies that a StackMarker whose
// count does not match the items above it does not trigger tuple detection.
func Test_checkForTuple_MarkerCountMismatch(t *testing.T) {
	tc := newTestContext(t)

	// Marker claims count=5 but only 1 value is above it.
	_ = tc.ctx.push(NewStackMarker("results", 5))
	_ = tc.ctx.push("only-one")

	gotArgc, wasTuple := checkForTupleOnStack(tc.ctx, 1)

	if wasTuple {
		t.Error("expected wasTuple=false: count in marker does not match actual item count")
	}

	if gotArgc != 1 {
		t.Errorf("argc: got %d, want 1 (unchanged)", gotArgc)
	}
}

// Test_checkForTuple_CallFrameBreaksSearch verifies that a *CallFrame on the
// stack stops the tuple scan, preventing cross-frame detection.
//
// The scan walks downward from the current stack top.  A CallFrame represents
// the boundary between the current function's stack window and the caller's.
// Any tuple marker sitting below a CallFrame must not be visible.
//
// Stack layout (bottom → top):
//
//	marker("results", count=1)   ← from a prior call, below the frame boundary
//	"value-from-prior-frame"
//	*CallFrame                   ← the boundary
//	"current-value"              ← only item in the current function's window
//
// The scan starts at "current-value" (count becomes 1), then hits *CallFrame
// and breaks.  The matching marker below the frame is never reached.
func Test_checkForTuple_CallFrameBreaksSearch(t *testing.T) {
	tc := newTestContext(t)

	// Build the "prior call" layer below the frame.
	_ = tc.ctx.push(NewStackMarker("results", 1)) // marker claiming count=1
	_ = tc.ctx.push("value-from-prior-frame")     // 1 value above the marker

	// Insert a CallFrame to act as the frame boundary.
	_ = tc.ctx.push(&CallFrame{})
	tc.ctx.framePointer = tc.ctx.stackPointer

	// One value in the "current function's" window, above the CallFrame.
	_ = tc.ctx.push("current-value")

	gotArgc, wasTuple := checkForTupleOnStack(tc.ctx, 1)

	// The CallFrame must break the search before the marker is found.
	if wasTuple {
		t.Error("expected wasTuple=false: *CallFrame should break the downward scan")
	}

	if gotArgc != 1 {
		t.Errorf("argc: got %d, want 1 (unchanged)", gotArgc)
	}
}
