package bytecode

// callRuntimeFunction_test.go contains unit tests for the three functions in
// callRuntimeFunction.go:
//
//   - callRuntimeFunction   – the top-level dispatcher for runtime ("builtin")
//                             function calls from Ego code
//   - functionReturnedValueAndError – decides how to handle the result after
//                             a runtime function returns
//   - synthesizeDefinition  – builds a FunctionDefinition from a saved
//                             data.Function when the global dictionary
//                             has no entry for the function
//
// # Architecture overview
//
// Runtime (builtin) functions have the signature:
//
//	func(s *symbols.SymbolTable, args data.List) (any, error)
//
// They differ from native Go functions (math.Abs, etc.) in that they receive
// an Ego symbol table and a data.List rather than individual Go-typed args.
// callRuntimeFunction is the glue between the bytecode run loop and these
// functions.
//
// The call sequence is:
//  1. Look up the function in builtins.FunctionDictionary by pointer equality.
//  2. If not found but a savedDefinition is provided, synthesise a definition.
//  3. Validate the argument count against the definition's min/max.
//  4. Build a fresh child symbol table for the function to use as its scope.
//  5. If a "this" receiver is on the context's receiver stack, inject it.
//  6. If the definition carries a Context flag, append the *Context to args.
//  7. If the context is sandboxed and the function is marked Sandboxed, block.
//  8. Call the function.
//  9. Route the result: data.List → push with "results" marker;
//     HasErrReturn → push marker+err+result; single ErrorKind return → push
//     value only; other → push scalar or propagate error.
//
// # How to read these tests
//
// Each test uses newTestContext (see testhelpers_test.go) to build a context,
// then calls callRuntimeFunction (or one of the helpers) directly with a
// simple anonymous runtime function and verifies the stack/symbol state.

