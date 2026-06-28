package bytecode

// stack_test.go — unit tests for bytecode/stack.go
//
// # What is tested
//
// The original file covered:
//   - isStackMarker (table-driven)
//   - StackMarker.String (table-driven)
//   - findMarker (table-driven, raw &Context{} literals)
//
// This version retains those tests and adds flat, independently-named tests
// for every function in stack.go that had no coverage at all:
//
//   NewStackMarker          — empty label uses defs.Anon; values stored
//   isStackMarker           — CallFrame, case-insensitive labels, data values
//   findMarker              — flat-style parallels for the table cases
//   dropToMarkerByteCode    — named marker, any-marker, no marker, framePointer
//   stackCheckByteCode      — pass, fail (too few items, no marker)
//   pushByteCode            — plain value; literal ByteCode clones+captures scope
//   dropByteCode            — drop 1, drop N, silent underflow (STACK-3)
//   dupByteCode             — duplicates TOS; empty stack
//   readStackByteCode       — read TOS, read deeper, panic on empty (STACK-2)
//   swapByteCode            — swaps top two; one-item underflow
//   copyByteCode            — documents STACK-1 (pushes 2 instead of copy)
//   getVarArgsByteCode      — with/without arg list; position beyond end
//
// # Bugs fixed (STACK-1 through STACK-3)
//
// STACK-1: copyByteCode pushed the literal integer 2 instead of the JSON
//          deep-copy v2.  Fixed: c.push(2) → c.push(v2).
//
// STACK-2: readStackByteCode guard used ">" instead of ">=", so idx==stackPointer
//          (e.g. idx=0 on an empty stack) bypassed the check and caused a runtime
//          panic from a negative slice index.  Fixed: changed to ">=".
//
// STACK-3: dropByteCode returned nil on stack underflow instead of propagating
//          the error, making over-drops invisible to callers.
//          Fixed: return err instead of return nil in the Pop loop.
//
// # Testing conventions
//
// New tests use the newTestContext helper from testhelpers_test.go so they get
// a properly initialized Context with symbol tables and bytecode, rather than
// raw &Context{} struct literals.  Flat (non-table-driven) style is used so
// each scenario is independently runnable with -run.

import (
	"testing"

	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
)

// ─────────────────────────────────────────────────────────────────────────────
// Original tests (retained verbatim)
// ─────────────────────────────────────────────────────────────────────────────

