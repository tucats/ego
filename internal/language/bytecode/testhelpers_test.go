package bytecode

// testhelpers_test.go provides a reusable testing infrastructure for bytecode
// instruction unit tests.  Every bytecode instruction test in this package
// should use these helpers instead of constructing its own Context, symbol
// table, and assertions from scratch.
//
// # Why a shared infrastructure?
//
// Each bytecode instruction function has the same signature:
//
//	func xxxByteCode(c *Context, i any) error
//
// Setting up a Context correctly requires creating compatible symbol tables,
// a ByteCode object, and wiring them together.  Without shared helpers that
// boilerplate is repeated in every test file, making tests hard to read and
// maintain.
//
// # Design summary
//
// testContext wraps a *Context and a *testing.T.  Build one with
// newTestContext, then call the "with" methods to set up initial state, then
// call the bytecode function under test, and finally use the "assert" methods
// to verify the outcome.
//
//	tc := newTestContext(t).
//	    withStack(42, "hello").
//	    withSymbol("x", 0).
//	    withArgList(100, "world")
//
//	err := someByteCode(tc.ctx, operand)
//	tc.assertNoError(err)
//	tc.assertSymbolValue("x", 100)
//
// # Extending this file
//
// If a new bytecode instruction requires setup or assertions that are not
// yet covered here, add new "with" or "assert" methods to testContext.
// Keep each method focused on one concern and document its behavior.

import (
	"reflect"
	"testing"

	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/symbols"
)

// nilError is a sentinel string used by older table-driven tests when an
// expected error value is nil (meaning "no error expected").  It appears in
// legacy test files that predate the assertNoError/assertError helpers.
const nilError = "nil error"

// testContext is the central test-harness type.  It holds a fully initialized
// *Context (the real runtime type used by bytecode instructions) together with
// a reference to the *testing.T used to report failures.
//
// Build a testContext with newTestContext, configure it with the "with" helper
// methods, run the bytecode function under test, then verify the outcome with
// the "assert" helper methods.
type testContext struct {
	// ctx is the live bytecode execution context.  Bytecode instruction
	// functions receive this pointer as their first argument.
	ctx *Context

	// t is the Go test instance.  All assertion helpers report failures
	// through this value.
	t *testing.T
}

// newTestContext creates a minimal but correctly wired *Context for use in
// bytecode instruction tests.  The symbol-table hierarchy is:
//
//	root (NewRootSymbolTable)
//	  └── local (NewChildSymbolTable)
//
// Using a two-level hierarchy mirrors what compiled Ego code sees at runtime:
// a function's local scope is always a child of some outer (root) scope.
//
// The returned testContext has an empty stack and an otherwise empty symbol
// table.  Use the "with" methods to populate initial state before calling
// the bytecode function under test.
func newTestContext(t *testing.T) *testContext {
	t.Helper()

	root := symbols.NewRootSymbolTable("test root")
	local := symbols.NewChildSymbolTable("test local", root)
	bc := &ByteCode{}

	return &testContext{
		ctx: NewContext(local, bc),
		t:   t,
	}
}

// withStack pushes items onto the context's stack, left to right, so that
// items[len-1] ends up on top.  This matches the way test-case literals are
// written: the last element in the slice is the one that will be popped first.
//
// Example: withStack(1, 2, 3) leaves 3 on top, ready to be popped first.
//
// Returns tc so calls can be chained.
func (tc *testContext) withStack(items ...any) *testContext {
	tc.t.Helper()

	for _, item := range items {
		if err := tc.ctx.push(item); err != nil {
			tc.t.Fatalf("testContext.withStack: push failed: %v", err)
		}
	}

	return tc
}

// withSymbol creates and initializes a named variable in the local symbol
// table.  The variable is created first (so it exists) and then set to the
// supplied value.  This simulates what the compiler emits when declaring a
// local variable before a function body runs.
//
// Returns tc so calls can be chained.
func (tc *testContext) withSymbol(name string, value any) *testContext {
	tc.t.Helper()

	if err := tc.ctx.create(name); err != nil {
		tc.t.Fatalf("testContext.withSymbol: create %q failed: %v", name, err)
	}

	if err := tc.ctx.set(name, value); err != nil {
		tc.t.Fatalf("testContext.withSymbol: set %q failed: %v", name, err)
	}

	return tc
}

// withArgList stores args as a *data.Array in the symbol table under the
// defs.ArgumentListVariable name ("__args").  This is the variable that
// argByteCode (and any other instruction that reads function arguments) looks
// up at runtime.
//
// Elements are stored as an InterfaceType array so any value type is accepted.
//
// Returns tc so calls can be chained.
func (tc *testContext) withArgList(args ...any) *testContext {
	tc.t.Helper()

	argArray := data.NewArrayFromInterfaces(data.InterfaceType, args...)
	tc.ctx.symbols.SetAlways(defs.ArgumentListVariable, argArray)

	return tc
}

// withTypeStrictness sets the type-checking mode for the context.
// Use the defs.XxxTypeEnforcement constants:
//
//	defs.StrictTypeEnforcement   – types must match exactly
//	defs.RelaxedTypeEnforcement  – numeric widening is allowed
//	defs.NoTypeEnforcement       – all coercions are permitted (default)
//
// Returns tc so calls can be chained.
func (tc *testContext) withTypeStrictness(level int) *testContext {
	tc.ctx.typeStrictness = level

	return tc
}