import (
	"testing"

	"github.com/tucats/ego/builtins"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// ─── test-local runtime function helpers ─────────────────────────────────────

// runtimeFnReturning42 is a simple runtime function that ignores its arguments
// and returns the integer 42 wrapped in a data.List.  It is used by tests that
// only care about successful dispatch and result-pushing behaviour.
func runtimeFnReturning42(s *symbols.SymbolTable, args data.List) (any, error) {
	return data.NewList(42, nil), nil
}

// runtimeFnReturningError is a runtime function that returns a non-nil error
// (and no value) wrapped in a data.List.
func runtimeFnReturningError(s *symbols.SymbolTable, args data.List) (any, error) {
	return data.NewList(nil, errors.ErrAssert), nil
}

// runtimeFnEchoFirstArg returns its first argument as the sole result item.
// Tests use this to confirm that arguments are correctly forwarded into the
// function's data.List.
func runtimeFnEchoFirstArg(s *symbols.SymbolTable, args data.List) (any, error) {
	if args.Len() == 0 {
		return data.NewList(nil, errors.ErrArgumentCount), nil
	}

	return data.NewList(args.Get(0), nil), nil
}

// runtimeFnThatInspectsThis reads the __this variable from its own symbol
// table and returns it.  Tests use this to confirm that the receiver ("this")
// is injected into the function's scope when one is present on the receiver
// stack.
func runtimeFnThatInspectsThis(s *symbols.SymbolTable, args data.List) (any, error) {
	if v, ok := s.Get(defs.ThisVariable); ok {
		return data.NewList(v, nil), nil
	}

	return data.NewList(nil, nil), nil
}

// runtimeFnThatCountsArgs returns the number of arguments it received, as an
// integer in a data.List.  This makes it easy to verify that optional context
// injection has (or has not) appended an extra argument.
func runtimeFnThatCountsArgs(s *symbols.SymbolTable, args data.List) (any, error) {
	return data.NewList(args.Len(), nil), nil
}

// runtimeFnScalarResult returns a plain integer (not a data.List) as its
// result.  callRuntimeFunction should push scalar results directly onto the
// stack without a "results" marker.
func runtimeFnScalarResult(s *symbols.SymbolTable, args data.List) (any, error) {
	return 99, nil
}

// runtimeFnGoError returns a non-nil Go error directly (not in a data.List).
// callRuntimeFunction should convert this to a runtimeError and propagate it.
func runtimeFnGoError(s *symbols.SymbolTable, args data.List) (any, error) {
	return nil, errors.ErrAssert
}

// savedDecl builds a *data.Function (savedDefinition) with a given number of
// plain integer parameters and optional variadic/ArgCount settings.  This is
// used when testing the synthesiseDefinition and arg-count-check paths.
func savedDecl(paramCount int, variadic bool, argCount data.Range) *data.Function {
	params := make([]data.Parameter, paramCount)
	for i := range params {
		params[i] = data.Parameter{Name: "p", Type: data.IntType}
	}

	return &data.Function{
		Declaration: &data.Declaration{
			Name:       "testFn",
			Parameters: params,
			Variadic:   variadic,
			ArgCount:   argCount,
		},
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 1: callRuntimeFunction — anonymous functions (no definition)
// ─────────────────────────────────────────────────────────────────────────────
//
// When a runtime function is not registered in builtins.FunctionDictionary
// and no savedDefinition is provided, callRuntimeFunction skips all arg-count
// checks and calls the function directly.  The result is pushed onto the stack.

// Test_callRuntimeFunction_DataListResult verifies the common case: a
// data.List result is exploded onto the stack with a "results" StackMarker at
// the bottom.  The list items are pushed in reverse order so the first element
// (index 0) ends up on top and is popped first.
func Test_callRuntimeFunction_DataListResult(t *testing.T) {
	tc := newTestContext(t)

	err := callRuntimeFunction(tc.ctx, runtimeFnReturning42, nil, false, []any{})

	tc.assertNoError(err)

	// The integer 42 (list index 0) should be on top.
	tc.assertTopStack(42)

	// The nil error (list index 1) was also pushed; pop it.
	tc.assertTopStack(nil)

	// The "results" StackMarker must be at the bottom of the group.
	marker, popErr := tc.ctx.Pop()
	if popErr != nil || !isStackMarker(marker, "results") {
		t.Errorf("expected 'results' StackMarker, got %T %v", marker, marker)
	}
}

// Test_callRuntimeFunction_ScalarResult verifies that when the runtime
// function returns a plain (non-List) value, it is pushed directly without
// any "results" marker.  This path is used by builtin functions that return
// a single value such as an int or string.
func Test_callRuntimeFunction_ScalarResult(t *testing.T) {
	tc := newTestContext(t)

	err := callRuntimeFunction(tc.ctx, runtimeFnScalarResult, nil, false, []any{})

	tc.assertNoError(err)
	tc.assertTopStack(99)
	tc.assertStackEmpty()
}

// Test_callRuntimeFunction_GoErrorPropagated verifies that when the runtime
// function returns a raw Go error (not in a data.List), callRuntimeFunction
// wraps it in a runtimeError and returns it.  Nothing should be pushed onto
// the stack.
func Test_callRuntimeFunction_GoErrorPropagated(t *testing.T) {
	tc := newTestContext(t)

	err := callRuntimeFunction(tc.ctx, runtimeFnGoError, nil, false, []any{})

	tc.assertError(err, errors.ErrAssert)
	tc.assertStackEmpty()
}

// Test_callRuntimeFunction_ArgsForwardedToFunction verifies that the []any
// args slice is correctly packaged into a data.List and forwarded to the
// function.  runtimeFnEchoFirstArg returns its first argument as the result;
// we confirm that result matches what we passed in.
func Test_callRuntimeFunction_ArgsForwardedToFunction(t *testing.T) {
	tc := newTestContext(t)

	err := callRuntimeFunction(tc.ctx, runtimeFnEchoFirstArg, nil, false, []any{"hello"})

	tc.assertNoError(err)
	tc.assertTopStack("hello")
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 2: callRuntimeFunction — argument count validation
// ─────────────────────────────────────────────────────────────────────────────
//
// When a savedDefinition is provided (but the function is not in the global
// dictionary), callRuntimeFunction calls synthesizeDefinition to build
// min/max arg counts, then validates len(args) against those bounds.

// Test_callRuntimeFunction_TooFewArgs verifies that passing fewer arguments
// than the declared minimum returns ErrArgumentCount.
func Test_callRuntimeFunction_TooFewArgs(t *testing.T) {
	tc := newTestContext(t)

	// Function expects 2 args; we supply 0.
	saved := savedDecl(2, false, data.Range{})

	err := callRuntimeFunction(tc.ctx, runtimeFnReturning42, saved, false, []any{})

	tc.assertError(err, errors.ErrArgumentCount)
}

// Test_callRuntimeFunction_TooManyArgs verifies that passing more arguments
// than the declared maximum returns ErrArgumentCount.
func Test_callRuntimeFunction_TooManyArgs(t *testing.T) {
	tc := newTestContext(t)

	// Function expects 1 arg; we supply 3.
	saved := savedDecl(1, false, data.Range{})

	err := callRuntimeFunction(tc.ctx, runtimeFnReturning42, saved, false, []any{1, 2, 3})

	tc.assertError(err, errors.ErrArgumentCount)
}

// Test_callRuntimeFunction_ExactArgCount verifies that providing exactly the
// declared number of arguments passes the count check and the function runs.
func Test_callRuntimeFunction_ExactArgCount(t *testing.T) {
	tc := newTestContext(t)

	// Function expects 1 arg; we supply exactly 1.
	saved := savedDecl(1, false, data.Range{})

	err := callRuntimeFunction(tc.ctx, runtimeFnReturning42, saved, false, []any{42})

	tc.assertNoError(err)
}

// Test_callRuntimeFunction_VariadicAcceptsMoreThanMin verifies that a variadic
// function (synthesized definition) accepts more arguments than the minimum.
func Test_callRuntimeFunction_VariadicAcceptsMoreThanMin(t *testing.T) {
	tc := newTestContext(t)

	// Variadic with 2 params → MinArgCount = 1, MaxArgCount = 99999.
	saved := savedDecl(2, true, data.Range{})

	// Supplying 5 args should be well within the range.
	err := callRuntimeFunction(tc.ctx, runtimeFnReturning42, saved, false,
		[]any{1, 2, 3, 4, 5})

	tc.assertNoError(err)
}

// Test_callRuntimeFunction_ExplicitArgCountRange verifies that an explicit
// ArgCount range (not [0,0]) is honoured by the synthesized definition.
func Test_callRuntimeFunction_ExplicitArgCountRange(t *testing.T) {
	tc := newTestContext(t)

	// ArgCount [2, 4] → accept 2..4 arguments.
	saved := savedDecl(1, false, data.Range{2, 4})

	// 3 args is within range — should succeed.
	err := callRuntimeFunction(tc.ctx, runtimeFnReturning42, saved, false,
		[]any{1, 2, 3})

	tc.assertNoError(err)
}

// Test_callRuntimeFunction_ExplicitArgCountRange_TooFew verifies that an
// explicit minimum is enforced.
func Test_callRuntimeFunction_ExplicitArgCountRange_TooFew(t *testing.T) {
	tc := newTestContext(t)

	// ArgCount [2, 4] → at least 2 required; 1 should fail.
	saved := savedDecl(1, false, data.Range{2, 4})

	err := callRuntimeFunction(tc.ctx, runtimeFnReturning42, saved, false, []any{1})

	tc.assertError(err, errors.ErrArgumentCount)
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 3: callRuntimeFunction — scope and symbol table setup
// ─────────────────────────────────────────────────────────────────────────────

// Test_callRuntimeFunction_FullScopeTrue verifies that when fullScope=true the
// function's symbol table is a child of c.symbols (giving it direct access to
// the caller's local variables).
//
// We confirm this by putting a variable in c.symbols and checking whether the
// function can see it via its own symbol table — but because the function
// already runs in a child scope, we rely instead on verifying that the name
// of the parent table is c.symbols.Name.
func Test_callRuntimeFunction_FullScopeTrue(t *testing.T) {
	tc := newTestContext(t)

	// Capture the current symbol table name so we can identify it later.
	callerTableName := tc.ctx.symbols.Name

	// Store the function's parent table name as the result.
	captureFn := func(s *symbols.SymbolTable, args data.List) (any, error) {
		if s.Parent() != nil {
			return data.NewList(s.Parent().Name, nil), nil
		}

		return data.NewList("no-parent", nil), nil
	}

	err := callRuntimeFunction(tc.ctx, captureFn, nil, true /*fullScope*/, []any{})

	tc.assertNoError(err)

	// When fullScope=true, parentTable = c.symbols, so the function's parent
	// should be the caller's symbol table.
	tc.assertTopStack(callerTableName)
}

// Test_callRuntimeFunction_ReceiverInjectedIntoFunctionScope verifies that
// when a "this" receiver is on the context's receiver stack, it is injected
// into the function's symbol table under defs.ThisVariable ("__this") before
// the function is called.
func Test_callRuntimeFunction_ReceiverInjectedIntoFunctionScope(t *testing.T) {
	tc := newTestContext(t)

	// Push a receiver value so popThis() finds it.
	tc.ctx.pushThis("receiver", "my-receiver-value")

	err := callRuntimeFunction(tc.ctx, runtimeFnThatInspectsThis, nil, false, []any{})

	tc.assertNoError(err)

	// runtimeFnThatInspectsThis returns __this; the value should match what
	// we pushed.
	tc.assertTopStack("my-receiver-value")
}

// Test_callRuntimeFunction_NoReceiverWhenStackEmpty verifies that when the
// receiver stack is empty, __this is NOT injected and the function receives
// an empty symbol table entry for that name.
func Test_callRuntimeFunction_NoReceiverWhenStackEmpty(t *testing.T) {
	tc := newTestContext(t)
	// receiverStack starts empty — no pushThis call.

	err := callRuntimeFunction(tc.ctx, runtimeFnThatInspectsThis, nil, false, []any{})

	tc.assertNoError(err)

	// __this was not set, so the function returned nil for it.
	tc.assertTopStack(nil)
}

// Test_callRuntimeFunction_ContextInjectedWhenFlagSet verifies that when
// savedDefinition.Context == true, the *Context is appended as an extra
// argument to the function call.  runtimeFnThatCountsArgs returns len(args)
// so we can detect whether the extra argument was added.
func Test_callRuntimeFunction_ContextInjectedWhenFlagSet(t *testing.T) {
	tc := newTestContext(t)

	saved := &data.Function{
		Context: true, // tells callRuntimeFunction to inject *Context as last arg
		Declaration: &data.Declaration{
			Name:       "contextFn",
			Parameters: []data.Parameter{},
		},
	}

	// Call with 0 user args. With Context=true, 1 extra arg (*Context) is
	// appended, so the function should see 1 argument in total.
	err := callRuntimeFunction(tc.ctx, runtimeFnThatCountsArgs, saved, false, []any{})

	tc.assertNoError(err)
	tc.assertTopStack(1) // one arg: the injected *Context
}

// Test_callRuntimeFunction_ContextNotInjectedWhenFlagUnset verifies that when
// savedDefinition.Context == false (or savedDefinition is nil), no extra
// argument is appended.
func Test_callRuntimeFunction_ContextNotInjectedWhenFlagUnset(t *testing.T) {
	tc := newTestContext(t)

	// nil savedDefinition → no context injection.
	err := callRuntimeFunction(tc.ctx, runtimeFnThatCountsArgs, nil, false, []any{1, 2})

	tc.assertNoError(err)
	tc.assertTopStack(2) // exactly the two args we passed, no extras
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 4: callRuntimeFunction — FullScope propagation from definition
// ─────────────────────────────────────────────────────────────────────────────

// Test_callRuntimeFunction_DefinitionFullScope verifies that when a
// FunctionDefinition (from the global dictionary or synthesized) has
// FullScope=true, the fullScope flag is promoted to true even if the caller
// passed false.
//
// We register a one-off function in the FunctionDictionary for this test, run
// the call, then remove it so we don't pollute other tests.
func Test_callRuntimeFunction_DefinitionFullScope(t *testing.T) {
	tc := newTestContext(t)
	callerTableName := tc.ctx.symbols.Name

	// A function that captures its own parent table name.
	fullScopeFn := func(s *symbols.SymbolTable, args data.List) (any, error) {
		if s.Parent() != nil {
			return data.NewList(s.Parent().Name, nil), nil
		}

		return data.NewList("no-parent", nil), nil
	}

	// Temporarily register the function with FullScope=true.
	const testKey = "testFullScopeFn"
	builtins.FunctionDictionary[testKey] = builtins.FunctionDefinition{
		Name:            testKey,
		FunctionAddress: fullScopeFn,
		FullScope:       true,
		MinArgCount:     0,
		MaxArgCount:     0,
	}

	defer delete(builtins.FunctionDictionary, testKey)

	// Pass fullScope=false; the definition should override it to true.
	err := callRuntimeFunction(tc.ctx, fullScopeFn, nil, false /*fullScope*/, []any{})

	tc.assertNoError(err)

	// FullScope=true means parentTable = c.symbols, so the function's parent
	// is the caller's symbol table.
	tc.assertTopStack(callerTableName)
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 5: callRuntimeFunction — sandbox enforcement
// ─────────────────────────────────────────────────────────────────────────────

// Test_callRuntimeFunction_SandboxedFunctionBlocked verifies that when the
// context is sandboxed AND savedDefinition.Sandboxed is true, the call is
// rejected with ErrNoPrivilegeForOperation.
func Test_callRuntimeFunction_SandboxedFunctionBlocked(t *testing.T) {
	tc := newTestContext(t)
	tc.ctx.sandboxedIO.Store(true)

	saved := &data.Function{
		Sandboxed: true,
		Declaration: &data.Declaration{Name: "blockedFn"},
	}

	err := callRuntimeFunction(tc.ctx, runtimeFnReturning42, saved, false, []any{})

	tc.assertError(err, errors.ErrNoPrivilegeForOperation)
}

// Test_callRuntimeFunction_SandboxedContextNonSandboxedFunction verifies that
// a non-sandboxed function is allowed even when the context is sandboxed.
func Test_callRuntimeFunction_SandboxedContextNonSandboxedFunction(t *testing.T) {
	tc := newTestContext(t)
	tc.ctx.sandboxedIO.Store(true)

	saved := &data.Function{
		Sandboxed: false, // this function is permitted in sandbox
		Declaration: &data.Declaration{Name: "allowedFn"},
	}

	err := callRuntimeFunction(tc.ctx, runtimeFnReturning42, saved, false, []any{})

	tc.assertNoError(err)
}

// Test_callRuntimeFunction_NilSavedDefNoSandboxPanic verifies that the nil
// guard on savedDefinition prevents a panic when the context is sandboxed but
// savedDefinition is nil (bare function, not wrapped in data.Function).
// This is the CALL-3 fix confirmed in the runtime context.
func Test_callRuntimeFunction_NilSavedDefNoSandboxPanic(t *testing.T) {
	tc := newTestContext(t)
	tc.ctx.sandboxedIO.Store(true)

	// nil savedDefinition: savedDefinition.Sandboxed would panic without guard.
	err := callRuntimeFunction(tc.ctx, runtimeFnReturning42, nil, false, []any{})

	// Should succeed (nil guard short-circuits the sandbox check).
	tc.assertNoError(err)
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 6: functionReturnedValueAndError
// ─────────────────────────────────────────────────────────────────────────────
//
// functionReturnedValueAndError is called after the runtime function runs to
// decide how to route the result based on the function definition's metadata.

// Test_functionReturnedValueAndError_HasErrReturn_NoError verifies the
// HasErrReturn path when there is no error.  Both the result value and the nil
// error are pushed, separated by a "results" marker.  The function returns
// (true, nil) so callRuntimeFunction returns nil to the run loop.
func Test_functionReturnedValueAndError_HasErrReturn_NoError(t *testing.T) {
	tc := newTestContext(t)

	def := &builtins.FunctionDefinition{HasErrReturn: true}

	handled, retErr := functionReturnedValueAndError(def, tc.ctx, nil /*err*/, "my-result")

	if !handled {
		t.Fatal("expected handled=true for HasErrReturn=true")
	}

	if retErr != nil {
		t.Errorf("retErr: got %v, want nil", retErr)
	}

	// Stack (top → bottom): result, nil-error, marker.
	tc.assertTopStack("my-result")
	tc.assertTopStack(nil)

	marker, _ := tc.ctx.Pop()
	if !isStackMarker(marker, "results") {
		t.Errorf("expected 'results' marker, got %T %v", marker, marker)
	}
}

// Test_functionReturnedValueAndError_HasErrReturn_WithError verifies the
// HasErrReturn path when the function returned a non-nil error.  Both the
// result and the error are pushed; the function returns (true, err) so the
// error is propagated to the run loop (making try/catch work).
func Test_functionReturnedValueAndError_HasErrReturn_WithError(t *testing.T) {
	tc := newTestContext(t)

	def := &builtins.FunctionDefinition{HasErrReturn: true}
	testErr := errors.ErrAssert

	handled, retErr := functionReturnedValueAndError(def, tc.ctx, testErr, "partial-result")

	if !handled {
		t.Fatal("expected handled=true")
	}

	// The error must be returned so the run loop can catch it.
	if retErr == nil {
		t.Error("retErr: expected non-nil, got nil")
	}

	// Stack (top → bottom): result, error, marker.
	tc.assertTopStack("partial-result")

	errOnStack, _ := tc.ctx.Pop()
	if errOnStack == nil {
		t.Error("error not pushed onto stack")
	}

	marker, _ := tc.ctx.Pop()
	if !isStackMarker(marker, "results") {
		t.Errorf("expected 'results' marker, got %T %v", marker, marker)
	}
}

// Test_functionReturnedValueAndError_SingleErrorReturn_NoError verifies that
// when the declaration declares a single return of ErrorKind and no error
// occurred, the result is pushed and (true, nil) is returned.
func Test_functionReturnedValueAndError_SingleErrorReturn_NoError(t *testing.T) {
	tc := newTestContext(t)

	def := &builtins.FunctionDefinition{
		Declaration: &data.Declaration{
			Returns: []*data.Type{data.ErrorType},
		},
	}

	handled, retErr := functionReturnedValueAndError(def, tc.ctx, nil /*no error*/, "ok-result")

	if !handled {
		t.Fatal("expected handled=true for single ErrorKind return")
	}

	if retErr != nil {
		t.Errorf("retErr: got %v, want nil", retErr)
	}

	tc.assertTopStack("ok-result")
}

// Test_functionReturnedValueAndError_SingleErrorReturn_WithError verifies that
// when the declaration declares a single ErrorKind return and an error did
// occur, the result is NOT pushed (nothing goes on the stack) and (true, err)
// is returned so the error propagates.
func Test_functionReturnedValueAndError_SingleErrorReturn_WithError(t *testing.T) {
	tc := newTestContext(t)

	def := &builtins.FunctionDefinition{
		Declaration: &data.Declaration{
			Returns: []*data.Type{data.ErrorType},
		},
	}

	handled, retErr := functionReturnedValueAndError(def, tc.ctx, errors.ErrAssert, "unused")

	if !handled {
		t.Fatal("expected handled=true")
	}

	if retErr == nil {
		t.Error("retErr: expected non-nil (the error)")
	}

	// Nothing must be on the stack — the error is the return value, not the
	// result.
	tc.assertStackEmpty()
}

// Test_functionReturnedValueAndError_MultipleReturns_NotHandled verifies that
// when the declaration has more than one return type, this function returns
// (false, nil) — the caller handles the result through the generic path.
func Test_functionReturnedValueAndError_MultipleReturns_NotHandled(t *testing.T) {
	tc := newTestContext(t)

	def := &builtins.FunctionDefinition{
		Declaration: &data.Declaration{
			// Two returns: string and error — NOT just a single error kind.
			Returns: []*data.Type{data.StringType, data.ErrorType},
		},
	}

	handled, retErr := functionReturnedValueAndError(def, tc.ctx, nil, "value")

	if handled {
		t.Error("expected handled=false for multi-return declaration")
	}

	if retErr != nil {
		t.Errorf("retErr: got %v, want nil", retErr)
	}

	tc.assertStackEmpty()
}

// Test_functionReturnedValueAndError_NilDeclaration_NotHandled verifies that
// when the definition has no declaration (Declaration == nil), the function
// returns (false, nil) and does not push anything.
func Test_functionReturnedValueAndError_NilDeclaration_NotHandled(t *testing.T) {
	tc := newTestContext(t)

	def := &builtins.FunctionDefinition{
		// HasErrReturn=false and Declaration=nil → fall through to (false, nil).
	}

	handled, retErr := functionReturnedValueAndError(def, tc.ctx, nil, "value")

	if handled {
		t.Error("expected handled=false for nil Declaration")
	}

	if retErr != nil {
		t.Errorf("retErr: got %v, want nil", retErr)
	}

	tc.assertStackEmpty()
}

// Test_functionReturnedValueAndError_NonErrorReturnType_NotHandled verifies
// that a declaration with a single non-error return type (e.g. StringType) is
// not handled by this function.
func Test_functionReturnedValueAndError_NonErrorReturnType_NotHandled(t *testing.T) {
	tc := newTestContext(t)

	def := &builtins.FunctionDefinition{
		Declaration: &data.Declaration{
			Returns: []*data.Type{data.StringType},
		},
	}

	handled, _ := functionReturnedValueAndError(def, tc.ctx, nil, "hello")

	if handled {
		t.Error("expected handled=false for StringType return (only ErrorKind triggers)")
	}

	tc.assertStackEmpty()
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 7: synthesizeDefinition
// ─────────────────────────────────────────────────────────────────────────────
//
// synthesizeDefinition builds a *builtins.FunctionDefinition from a
// *data.Function (the savedDefinition) when the global function dictionary
// has no entry for the function pointer.

// Test_synthesizeDefinition_NonVariadic_DefaultArgCount verifies that for a
// non-variadic function whose ArgCount is the zero value [0, 0], the
// synthesized definition sets MinArgCount = MaxArgCount = len(Parameters).
func Test_synthesizeDefinition_NonVariadic_DefaultArgCount(t *testing.T) {
	saved := savedDecl(3, false, data.Range{}) // 3 params, ArgCount=[0,0]

	got := synthesizeDefinition(nil, "myFunc", saved)

	if got.MinArgCount != 3 {
		t.Errorf("MinArgCount: got %d, want 3", got.MinArgCount)
	}

	if got.MaxArgCount != 3 {
		t.Errorf("MaxArgCount: got %d, want 3", got.MaxArgCount)
	}

	if got.Name != "myFunc" {
		t.Errorf("Name: got %q, want %q", got.Name, "myFunc")
	}
}

// Test_synthesizeDefinition_NonVariadic_ExplicitArgCount verifies that an
// explicit [min, max] ArgCount range is correctly propagated to the synthesized
// definition.
func Test_synthesizeDefinition_NonVariadic_ExplicitArgCount(t *testing.T) {
	saved := savedDecl(1, false, data.Range{2, 5})

	got := synthesizeDefinition(nil, "rangeFunc", saved)

	if got.MinArgCount != 2 {
		t.Errorf("MinArgCount: got %d, want 2", got.MinArgCount)
	}

	if got.MaxArgCount != 5 {
		t.Errorf("MaxArgCount: got %d, want 5", got.MaxArgCount)
	}
}

// Test_synthesizeDefinition_Variadic_TwoParams verifies that a variadic
// function with 2 declared parameters gets MinArgCount = 1 (the number of
// required non-variadic parameters) and MaxArgCount = 99999 (effectively
// unlimited).
func Test_synthesizeDefinition_Variadic_TwoParams(t *testing.T) {
	// 2-param variadic: first param is required, second is variadic.
	saved := savedDecl(2, true, data.Range{})

	got := synthesizeDefinition(nil, "varFn", saved)

	if got.MinArgCount != 1 {
		t.Errorf("MinArgCount: got %d, want 1", got.MinArgCount)
	}

	if got.MaxArgCount != 99999 {
		t.Errorf("MaxArgCount: got %d, want 99999", got.MaxArgCount)
	}
}

// Test_synthesizeDefinition_Variadic_ZeroParams verifies the CALL-10 fix:
// a variadic function with zero declared parameters now produces
// MinArgCount = 0 (not -1).
//
// Before the fix, the formula len(Parameters)-1 = 0-1 = -1 expressed
// "minimum 0 arguments" correctly by accident (len(args) < -1 is never true),
// but -1 was semantically wrong.  The clamp ensures the minimum is always
// expressed as a non-negative integer.
func Test_synthesizeDefinition_Variadic_ZeroParams(t *testing.T) {
	// 0-param variadic function — unusual but technically possible.
	saved := savedDecl(0, true, data.Range{})

	got := synthesizeDefinition(nil, "bareVariadic", saved)

	// After the CALL-10 fix: MinArgCount must be 0, not -1.
	if got.MinArgCount != 0 {
		t.Errorf("MinArgCount: got %d, want 0 (CALL-10 fix); -1 means the fix was not applied",
			got.MinArgCount)
	}
}

// Test_synthesizeDefinition_DeclarationPreserved verifies that the synthesized
// definition carries the same *data.Declaration pointer as the savedDefinition.
// Other parts of the runtime (e.g. strict type checking) read from
// Declaration, so it must not be lost or copied.
func Test_synthesizeDefinition_DeclarationPreserved(t *testing.T) {
	saved := savedDecl(1, false, data.Range{})

	got := synthesizeDefinition(nil, "fn", saved)

	if got.Declaration != saved.Declaration {
		t.Error("synthesizeDefinition: Declaration not preserved (pointer mismatch)")
	}
}