func TestIsStackMarker(t *testing.T) {
	tests := []struct {
		name  string
		i     any
		types []string
		want  bool
	}{
		{
			name: "simple type test of simple marker",
			i:    NewStackMarker("test"),
			want: true,
		},
		{
			name: "simple type test of not-a-marker",
			i:    "test",
			want: false,
		},
		{
			name: "simple type test of complex marker",
			i:    NewStackMarker("test", 33, true),
			want: true,
		},
		{
			name:  "complex type test of matching marker",
			i:     NewStackMarker("test", 33, true),
			types: []string{"true"},
			want:  true,
		},
		{
			name:  "complex type test of mismatched marker",
			i:     NewStackMarker("test", 33, true),
			types: []string{"call"},
			want:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isStackMarker(tt.i, tt.types...); got != tt.want {
				t.Errorf("IsStackMarker() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStackMarker_String(t *testing.T) {
	tests := []struct {
		name   string
		marker StackMarker
		want   string
	}{
		{
			name:   "simple marker",
			marker: NewStackMarker("test"),
			want:   "Marker<test>",
		},
		{
			name:   "marker with one item",
			marker: NewStackMarker("test", 33),
			want:   "Marker<test, 33>",
		},
		{
			name:   "marker with multiple items",
			marker: NewStackMarker("test", 33, "foo", true),
			want:   "Marker<test, 33, foo, true>",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.marker.String(); got != tt.want {
				t.Errorf("StackMarker.String() = %v, want %v", got, tt.want)
			}
		})
	}
}
func TestFindMarker(t *testing.T) {
	tests := []struct {
		name     string
		context  *Context
		input    any
		expected int
	}{
		{
			name: "find marker in after multiple items on stack",
			context: &Context{
				stack:        []any{NewStackMarker("marker1"), 103, 102, 101},
				stackPointer: 3,
				framePointer: 0,
			},
			input:    "marker1",
			expected: 3,
		},
		{
			name: "find marker in stack",
			context: &Context{
				stack:        []any{"test", NewStackMarker("marker1"), "marker2"},
				stackPointer: 2,
				framePointer: 0,
			},
			input:    "marker1",
			expected: 1,
		},
		{
			name: "find marker in stack with multiple markers",
			context: &Context{
				stack:        []any{"test", NewStackMarker("marker1"), "marker2", NewStackMarker("marker3")},
				stackPointer: 3,
				framePointer: 0,
			},
			input:    "marker3",
			expected: 0,
		},
		{
			name: "find marker in stack with no match",
			context: &Context{
				stack:        []any{"test", NewStackMarker("marker1"), "marker2"},
				stackPointer: 2,
				framePointer: 0,
			},
			input:    "marker3",
			expected: 0,
		},
		{
			name: "find marker in empty stack",
			context: &Context{
				stack:        []any{},
				stackPointer: 0,
				framePointer: 0,
			},
			input:    "marker1",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := findMarker(tt.context, tt.input); got != tt.expected {
				t.Errorf("findMarker() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 1 — NewStackMarker
// ─────────────────────────────────────────────────────────────────────────────

// Test_NewStackMarker_EmptyLabel verifies that an empty label string is
// replaced with defs.Anon (the anonymous-variable sentinel "~").
// This ensures that every StackMarker always has a non-empty label for
// display and search purposes.
func Test_NewStackMarker_EmptyLabel(t *testing.T) {
	m := NewStackMarker("")
	if m.label != defs.Anon {
		t.Errorf("empty label: got %q, want defs.Anon (%q)", m.label, defs.Anon)
	}
}

// Test_NewStackMarker_LabelPreserved verifies that a non-empty label is stored
// exactly as given (no case folding or trimming).
func Test_NewStackMarker_LabelPreserved(t *testing.T) {
	m := NewStackMarker("MyLabel")
	if m.label != "MyLabel" {
		t.Errorf("label = %q, want %q", m.label, "MyLabel")
	}
}

// Test_NewStackMarker_ValuesStored verifies that optional extra values passed
// to NewStackMarker are stored in the marker's values slice.
func Test_NewStackMarker_ValuesStored(t *testing.T) {
	m := NewStackMarker("frame", 42, true, "extra")
	if len(m.values) != 3 {
		t.Fatalf("values length = %d, want 3", len(m.values))
	}

	if m.values[0] != 42 || m.values[1] != true || m.values[2] != "extra" {
		t.Errorf("values = %v, want [42 true extra]", m.values)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 2 — isStackMarker (additional coverage)
// ─────────────────────────────────────────────────────────────────────────────

// Test_isStackMarker_CallFrame verifies that a *CallFrame value is recognized
// as a stack marker even though it is not a StackMarker struct.
// Call frames are pushed onto the runtime stack to preserve execution context
// when a function is called; isStackMarker needs to detect them so that
// operators that scan the stack (e.g. DropToMarker) stop at frame boundaries.
func Test_isStackMarker_CallFrame(t *testing.T) {
	frame := &CallFrame{}
	if !isStackMarker(frame) {
		t.Error("*CallFrame should be recognized as a stack marker")
	}
}

// Test_isStackMarker_LabelMatchCaseInsensitive verifies that label lookup uses
// strings.EqualFold, meaning "LET" matches a marker labelled "let".
// Case-insensitive comparison makes the lookup more forgiving when labels come
// from user-entered source text.
func Test_isStackMarker_LabelMatchCaseInsensitive(t *testing.T) {
	m := NewStackMarker("let")
	if !isStackMarker(m, "LET") {
		t.Error("isStackMarker should match label case-insensitively")
	}

	if !isStackMarker(m, "Let") {
		t.Error("isStackMarker should match label case-insensitively")
	}
}

// Test_isStackMarker_MatchByDataValue verifies that a search string matches one
// of the extra data values stored inside the marker, not just the label.
// This lets callers find markers by their payload rather than their name.
func Test_isStackMarker_MatchByDataValue(t *testing.T) {
	m := NewStackMarker("frame", "session-42")
	if !isStackMarker(m, "session-42") {
		t.Error("isStackMarker should match data value stored in marker")
	}
}

// Test_isStackMarker_NoMatchReturnsNil verifies that a non-StackMarker, non-
// CallFrame value always returns false regardless of the search values given.
func Test_isStackMarker_NoMatchReturnsNil(t *testing.T) {
	if isStackMarker(nil) {
		t.Error("nil should not be a stack marker")
	}

	if isStackMarker(42) {
		t.Error("integer should not be a stack marker")
	}

	if isStackMarker("let") {
		t.Error("string should not be a stack marker")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 3 — findMarker (flat-style new coverage)
// ─────────────────────────────────────────────────────────────────────────────

// Test_findMarker_SearchByStackMarker verifies that passing a StackMarker
// (rather than a string) as the search key uses the marker's label field.
func Test_findMarker_SearchByStackMarker(t *testing.T) {
	ctx := &Context{
		stack:        []any{"a", NewStackMarker("mymark"), "b"},
		stackPointer: 2,
		framePointer: 0,
	}
	// The marker is one step below the top (depth 1).
	got := findMarker(ctx, NewStackMarker("mymark"))
	if got != 1 {
		t.Errorf("findMarker by StackMarker: depth = %d, want 1", got)
	}
}

// Test_findMarker_RespectsFramePointer verifies that the search does not go
// below framePointer.  Frame pointers mark function-call boundaries; scanning
// past them would incorrectly find markers from the caller's stack frame.
func Test_findMarker_RespectsFramePointer(t *testing.T) {
	// The marker is below framePointer — it should NOT be found.
	ctx := &Context{
		stack:        []any{NewStackMarker("deep"), "a", "b"},
		stackPointer: 2,
		framePointer: 2, // marker is at index 0, below framePointer 2
	}

	got := findMarker(ctx, "deep")
	if got != 0 {
		// 0 means "not found" — which is the expected result here because
		// the marker is below the frame pointer boundary.
		t.Errorf("findMarker should not cross framePointer; got %d", got)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 4 — dropToMarkerByteCode
// ─────────────────────────────────────────────────────────────────────────────

// Test_dropToMarkerByteCode_DropsToNamedMarker verifies the common case: items
// are popped until a marker with the specified label is consumed.  Items below
// the marker remain on the stack.
func Test_dropToMarkerByteCode_DropsToNamedMarker(t *testing.T) {
	// Stack layout (bottom to top): "below", Marker("let"), "a", "b"
	// After dropping to "let": "below" should remain.
	tc := newTestContext(t).withStack("below", NewStackMarker("let"), "a", "b")

	err := dropToMarkerByteCode(tc.ctx, NewStackMarker("let"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Only "below" should remain on the stack.
	tc.assertTopStack("below")
	tc.assertStackEmpty()
}

// Test_dropToMarkerByteCode_NilOperandDropsToFirstMarker verifies that when the
// operand is nil (no specific marker label), the function drops items until it
// hits any StackMarker.
func Test_dropToMarkerByteCode_NilOperandDropsToFirstMarker(t *testing.T) {
	// Stack: "below", Marker("anything"), "x", "y"
	tc := newTestContext(t).withStack("below", NewStackMarker("anything"), "x", "y")

	err := dropToMarkerByteCode(tc.ctx, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// "below" remains; the marker and everything above it were consumed.
	tc.assertTopStack("below")
	tc.assertStackEmpty()
}

// Test_dropToMarkerByteCode_NoMarkerDrainsToFramePointer verifies that when no
// matching marker exists, the function drains the stack down to framePointer
// (the current call-frame boundary) and returns nil, not an error.
func Test_dropToMarkerByteCode_NoMarkerDrainsToFramePointer(t *testing.T) {
	// No marker on the stack; everything above framePointer is discarded.
	tc := newTestContext(t).withStack("a", "b", "c")

	err := dropToMarkerByteCode(tc.ctx, NewStackMarker("nonexistent"))
	if err != nil {
		t.Fatalf("expected nil error when no marker found, got: %v", err)
	}

	// Stack is fully drained (the context's framePointer is 0 from newTestContext).
	tc.assertStackEmpty()
}

// Test_dropToMarkerByteCode_RespectsFramePointer verifies that the function
// stops at framePointer even if no marker has been found, preventing it from
// consuming items from an outer function's frame.
func Test_dropToMarkerByteCode_RespectsFramePointer(t *testing.T) {
	// Manually set a high framePointer to simulate a non-zero call boundary.
	tc := newTestContext(t).withStack("outer", "inner1", "inner2")
	tc.ctx.framePointer = 1 // protect stack[0] ("outer") from being dropped

	err := dropToMarkerByteCode(tc.ctx, NewStackMarker("missing"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// "inner1" and "inner2" were dropped; "outer" is below framePointer.
	// The stack pointer now equals framePointer; the stack appears empty to callers.
	if tc.ctx.stackPointer > tc.ctx.framePointer {
		t.Errorf("stack not fully drained above framePointer: sp=%d, fp=%d",
			tc.ctx.stackPointer, tc.ctx.framePointer)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 5 — stackCheckByteCode
// ─────────────────────────────────────────────────────────────────────────────

// Test_stackCheckByteCode_PassWithMarkerPresent verifies that stackCheckByteCode
// returns nil when the stack contains at least `count` items AND a StackMarker
// is present somewhere below them.
//
// stackCheckByteCode is used to verify that a function's multiple return values
// are actually on the stack before the caller tries to read them.
func Test_stackCheckByteCode_PassWithMarkerPresent(t *testing.T) {
	// Stack (bottom→top): Marker("results"), "retral1", "retval2"
	// Checking for count=2 should succeed because a marker is present.
	tc := newTestContext(t).withStack(NewStackMarker("results"), "retval1", "retval2")
	tc.ctx.bc.nextAddress = 10

	err := stackCheckByteCode(tc.ctx, 2)
	if err != nil {
		t.Errorf("expected nil when marker present with enough items, got: %v", err)
	}
}

// Test_stackCheckByteCode_FailTooFewItems verifies that the check fails when
// the stack has fewer items than the required count, regardless of markers.
func Test_stackCheckByteCode_FailTooFewItems(t *testing.T) {
	// Only one item; asking for 3 must fail.
	tc := newTestContext(t).withStack("only")

	err := stackCheckByteCode(tc.ctx, 3)
	if err == nil {
		t.Fatal("expected error when stack has too few items, got nil")
	}

	if !errors.Equal(errors.New(err), errors.ErrReturnValueCount) {
		t.Errorf("expected ErrReturnValueCount, got %v", err)
	}
}

// Test_stackCheckByteCode_FailNoMarker verifies that the check fails when there
// are enough items but no StackMarker on the stack.  The marker signals that the
// values were pushed as part of a function return sequence.
func Test_stackCheckByteCode_FailNoMarker(t *testing.T) {
	// Two plain items, no marker — must fail.
	tc := newTestContext(t).withStack("a", "b")

	err := stackCheckByteCode(tc.ctx, 2)
	if err == nil {
		t.Fatal("expected error when no marker on stack, got nil")
	}

	if !errors.Equal(errors.New(err), errors.ErrReturnValueCount) {
		t.Errorf("expected ErrReturnValueCount, got %v", err)
	}
}

// Test_stackCheckByteCode_InvalidOperand verifies that a non-integer operand
// (the count must be an integer) causes the check to fail with an error.
func Test_stackCheckByteCode_InvalidOperand(t *testing.T) {
	tc := newTestContext(t).withStack("a", "b")

	err := stackCheckByteCode(tc.ctx, "not-a-number")
	if err == nil {
		t.Fatal("expected error for non-integer operand, got nil")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 6 — pushByteCode
// ─────────────────────────────────────────────────────────────────────────────

// Test_pushByteCode_PlainValue verifies the common case: a non-ByteCode value
// (integer, string, etc.) is pushed directly onto the stack without cloning.
func Test_pushByteCode_PlainValue(t *testing.T) {
	tc := newTestContext(t)

	err := pushByteCode(tc.ctx, 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tc.assertTopStack(42)
	tc.assertStackEmpty()
}

// Test_pushByteCode_LiteralBytecodeClonesCapturesScope verifies the closure
// path: when the operand is a literal *ByteCode (IsLiteral() == true), the
// function clones the ByteCode and captures the current symbol table onto the
// clone.  This is how Ego closures capture their defining scope.
func Test_pushByteCode_LiteralBytecodeClonesCapturesScope(t *testing.T) {
	tc := newTestContext(t)

	// Create a ByteCode and mark it as a literal (closure).
	bc := New("closure")
	bc.Literal(true)

	err := pushByteCode(tc.ctx, bc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Pop the result and confirm it is a *ByteCode with a captured scope.
	v, popErr := tc.ctx.Pop()
	if popErr != nil {
		t.Fatalf("Pop: %v", popErr)
	}

	clone, ok := v.(*ByteCode)
	if !ok {
		t.Fatalf("expected *ByteCode on stack, got %T", v)
	}

	// The clone must not be the same pointer as the original.
	if clone == bc {
		t.Error("pushByteCode should have cloned the literal, not pushed the original")
	}

	// The clone must have captured the context's current symbol table.
	if clone.capturedScope != tc.ctx.symbols {
		t.Error("clone's capturedScope should equal the context's symbol table")
	}
}

// Test_pushByteCode_NonLiteralBytecodeNotCloned verifies that a *ByteCode whose
// IsLiteral() returns false is pushed as-is (no clone, no scope capture).
// Named function definitions are pushed this way — they do not capture scope.
func Test_pushByteCode_NonLiteralBytecodeNotCloned(t *testing.T) {
	tc := newTestContext(t)

	bc := New("function")
	// literal flag is false by default

	err := pushByteCode(tc.ctx, bc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	v, _ := tc.ctx.Pop()

	pushed, ok := v.(*ByteCode)
	if !ok {
		t.Fatalf("expected *ByteCode, got %T", v)
	}

	// Non-literal: the same pointer should be pushed (no clone).
	if pushed != bc {
		t.Error("non-literal ByteCode should be pushed as the original pointer")
	}
}

// Test_pushByteCode_PreservesCapturedScope is the regression test for BUG-02:
// "go func() {}() closures cannot read outer-scope variables".
//
// # Root cause
//
// When a Go statement executes (go func() { ... }()), the parent context first
// pushes the literal closure through pushByteCode — that push clones the raw
// compiled bytecode and stamps the parent's local symbol table onto the clone's
// capturedScope field.  goByteCode then pops this clone as the function value fx.
//
// GoRoutine rebuilds a tiny call sequence ("Push fx; Call N") that runs in a
// minimal goroutine context whose c.symbols is a child of the global root table
// and knows nothing about the parent's local variables.  When that "Push fx"
// instruction executes, pushByteCode previously cloned fx (correctly copying
// capturedScope from the parent-captured clone) but then unconditionally
// overwrote capturedScope with c.symbols — the goroutine's root-child scope —
// discarding the parent's local scope entirely.  Every outer variable reference
// inside the closure then produced "unknown identifier".
//
// # Fix
//
// pushByteCode now only stamps c.symbols onto the clone when capturedScope is
// nil.  For the normal and loop-iteration cases the clone always starts with a
// nil capturedScope (copied from the original compiled literal), so behavior is
// unchanged.  For the goroutine re-push case the clone inherits a non-nil
// capturedScope from fx and the guard leaves it intact.
func Test_pushByteCode_PreservesCapturedScope(t *testing.T) {
	// Simulate the parent context: pushByteCode captures the parent's scope.
	parentCtx := newTestContext(t)

	bc := New("goroutine-closure")
	bc.Literal(true)

	// First push: the parent context stamps its own symbol table.
	if err := pushByteCode(parentCtx.ctx, bc); err != nil {
		t.Fatalf("parent push: %v", err)
	}

	fx, popErr := parentCtx.ctx.Pop()
	if popErr != nil {
		t.Fatalf("parent pop: %v", popErr)
	}

	fxCode, ok := fx.(*ByteCode)
	if !ok {
		t.Fatalf("expected *ByteCode, got %T", fx)
	}

	// The clone must carry the parent's scope as its captured scope.
	parentScope := fxCode.capturedScope
	if parentScope == nil {
		t.Fatal("first push did not capture the parent scope")
	}

	if parentScope != parentCtx.ctx.symbols {
		t.Error("first push: capturedScope should be the parent context's symbol table")
	}

	// Simulate GoRoutine: a separate goroutine context with a different (minimal)
	// symbol table re-pushes fx.  Before the BUG-02 fix, this second push would
	// overwrite capturedScope with the goroutine's root-child scope.
	goroutineCtx := newTestContext(t)

	// Sanity-check: the goroutine context has a different symbol table.
	if goroutineCtx.ctx.symbols == parentCtx.ctx.symbols {
		t.Fatal("test setup error: goroutine context must have a distinct symbol table")
	}

	if err := pushByteCode(goroutineCtx.ctx, fxCode); err != nil {
		t.Fatalf("goroutine push: %v", err)
	}

	fxCode2, popErr2 := goroutineCtx.ctx.Pop()
	if popErr2 != nil {
		t.Fatalf("goroutine pop: %v", popErr2)
	}

	rePushed, ok := fxCode2.(*ByteCode)
	if !ok {
		t.Fatalf("expected *ByteCode from goroutine push, got %T", fxCode2)
	}

	// The re-pushed clone must still point at the PARENT scope, not
	// the goroutine context's scope.
	if rePushed.capturedScope != parentScope {
		t.Errorf("BUG-02: second push overwrote capturedScope:\n  got  %q\n  want %q",
			rePushed.capturedScope.Name, parentScope.Name)
	}

	if rePushed.capturedScope == goroutineCtx.ctx.symbols {
		t.Error("BUG-02: capturedScope was replaced with the goroutine context's scope")
	}
}

// Test_pushByteCode_LoopIterationsCaptureDifferentScopes verifies that the
// loop-closure case (FUNC-H2) continues to work after the BUG-02 fix.
//
// Each iteration of a loop pushes the same compiled literal but must capture a
// distinct per-iteration scope.  The original compiled literal has a nil
// capturedScope, so the BUG-02 guard (skip stamp when capturedScope != nil) does
// not interfere: every push produces a fresh clone with the current iteration's
// scope.
func Test_pushByteCode_LoopIterationsCaptureDifferentScopes(t *testing.T) {
	// A single compiled literal with no captured scope — exactly how the
	// compiler emits a function literal before any execution.
	original := New("loop-closure")
	original.Literal(true)

	// Simulate two loop iterations in two different contexts.
	ctx1 := newTestContext(t)
	ctx2 := newTestContext(t)

	if err := pushByteCode(ctx1.ctx, original); err != nil {
		t.Fatalf("ctx1 push: %v", err)
	}

	if err := pushByteCode(ctx2.ctx, original); err != nil {
		t.Fatalf("ctx2 push: %v", err)
	}

	v1, _ := ctx1.ctx.Pop()
	v2, _ := ctx2.ctx.Pop()

	clone1, ok1 := v1.(*ByteCode)
	clone2, ok2 := v2.(*ByteCode)

	if !ok1 || !ok2 {
		t.Fatalf("expected *ByteCode from both pushes; got %T and %T", v1, v2)
	}

	// Each clone must capture its own context's scope.
	if clone1.capturedScope != ctx1.ctx.symbols {
		t.Error("clone1 did not capture ctx1's symbol table")
	}

	if clone2.capturedScope != ctx2.ctx.symbols {
		t.Error("clone2 did not capture ctx2's symbol table")
	}

	// The two captured scopes must be distinct.
	if clone1.capturedScope == clone2.capturedScope {
		t.Error("different loop iterations captured the same scope")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 7 — dropByteCode
// ─────────────────────────────────────────────────────────────────────────────

// Test_dropByteCode_NilOperandDropsOne verifies the default behavior: a nil
// operand means "drop exactly one item".
func Test_dropByteCode_NilOperandDropsOne(t *testing.T) {
	tc := newTestContext(t).withStack("keep", "drop")

	err := dropByteCode(tc.ctx, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// "drop" (top) was removed; "keep" remains.
	tc.assertTopStack("keep")
	tc.assertStackEmpty()
}

// Test_dropByteCode_DropN verifies that an integer operand N drops N items
// from the top of the stack, leaving the rest intact.
func Test_dropByteCode_DropN(t *testing.T) {
	tc := newTestContext(t).withStack("keep", "d1", "d2", "d3")

	err := dropByteCode(tc.ctx, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tc.assertTopStack("keep")
	tc.assertStackEmpty()
}

// Test_dropByteCode_InvalidOperandReturnsError verifies that a non-integer
// operand causes dropByteCode to return an error.
func Test_dropByteCode_InvalidOperandReturnsError(t *testing.T) {
	tc := newTestContext(t).withStack("a")

	err := dropByteCode(tc.ctx, "not-a-number")
	if err == nil {
		t.Fatal("expected error for non-integer operand, got nil")
	}
}

// Test_dropByteCode_UnderflowReturnsError_STACK3 verifies the STACK-3 fix:
// when Drop is asked to remove more items than the stack contains, it now
// returns ErrStackUnderflow instead of silently returning nil.
//
// STACK-3 fix: changed `return nil` to `return err` inside the Pop loop so
// that over-drops are reported consistently with every other stack-consuming
// instruction in the package.
func Test_dropByteCode_UnderflowReturnsError_STACK3(t *testing.T) {
	// Stack has 1 item; ask to drop 5 — stack runs dry after the first pop.
	tc := newTestContext(t).withStack("single")

	err := dropByteCode(tc.ctx, 5)

	// STACK-3 fix: must now return an error, not nil.
	if err == nil {
		t.Fatal("STACK-3: expected ErrStackUnderflow for over-drop, got nil")
	}

	if !errors.Equal(errors.New(err), errors.ErrStackUnderflow) {
		t.Errorf("STACK-3: expected ErrStackUnderflow, got %v", err)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 8 — dupByteCode
// ─────────────────────────────────────────────────────────────────────────────

// Test_dupByteCode_DuplicatesTop verifies that the top-of-stack value is
// duplicated: after Dup, the same value appears twice at the top.
func Test_dupByteCode_DuplicatesTop(t *testing.T) {
	tc := newTestContext(t).withStack("original")

	err := dupByteCode(tc.ctx, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Both the duplicate and the original must be on the stack.
	tc.assertTopStack("original") // top — the duplicate
	tc.assertTopStack("original") // next — the original
	tc.assertStackEmpty()
}

// Test_dupByteCode_EmptyStackReturnsError verifies that Dup on an empty stack
// returns an error rather than panicking.
func Test_dupByteCode_EmptyStackReturnsError(t *testing.T) {
	tc := newTestContext(t)

	err := dupByteCode(tc.ctx, nil)
	if err == nil {
		t.Fatal("expected error for Dup on empty stack, got nil")
	}

	if !errors.Equal(errors.New(err), errors.ErrStackUnderflow) {
		t.Errorf("expected ErrStackUnderflow, got %v", err)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 9 — readStackByteCode
// ─────────────────────────────────────────────────────────────────────────────

// Test_readStackByteCode_ReadTop verifies that index 0 duplicates the top-of-
// stack item (same as Dup but expressed as ReadStack).
func Test_readStackByteCode_ReadTop(t *testing.T) {
	tc := newTestContext(t).withStack("bottom", "top")

	err := readStackByteCode(tc.ctx, 0) // idx=0 means TOS
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// A copy of "top" is pushed; original stack is unchanged beneath it.
	tc.assertTopStack("top")  // the pushed copy
	tc.assertTopStack("top")  // the original TOS
	tc.assertTopStack("bottom")
	tc.assertStackEmpty()
}

// Test_readStackByteCode_ReadDeeper verifies that a positive index reads a value
// further down the stack without removing or disturbing the items above it.
func Test_readStackByteCode_ReadDeeper(t *testing.T) {
	// Stack (bottom→top): "deep", "middle", "top"
	tc := newTestContext(t).withStack("deep", "middle", "top")

	// idx=1 means the item one below TOS, i.e. "middle".
	err := readStackByteCode(tc.ctx, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// "middle" is pushed on top; original stack is undisturbed.
	tc.assertTopStack("middle")
	tc.assertTopStack("top")
	tc.assertTopStack("middle")
	tc.assertTopStack("deep")
	tc.assertStackEmpty()
}

// Test_readStackByteCode_NegativeIndexAliasedToPositive verifies that a
// negative index is treated as its absolute value.  idx=-1 is the same as
// idx=1 (one below TOS).
func Test_readStackByteCode_NegativeIndexAliasedToPositive(t *testing.T) {
	tc := newTestContext(t).withStack("bottom", "top")

	err := readStackByteCode(tc.ctx, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tc.assertTopStack("bottom") // |−1| = 1 → one below TOS
}

// Test_readStackByteCode_IndexEqualsStackPointer_STACK2 verifies the STACK-2
// fix: when idx == stackPointer (e.g. one item on the stack, idx=1), the guard
// now catches the boundary case and returns ErrStackUnderflow instead of
// computing a negative slice index and panicking.
//
// STACK-2 fix: changed guard from `idx > c.stackPointer` to
// `idx >= c.stackPointer`.
func Test_readStackByteCode_IndexEqualsStackPointer_STACK2(t *testing.T) {
	tc := newTestContext(t).withStack("only") // stackPointer = 1

	// idx=1 with stackPointer=1: the old guard (>) let this through; the new
	// guard (>=) correctly rejects it.
	err := readStackByteCode(tc.ctx, 1)

	if err == nil {
		t.Fatal("STACK-2: expected ErrStackUnderflow when idx == stackPointer, got nil")
	}

	if !errors.Equal(errors.New(err), errors.ErrStackUnderflow) {
		t.Errorf("STACK-2: expected ErrStackUnderflow, got %v", err)
	}
}

// Test_readStackByteCode_EmptyStack_STACK2 verifies the STACK-2 fix: calling
// readStackByteCode with idx=0 on an empty stack now returns ErrStackUnderflow
// instead of panicking.
//
// STACK-2 fix: the guard `idx >= c.stackPointer` correctly catches the case
// where stackPointer=0 and idx=0 (0 >= 0 is true → error returned), preventing
// the negative slice index that previously caused a runtime panic.
func Test_readStackByteCode_EmptyStack_STACK2(t *testing.T) {
	tc := newTestContext(t) // stack is empty; stackPointer=0

	err := readStackByteCode(tc.ctx, 0)

	if err == nil {
		t.Fatal("STACK-2: expected ErrStackUnderflow on empty stack with idx=0, got nil")
	}

	if !errors.Equal(errors.New(err), errors.ErrStackUnderflow) {
		t.Errorf("STACK-2: expected ErrStackUnderflow, got %v", err)
	}
}

// Test_readStackByteCode_InvalidOperand verifies that a non-integer operand
// causes an error.
func Test_readStackByteCode_InvalidOperand(t *testing.T) {
	tc := newTestContext(t).withStack("a")

	err := readStackByteCode(tc.ctx, "bad")
	if err == nil {
		t.Fatal("expected error for non-integer operand, got nil")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 10 — swapByteCode
// ─────────────────────────────────────────────────────────────────────────────

// Test_swapByteCode_SwapsTopTwo verifies that Swap exchanges the top two stack
// items: what was TOS-1 is now TOS, and vice versa.
func Test_swapByteCode_SwapsTopTwo(t *testing.T) {
	tc := newTestContext(t).withStack("first", "second")

	err := swapByteCode(tc.ctx, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// After swap: "first" should now be on top.
	tc.assertTopStack("first")
	tc.assertTopStack("second")
	tc.assertStackEmpty()
}

// Test_swapByteCode_OneItemUnderflow verifies that Swap returns an error when
// there is only one item on the stack (can't swap without two).
func Test_swapByteCode_OneItemUnderflow(t *testing.T) {
	tc := newTestContext(t).withStack("solo")

	err := swapByteCode(tc.ctx, nil)
	if err == nil {
		t.Fatal("expected underflow error with only one item, got nil")
	}

	if !errors.Equal(errors.New(err), errors.ErrStackUnderflow) {
		t.Errorf("expected ErrStackUnderflow, got %v", err)
	}
}

// Test_swapByteCode_EmptyStackUnderflow verifies that Swap returns an error on
// an empty stack.
func Test_swapByteCode_EmptyStackUnderflow(t *testing.T) {
	tc := newTestContext(t)

	err := swapByteCode(tc.ctx, nil)
	if err == nil {
		t.Fatal("expected underflow error on empty stack, got nil")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 11 — copyByteCode
// ─────────────────────────────────────────────────────────────────────────────

// Test_copyByteCode_PushesDeepCopy_STACK1 verifies the STACK-1 fix: after
// copyByteCode runs, TOS is a true deep copy of the original value (not the
// integer literal 2 that the original buggy code pushed).
//
// STACK-1 fix: changed `c.push(2)` to `c.push(v2)`.
//
// Type preservation note: data.Copy preserves the exact Go type of each value,
// so a copy of int(99) stays int(99).  This is an improvement over the old
// JSON round-trip, which always produced float64 for any numeric input.
func Test_copyByteCode_PushesDeepCopy_STACK1(t *testing.T) {
	tc := newTestContext(t).withStack(99)

	err := copyByteCode(tc.ctx, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// TOS must be the deep copy: data.Copy preserves the int type.
	tos, popErr := tc.ctx.Pop()
	if popErr != nil {
		t.Fatalf("Pop failed: %v", popErr)
	}

	if tos == 2 {
		t.Fatal("STACK-1: copyByteCode still pushes integer 2 instead of the deep copy")
	}

	if tos != int(99) {
		t.Errorf("STACK-1: TOS = %v (%T), want int(99)", tos, tos)
	}
}

// Test_copyByteCode_OriginalRemainsOnStack verifies that the original value is
// preserved below the copy after copyByteCode runs.
func Test_copyByteCode_OriginalRemainsOnStack(t *testing.T) {
	tc := newTestContext(t).withStack(77)

	err := copyByteCode(tc.ctx, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Discard TOS (the deep copy, int(77)).
	_, _ = tc.ctx.Pop()

	// The original int(77) must still be below.
	tc.assertTopStack(77)
	tc.assertStackEmpty()
}

// Test_copyByteCode_EmptyStackReturnsError verifies that Copy on an empty
// stack returns a stack-underflow error.
func Test_copyByteCode_EmptyStackReturnsError(t *testing.T) {
	tc := newTestContext(t)

	err := copyByteCode(tc.ctx, nil)
	if err == nil {
		t.Fatal("expected error for Copy on empty stack, got nil")
	}

	if !errors.Equal(errors.New(err), errors.ErrStackUnderflow) {
		t.Errorf("expected ErrStackUnderflow, got %v", err)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 12 — getVarArgsByteCode
// ─────────────────────────────────────────────────────────────────────────────

// Test_getVarArgsByteCode_NoArgList verifies that when no __args variable
// exists in the symbol table, getVarArgsByteCode returns ErrInvalidVariableArguments.
// This is the error case for calling a varargs function in an unexpected context.
func Test_getVarArgsByteCode_NoArgList(t *testing.T) {
	tc := newTestContext(t)

	err := getVarArgsByteCode(tc.ctx, 0)
	if err == nil {
		t.Fatal("expected ErrInvalidVariableArguments when no arg list, got nil")
	}

	if !errors.Equal(errors.New(err), errors.ErrInvalidVariableArguments) {
		t.Errorf("expected ErrInvalidVariableArguments, got %v", err)
	}
}

// Test_getVarArgsByteCode_WithArgs verifies that when an __args array exists,
// getVarArgsByteCode pushes a sub-array starting at the given position.
//
// Example: args = ["a", "b", "c", "d"], position = 2
// → pushes ["c", "d"] (everything from index 2 onwards).
func Test_getVarArgsByteCode_WithArgs(t *testing.T) {
	// Set up __args = ["a", "b", "c", "d"].
	tc := newTestContext(t).withArgList("a", "b", "c", "d")

	err := getVarArgsByteCode(tc.ctx, 2) // start at position 2
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// TOS should be a *data.Array containing ["c", "d"].
	v, popErr := tc.ctx.Pop()
	if popErr != nil {
		t.Fatalf("Pop failed: %v", popErr)
	}

	arr, ok := v.(*data.Array)
	if !ok {
		t.Fatalf("expected *data.Array, got %T", v)
	}

	if arr.Len() != 2 {
		t.Errorf("array length = %d, want 2", arr.Len())
	}
}

// Test_getVarArgsByteCode_PositionBeyondArgs verifies that when the requested
// position is beyond the length of the args array, an empty array is pushed
// rather than an error being returned.  This matches the semantics of a
// variadic function called with no variadic arguments.
func Test_getVarArgsByteCode_PositionBeyondArgs(t *testing.T) {
	// Only 2 args; ask for position 5.
	tc := newTestContext(t).withArgList("x", "y")

	err := getVarArgsByteCode(tc.ctx, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	v, _ := tc.ctx.Pop()

	arr, ok := v.(*data.Array)
	if !ok {
		t.Fatalf("expected *data.Array, got %T", v)
	}

	if arr.Len() != 0 {
		t.Errorf("array length = %d, want 0 (empty variadic)", arr.Len())
	}
}

// Test_getVarArgsByteCode_InvalidOperand verifies that a non-integer operand
// (the arg position must be an integer) causes an error.
func Test_getVarArgsByteCode_InvalidOperand(t *testing.T) {
	tc := newTestContext(t).withArgList("x")

	err := getVarArgsByteCode(tc.ctx, "bad")
	if err == nil {
		t.Fatal("expected error for non-integer operand, got nil")
	}
}