// withExtensions enables or disables Ego language extensions for the context.
// Some bytecodes behave differently when extensions are active (for example,
// addByteCode allows appending a scalar to an array).
//
// Returns tc so calls can be chained.
func (tc *testContext) withExtensions(enabled bool) *testContext {
	tc.ctx.extensions = enabled

	return tc
}

// withBytecodeSize sets the ByteCode's nextAddress field, which defines the
// upper bound for valid branch targets.  The branch instructions reject any
// destination address greater than nextAddress, so tests that need to branch
// to a non-zero address must call this first.
//
// Example: tc.withBytecodeSize(10) makes addresses 0..10 valid targets.
//
// Returns tc so calls can be chained.
func (tc *testContext) withBytecodeSize(size int) *testContext {
	tc.ctx.bc.nextAddress = size

	return tc
}

// withNextOpcode appends a single instruction to the context's bytecode
// stream and leaves the program counter pointing at it, as if the run loop
// were about to execute it next. This is used by tests for
// pushMultiReturnResult (see call.go) — it needs to distinguish "the next
// thing that runs is a StackCheck" (an explicit multi-value assignment is
// waiting for every return value) from every other situation (where only
// the primary return value should be pushed). Passing bytecode.StackCheck
// simulates the first case; any other opcode, or not calling this helper at
// all, simulates the second.
//
// Returns tc so calls can be chained.
func (tc *testContext) withNextOpcode(opcode Opcode) *testContext {
	tc.t.Helper()

	tc.ctx.bc.Emit(opcode)

	return tc
}

// ─── Assertion helpers ───────────────────────────────────────────────────────

// assertNoError fails the test if err is non-nil.  Use this when the bytecode
// function under test is expected to succeed.
func (tc *testContext) assertNoError(err error) {
	tc.t.Helper()

	if err != nil {
		tc.t.Errorf("unexpected error: %v", err)
	}
}

// assertError fails the test if the actual error does not match the expected
// error.  Matching is done by comparing the Error() string of both values.
// This is intentionally lenient: it lets tests specify only the "kind" of
// error (e.g. errors.ErrInvalidOperand) without worrying about attached
// context strings, because runtimeError wraps the original error with
// file/line information that varies between test runs.
//
// If wantErr is nil but err is non-nil, the test fails (unexpected error).
// If wantErr is non-nil but err is nil, the test fails (expected error absent).
// If both are non-nil, the test fails if their base error keys differ.
func (tc *testContext) assertError(err, wantErr error) {
	tc.t.Helper()

	if err == nil && wantErr == nil {
		return
	}

	if err != nil && wantErr == nil {
		tc.t.Errorf("unexpected error: %v", err)

		return
	}

	if err == nil && wantErr != nil {
		tc.t.Errorf("expected error %v, got nil", wantErr)

		return
	}

	// Both non-nil: compare using errors.Equals, which compares the
	// underlying i18n key rather than the full formatted string.
	if !errors.Equals(errors.New(err), errors.New(wantErr)) {
		tc.t.Errorf("wrong error:\n  got  %v\n  want %v", err, wantErr)
	}
}

// assertTopStack pops one item from the stack and checks that it equals want
// using reflect.DeepEqual.  The test fails if the stack is empty or if the
// popped value does not match want.
func (tc *testContext) assertTopStack(want any) {
	tc.t.Helper()

	v, err := tc.ctx.Pop()
	if err != nil {
		tc.t.Errorf("assertTopStack: Pop failed: %v", err)

		return
	}

	if !reflect.DeepEqual(v, want) {
		tc.t.Errorf("assertTopStack:\n  got  %v (%T)\n  want %v (%T)", v, v, want, want)
	}
}

// assertSymbolValue reads the named symbol from the local symbol table and
// checks that its value equals want using reflect.DeepEqual.  The test fails
// if the symbol does not exist or if its value does not match.
//
// Note: symbol names starting with defs.InvisiblePrefix ("__") can be read
// directly from ctx.symbols even though they are hidden from user-level
// symbol table displays.
func (tc *testContext) assertSymbolValue(name string, want any) {
	tc.t.Helper()

	v, found := tc.ctx.symbols.Get(name)
	if !found {
		tc.t.Errorf("assertSymbolValue: symbol %q not found in symbol table", name)

		return
	}

	if !reflect.DeepEqual(v, want) {
		tc.t.Errorf("assertSymbolValue %q:\n  got  %v (%T)\n  want %v (%T)", name, v, v, want, want)
	}
}

// assertStackEmpty fails the test if there are any items remaining on the
// stack after the bytecode function under test has run.  Use this when the
// instruction is expected to consume all items it placed on the stack.
func (tc *testContext) assertStackEmpty() {
	tc.t.Helper()

	if tc.ctx.stackPointer != 0 {
		tc.t.Errorf("assertStackEmpty: expected empty stack, but stackPointer = %d", tc.ctx.stackPointer)
	}
}

// assertProgramCounter fails the test if the context's program counter does
// not equal want.  Use this to verify that a branch instruction either
// updated the PC (branch taken) or left it unchanged (branch not taken).
//
// When calling a bytecode function directly (not through Run), the PC starts
// at 0.  A taken branch sets it to the target address; a not-taken branch
// leaves it at 0.
func (tc *testContext) assertProgramCounter(want int) {
	tc.t.Helper()

	if tc.ctx.programCounter != want {
		tc.t.Errorf("assertProgramCounter: got %d, want %d", tc.ctx.programCounter, want)
	}
}
