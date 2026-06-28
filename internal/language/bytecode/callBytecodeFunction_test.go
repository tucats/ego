package bytecode

// callBytecodeFunction_test.go contains unit tests for callBytecodeFunction
// and the closely related getPackageSymbols helper defined in flow.go.
//
// # What callBytecodeFunction does
//
// callBytecodeFunction sets up a new execution frame for a compiled Ego function
// stored as a *ByteCode.  It is always called from callByteCode (the general
// function-call dispatcher) when the function pointer resolves to *ByteCode.
//
// The function:
//  1. Decides which symbol table to use as the new scope's parent, based on
//     whether the function is a literal (closure) or a named function.
//  2. Creates and pushes a CallFrame onto the execution stack so that a later
//     Return instruction can restore the caller's state.
//  3. Stores the supplied argument slice in the new scope under the special
//     variable name "__args" (defs.ArgumentListVariable).
//  4. Always returns nil — it sets up state and lets the run loop continue
//     executing from program counter 0 of the new function's bytecode.
//
// # Symbol-table rules
//
// Named functions (literal == false):
//
//	The new symbol table has boundary = true.  This isolates the function from
//	the caller's local variables: symbol lookups inside the function only reach
//	the function's own symbols and the global root, skipping the caller's locals.
//
// Literal functions / closures (literal == true, capturedScope == nil):
//
//	The new symbol table has boundary = false and its parent is c.symbols
//	(the current active table at call time).  This lets the closure read caller-
//	local variables normally.
//
// Literal functions with a captured scope (literal == true, capturedScope != nil):
//
//	The new symbol table's parent is function.capturedScope rather than
//	c.symbols, so that the closure continues to see variables from the scope
//	where it was originally defined, even if that scope has since been popped
//	off the active chain.  boundary is also false.
//
// # How to read these tests
//
// Each test builds a testContext with newTestContext (see testhelpers_test.go),
// constructs a *ByteCode with the appropriate literal/captured-scope flags,
// calls callBytecodeFunction directly, and then inspects the resulting context
// state: c.symbols (the new scope), c.programCounter, c.bc, and the "__args"
// symbol.
//
// After a successful call, c.symbols points to the NEWLY CREATED scope; the
// previous scope can be reached via c.symbols.Parent().

import (
	"reflect"
	"testing"

	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/language/symbols"
)

// ─────────────────────────────────────────────────────────────────────────────
// Section 1: Return value
// ─────────────────────────────────────────────────────────────────────────────

// Test_callBytecodeFunction_AlwaysReturnsNil verifies the invariant that
// callBytecodeFunction never returns an error.  Its job is to set up state and
// then return nil so the run loop continues into the new function's bytecode.
func Test_callBytecodeFunction_AlwaysReturnsNil(t *testing.T) {
	tc := newTestContext(t)
	fn := &ByteCode{name: "testFunc"}

	err := callBytecodeFunction(tc.ctx, fn, []any{})

	tc.assertNoError(err)
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 2: Argument list storage
// ─────────────────────────────────────────────────────────────────────────────

// Test_callBytecodeFunction_StoresArgsInSymbolTable verifies that the supplied
// argument slice is stored under defs.ArgumentListVariable ("__args") in the
// newly created symbol table.  The ArgCheck and Arg instructions inside the
// function body read from this variable.
func Test_callBytecodeFunction_StoresArgsInSymbolTable(t *testing.T) {
	tc := newTestContext(t)
	fn := &ByteCode{name: "f"}
	args := []any{42, "hello", true}

	_ = callBytecodeFunction(tc.ctx, fn, args)

	v, found := tc.ctx.symbols.Get(defs.ArgumentListVariable)
	if !found {
		t.Fatal("__args not found in symbol table after callBytecodeFunction")
	}

	arr, ok := v.(*data.Array)
	if !ok {
		t.Fatalf("__args has wrong type: got %T, want *data.Array", v)
	}

	if arr.Len() != len(args) {
		t.Errorf("__args length: got %d, want %d", arr.Len(), len(args))
	}

	// Verify each argument is present at the expected index.
	for i, want := range args {
		got, err := arr.Get(i)
		if err != nil {
			t.Errorf("arr.Get(%d) error: %v", i, err)
			
			continue
		}

		if !reflect.DeepEqual(got, want) {
			t.Errorf("args[%d]: got %v (%T), want %v (%T)", i, got, got, want, want)
		}
	}
}

// Test_callBytecodeFunction_EmptyArgs verifies that passing a nil or empty
// argument slice stores an empty *data.Array rather than crashing.
func Test_callBytecodeFunction_EmptyArgs(t *testing.T) {
	tc := newTestContext(t)
	fn := &ByteCode{name: "noArgs"}

	_ = callBytecodeFunction(tc.ctx, fn, []any{})

	v, found := tc.ctx.symbols.Get(defs.ArgumentListVariable)
	if !found {
		t.Fatal("__args not found after zero-argument call")
	}

	arr, ok := v.(*data.Array)
	if !ok {
		t.Fatalf("__args type: got %T, want *data.Array", v)
	}

	if arr.Len() != 0 {
		t.Errorf("__args length for zero args: got %d, want 0", arr.Len())
	}
}

// Test_callBytecodeFunction_NilArgs verifies that passing a nil slice (not an
// empty one) also stores an empty *data.Array without panicking.
func Test_callBytecodeFunction_NilArgs(t *testing.T) {
	tc := newTestContext(t)
	fn := &ByteCode{name: "nilArgs"}

	// data.NewArrayFromInterfaces with a nil variadic receives zero elements.
	_ = callBytecodeFunction(tc.ctx, fn, nil)

	v, found := tc.ctx.symbols.Get(defs.ArgumentListVariable)
	if !found {
		t.Fatal("__args not found after nil-args call")
	}

	arr, ok := v.(*data.Array)
	if !ok {
		t.Fatalf("__args type: got %T, want *data.Array", v)
	}

	if arr.Len() != 0 {
		t.Errorf("__args length for nil args: got %d, want 0", arr.Len())
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 3: Call frame, program counter, and bytecode pointer
// ─────────────────────────────────────────────────────────────────────────────

// Test_callBytecodeFunction_PushesCallFrame verifies that a *CallFrame is
// present on the stack after the call.  This frame is used by the Return
// instruction to restore the caller's state when the function exits.
func Test_callBytecodeFunction_PushesCallFrame(t *testing.T) {
	tc := newTestContext(t)
	fn := &ByteCode{name: "f"}

	_ = callBytecodeFunction(tc.ctx, fn, []any{})

	// After callFramePush, framePointer points to the slot just AFTER the
	// CallFrame.  The frame itself is at framePointer-1.
	fp := tc.ctx.framePointer
	if fp == 0 {
		t.Fatal("framePointer not advanced — no call frame was pushed")
	}

	if _, ok := tc.ctx.stack[fp-1].(*CallFrame); !ok {
		t.Errorf("stack[%d]: expected *CallFrame, got %T", fp-1, tc.ctx.stack[fp-1])
	}
}

// Test_callBytecodeFunction_SetsPCToZero verifies that the program counter is
// reset to 0 so execution starts at the first instruction of the called
// function's bytecode stream.
func Test_callBytecodeFunction_SetsPCToZero(t *testing.T) {
	tc := newTestContext(t)
	tc.ctx.programCounter = 99 // simulate being mid-execution somewhere
	fn := &ByteCode{name: "f"}

	_ = callBytecodeFunction(tc.ctx, fn, []any{})

	tc.assertProgramCounter(0)
}

// Test_callBytecodeFunction_SetsBCToFunction verifies that c.bc is updated to
// point at the callee's *ByteCode.  The run loop fetches instructions from
// c.bc, so this is what causes execution to continue inside the new function.
func Test_callBytecodeFunction_SetsBCToFunction(t *testing.T) {
	tc := newTestContext(t)
	fn := &ByteCode{name: "target"}

	_ = callBytecodeFunction(tc.ctx, fn, []any{})

	if tc.ctx.bc != fn {
		t.Errorf("c.bc: got %p (%s), want %p (%s)", tc.ctx.bc, tc.ctx.bc.name, fn, fn.name)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 4: Named functions — scope boundary
// ─────────────────────────────────────────────────────────────────────────────

// Test_callBytecodeFunction_NamedFunction_IsBoundary verifies that a named
// (non-literal) function's symbol table is marked as a scope boundary.  A
// boundary prevents symbol lookups inside the function from seeing the caller's
// local variables, mirroring Go's normal scoping rules.
func Test_callBytecodeFunction_NamedFunction_IsBoundary(t *testing.T) {
	tc := newTestContext(t)
	fn := &ByteCode{name: "namedFn", literal: false}

	_ = callBytecodeFunction(tc.ctx, fn, []any{})

	if !tc.ctx.symbols.IsBoundary() {
		t.Error("named function symbol table: expected IsBoundary()=true, got false")
	}
}

// Test_callBytecodeFunction_NamedFunction_SymbolTableName verifies that the
// new symbol table name is prefixed with "function " followed by the
// function's name, making it identifiable in logs and debugger output.
func Test_callBytecodeFunction_NamedFunction_SymbolTableName(t *testing.T) {
	tc := newTestContext(t)
	fn := &ByteCode{name: "compute"}

	_ = callBytecodeFunction(tc.ctx, fn, []any{})

	got := tc.ctx.symbols.Name
	want := "function compute"

	if got != want {
		t.Errorf("symbol table Name: got %q, want %q", got, want)
	}
}

// Test_callBytecodeFunction_NamedFunction_SavedArgsAreAccessible verifies the
// end-to-end argument flow: after a call the arguments placed in __args are
// retrievable from the function's NEW symbol table (c.symbols), not the old one.
func Test_callBytecodeFunction_NamedFunction_SavedArgsAreAccessible(t *testing.T) {
	tc := newTestContext(t)
	fn := &ByteCode{name: "withArgs"}

	_ = callBytecodeFunction(tc.ctx, fn, []any{10, 20, 30})

	v, found := tc.ctx.symbols.Get(defs.ArgumentListVariable)
	if !found {
		t.Fatal("__args not found in new scope")
	}

	arr := v.(*data.Array)
	if arr.Len() != 3 {
		t.Errorf("arg count: got %d, want 3", arr.Len())
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 5: Literal functions (closures) without a captured scope
// ─────────────────────────────────────────────────────────────────────────────

// Test_callBytecodeFunction_LiteralFunction_IsNotBoundary verifies that a
// literal (closure) function's symbol table is NOT a scope boundary.  Without a
// boundary the closure can read caller-local variables, which is the intended
// closure semantics.
func Test_callBytecodeFunction_LiteralFunction_IsNotBoundary(t *testing.T) {
	tc := newTestContext(t)
	fn := (&ByteCode{name: "closureFn"}).Literal(true) // literal=true, capturedScope=nil

	_ = callBytecodeFunction(tc.ctx, fn, []any{})

	if tc.ctx.symbols.IsBoundary() {
		t.Error("literal function symbol table: expected IsBoundary()=false, got true")
	}
}

// Test_callBytecodeFunction_LiteralFunction_ParentIsCallerSymbols verifies
// that the parent of the new scope is the caller's active symbol table (the
// one that was current when callBytecodeFunction was invoked).  This is what
// allows the closure to find the caller's local variables.
func Test_callBytecodeFunction_LiteralFunction_ParentIsCallerSymbols(t *testing.T) {
	tc := newTestContext(t)
	callerSymbols := tc.ctx.symbols // capture before the call changes c.symbols
	fn := (&ByteCode{name: "closureFn"}).Literal(true)

	_ = callBytecodeFunction(tc.ctx, fn, []any{})

	// callFramePush creates the new table as a child of c.symbols (the caller's
	// table), so the parent of the new scope should be the caller's table.
	parent := tc.ctx.symbols.Parent()
	if parent != callerSymbols {
		t.Errorf("literal symbol table parent: got %p (%s), want %p (%s)",
			parent, parent.Name, callerSymbols, callerSymbols.Name)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 6: Literal functions with a captured scope (deep closure)
// ─────────────────────────────────────────────────────────────────────────────

// Test_callBytecodeFunction_CapturedScope_IsNotBoundary verifies that a
// literal function with a captured scope still has a non-boundary table.
// Captured-scope closures must be able to reach their defining scope.
func Test_callBytecodeFunction_CapturedScope_IsNotBoundary(t *testing.T) {
	tc := newTestContext(t)

	// Simulate the scope that was active when the closure was compiled.
	capturedScope := symbols.NewChildSymbolTable("captured", tc.ctx.symbols)
	fn := (&ByteCode{name: "deepClosure"}).Literal(true).CaptureScope(capturedScope)

	_ = callBytecodeFunction(tc.ctx, fn, []any{})

	if tc.ctx.symbols.IsBoundary() {
		t.Error("captured-scope closure table: expected IsBoundary()=false, got true")
	}
}

// Test_callBytecodeFunction_CapturedScope_ParentIsCapturedScope verifies the
// key invariant of captured-scope closures: the parent of the new symbol table
// is the CAPTURED scope (not c.symbols at call time).  This lets the closure
// continue seeing variables from its original defining scope even after that
// scope has been popped from the active chain.
func Test_callBytecodeFunction_CapturedScope_ParentIsCapturedScope(t *testing.T) {
	tc := newTestContext(t)

	capturedScope := symbols.NewChildSymbolTable("outer-closure", tc.ctx.symbols)
	fn := (&ByteCode{name: "deepClosure"}).Literal(true).CaptureScope(capturedScope)

	_ = callBytecodeFunction(tc.ctx, fn, []any{})

	parent := tc.ctx.symbols.Parent()
	if parent != capturedScope {
		t.Errorf("captured-scope closure parent:\n  got  %p (%s)\n  want %p (%s)",
			parent, parent.Name, capturedScope, capturedScope.Name)
	}
}

// Test_callBytecodeFunction_CapturedScope_CanSeeCapturedVariables verifies the
// practical effect of the captured scope: a variable set in the captured scope
// before the call is visible inside the new function's symbol table through the
// normal symbol lookup chain.
func Test_callBytecodeFunction_CapturedScope_CanSeeCapturedVariables(t *testing.T) {
	tc := newTestContext(t)

	// Place a variable in the scope that will be captured.
	capturedScope := symbols.NewChildSymbolTable("outer", tc.ctx.symbols)
	capturedScope.SetAlways("outerVar", 99)

	fn := (&ByteCode{name: "readOuter"}).Literal(true).CaptureScope(capturedScope)

	_ = callBytecodeFunction(tc.ctx, fn, []any{})

	// The function's new scope is a child of capturedScope, so Get should
	// traverse up and find "outerVar" in capturedScope.
	v, found := tc.ctx.symbols.Get("outerVar")
	if !found {
		t.Fatal("outerVar not found via captured scope chain")
	}

	if !reflect.DeepEqual(v, 99) {
		t.Errorf("outerVar: got %v, want 99", v)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 7: parentTable nil-fallback (named functions, root context)
// ─────────────────────────────────────────────────────────────────────────────
//
// When c.symbols is a root-level table, FindNextScope() returns nil, making
// parentTable nil.  callFramePush always uses c.symbols as the parent (not
// parentTable), so the call succeeds regardless.  The log statement is guarded
// by a conditional check for nil parentTable (CALL-4 fix).

// Test_callBytecodeFunction_NilFindNextScope_StillSucceeds verifies that a
// named function call succeeds even when c.symbols.FindNextScope() returns nil.
// The call frame is correctly set up because callFramePush uses c.symbols, not
// parentTable, to create the new scope.
func Test_callBytecodeFunction_NilFindNextScope_StillSucceeds(t *testing.T) {
	// Use a root-level symbol table so FindNextScope() returns nil.
	root := symbols.NewRootSymbolTable("isolated-root")
	bc := &ByteCode{name: "rootFn"}
	ctx := NewContext(root, bc)

	fn := &ByteCode{name: "callee", literal: false}

	err := callBytecodeFunction(ctx, fn, []any{"arg0"})

	if err != nil {
		t.Errorf("unexpected error with nil FindNextScope: %v", err)
	}

	// The call frame should be pushed and __args set normally.
	v, found := ctx.symbols.Get(defs.ArgumentListVariable)
	if !found {
		t.Fatal("__args not found after nil-FindNextScope call")
	}

	if arr, ok := v.(*data.Array); !ok || arr.Len() != 1 {
		t.Errorf("__args: got %v, expected 1-element array", v)
	}
}

// Test_callBytecodeFunction_PackageMethod_UsesCallFramePush verifies that when
// a package receiver is on the stack, callBytecodeFunction still uses
// callFramePush (the regular scope path) rather than a package-symbol-table
// clone.  This ensures the function can reach all globally imported packages.
//
// Note: the package receiver on the receiverStack is intentionally ignored by
// callBytecodeFunction.  Receivers are consumed by callNative or
// callRuntimeFunction for native Go functions; for compiled Ego functions the
// existing updatePackageFromLocalSymbols mechanism (invoked by callFramePop)
// handles any needed symbol writeback without requiring a scope clone.
func Test_callBytecodeFunction_PackageMethod_UsesCallFramePush(t *testing.T) {
	tc := newTestContext(t)

	pkg := data.NewPackage("math", "math")
	tc.ctx.pushThis("math", pkg)

	fn := &ByteCode{name: "Sin", literal: false}

	err := callBytecodeFunction(tc.ctx, fn, []any{})

	tc.assertNoError(err)

	// The scope must be a boundary (created by callFramePush for a named
	// function), NOT a clone of the package's symbol table.
	if tc.ctx.symbols.IsClone() {
		t.Error("symbol table is unexpectedly a clone: package receiver should be ignored by callBytecodeFunction")
	}

	if !tc.ctx.symbols.IsBoundary() {
		t.Error("symbol table is not a boundary: callFramePush was not used")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 8: getPackageSymbols and the package-method call path
// ─────────────────────────────────────────────────────────────────────────────
//
// getPackageSymbols returns the symbol table embedded inside a *data.Package
// that is the current method receiver.  callBytecodeFunction uses the result
// to decide whether to clone the package symbol table (package method call)
// or create a fresh child scope (plain function call).
//
// Prior to the CALL-5 fix, getPackageSymbols passed the raw `this` struct to
// GetPackageSymbolTable instead of `this.value`, so the *data.Package type
// assertion always failed and nil was returned.  The package-method clone path
// was therefore permanently unreachable.  The fix is a one-word change in
// flow.go: `receiver.value` instead of `receiver`.

// Test_getPackageSymbols_EmptyReceiverStack verifies the normal case: when the
// receiver stack is empty, getPackageSymbols returns nil (no package context).
func Test_getPackageSymbols_EmptyReceiverStack(t *testing.T) {
	tc := newTestContext(t)

	result := tc.ctx.getPackageSymbols()

	if result != nil {
		t.Errorf("expected nil for empty receiver stack, got %v", result)
	}
}

// Test_getPackageSymbols_PackageReceiver_ReturnsSymbolTable verifies that
// getPackageSymbols now correctly returns the package's embedded symbol table
// when the top of the receiver stack is a *data.Package (CALL-5 fix).
func Test_getPackageSymbols_PackageReceiver_ReturnsSymbolTable(t *testing.T) {
	tc := newTestContext(t)

	pkg := data.NewPackage("math", "math")
	tc.ctx.pushThis("math", pkg)

	result := tc.ctx.getPackageSymbols()

	if result == nil {
		t.Fatal("getPackageSymbols: expected non-nil symbol table for package receiver, got nil")
	}

	// GetPackageSymbolTable names its table "package <name>".
	want := "package math"
	if result.Name != want {
		t.Errorf("symbol table Name: got %q, want %q", result.Name, want)
	}
}

// Test_getPackageSymbols_NonPackageReceiver verifies that a non-package value
// on the receiver stack returns nil.  Only *data.Package values should trigger
// the package-symbol-table path.
func Test_getPackageSymbols_NonPackageReceiver(t *testing.T) {
	tc := newTestContext(t)

	tc.ctx.pushThis("notAPackage", data.NewStructFromMap(map[string]any{"x": 1}))

	result := tc.ctx.getPackageSymbols()

	if result != nil {
		t.Errorf("expected nil for non-package receiver, got %v", result)
	}
}

// Test_callBytecodeFunction_PackageMethod_CanAccessGlobalPackages verifies
// that when a function is called with a package receiver present, the function
// scope can still resolve globally imported packages.  This was the regression
// introduced when the package-symbol-table clone path was activated (the clone
// only contained the receiver package's own symbols, not the global registry).
// With the clone path removed, the function scope is a normal boundary child of
// c.symbols and has the full scope chain.
func Test_callBytecodeFunction_PackageMethod_CanAccessGlobalPackages(t *testing.T) {
	tc := newTestContext(t)

	// Register a "math" package in the root symbol table to simulate a normal
	// import.  This is what the function must be able to find after the call.
	mathPkg := data.NewPackage("math", "math")
	tc.ctx.symbols.Root().SetAlways("math", mathPkg)

	// Push the same package as the receiver (simulating `math.Factor(n)`)
	tc.ctx.pushThis("math", mathPkg)

	fn := &ByteCode{name: "Factor", literal: false}

	err := callBytecodeFunction(tc.ctx, fn, []any{10})

	tc.assertNoError(err)

	// Inside the new scope, the "math" package registered globally must still
	// be findable (it would not be if the scope were a clone of the package's
	// embedded symbol table).
	_, found := tc.ctx.symbols.Get("math")
	if !found {
		t.Error("'math' not found in function scope: global scope access is broken")
	}
}
