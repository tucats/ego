package bytecode

// callframe_test.go contains unit tests for the functions in callframe.go.
//
// # What a CallFrame is
//
// When Ego calls a function, the runtime must remember where to return to
// and what the caller's environment looked like.  A *CallFrame is a snapshot
// of the Context taken just before the call:
//
//   - The caller's program counter, bytecode stream, and symbol table
//   - The caller's module name, line number, package, and block depth
//   - The "this" receiver stack and deferred-statement stack
//   - The single-step debug flag and extensions setting
//   - The depth of the try/catch stack (so any extra entries created inside
//     the callee can be discarded when the frame is popped)
//
// The frame is pushed onto the runtime stack immediately before the callee's
// scope is activated.  When the callee returns, callFramePop reads the frame
// back off the stack and restores every saved field, then pushes any return
// values that accumulated above the frame.
//
// # Frame pointer convention
//
// After callFramePush / callFramePushWithTable the layout is:
//
//	stack index:  0 ... old_sp-1  |  old_sp       | old_sp+1 ... new_sp-1
//	              [caller items]  | *CallFrame     | [callee items]
//	              (below frame)   | framePointer-1 | (return values)
//
// c.framePointer is set to old_sp+1 (one past the frame slot).  To read
// the frame you must use stack[framePointer-1], not stack[framePointer].
//
// # How to read these tests
//
// Each test builds a testContext with newTestContext (see testhelpers_test.go),
// pushes a call frame via callFramePush or callFramePushWithTable, optionally
// places values above the frame to simulate return values, then calls
// callFramePop and asserts the context was correctly restored.

import (
	"strings"
	"testing"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// ─────────────────────────────────────────────────────────────────────────────
// Section 1: CallFrame.String()
// ─────────────────────────────────────────────────────────────────────────────
//
// String() formats a CallFrame as "module:line" (or "package.module:line",
// or "<anon>:0" when all name fields are empty).  It is used in stack-trace
// output produced by FormatFrames.

// Test_CallFrame_String_ModuleAndLine verifies the normal case: Module is set,
// Package is empty.  The result should be "module:line".
func Test_CallFrame_String_ModuleAndLine(t *testing.T) {
	f := &CallFrame{Module: "myFunc", Line: 42}

	got := f.String()
	want := "myFunc:42"

	if got != want {
		t.Errorf("String(): got %q, want %q", got, want)
	}
}

// Test_CallFrame_String_FallsBackToName verifies that when Module is empty the
// name field is used instead.  This happens for frames that were pushed before
// the module name was resolved.
func Test_CallFrame_String_FallsBackToName(t *testing.T) {
	f := &CallFrame{name: "innerFn", Line: 7}

	got := f.String()
	want := "innerFn:7"

	if got != want {
		t.Errorf("String(): got %q, want %q", got, want)
	}
}

// Test_CallFrame_String_PackagePrefixed verifies that a non-empty Package is
// prepended to the name with a dot separator.  This produces output like
// "math.Floor:15" in stack traces.
func Test_CallFrame_String_PackagePrefixed(t *testing.T) {
	f := &CallFrame{Module: "Floor", Package: "math", Line: 15}

	got := f.String()
	want := "math.Floor:15"

	if got != want {
		t.Errorf("String(): got %q, want %q", got, want)
	}
}

// Test_CallFrame_String_AllEmpty verifies that when all name fields are empty
// the anonymous placeholder defs.Anon ("<anon>") is used.
func Test_CallFrame_String_AllEmpty(t *testing.T) {
	f := &CallFrame{} // Module, name, Package all empty

	got := f.String()
	want := defs.Anon + ":0"

	if got != want {
		t.Errorf("String(): got %q, want %q", got, want)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 2: callFramePush — state saved before a function call
// ─────────────────────────────────────────────────────────────────────────────
//
// callFramePush saves the current context onto the stack as a *CallFrame, then
// sets up a fresh scope for the callee.  callFramePushWithTable does the same
// but accepts a pre-built symbol table instead of creating one from a name.

// Test_callFramePush_SavesPC verifies that the caller's program counter is
// captured in the frame and can be retrieved after a pop.
func Test_callFramePush_SavesPC(t *testing.T) {
	tc := newTestContext(t)
	tc.ctx.programCounter = 42

	targetBC := &ByteCode{name: "callee"}
	tc.ctx.callFramePush("callee scope", targetBC, 0, false)

	// After push the PC is reset to 0 (start of callee).  The saved value
	// is inside the frame on the stack.
	tc.assertProgramCounter(0)

	// Pop and check the caller's PC was restored.
	_ = tc.ctx.callFramePop()

	tc.assertProgramCounter(42)
}

// Test_callFramePush_SavesBytecode verifies that the caller's *ByteCode
// pointer is captured and restored across the push/pop cycle.
func Test_callFramePush_SavesBytecode(t *testing.T) {
	tc := newTestContext(t)
	callerBC := &ByteCode{name: "caller"}
	tc.ctx.bc = callerBC

	calleeBC := &ByteCode{name: "callee"}
	tc.ctx.callFramePush("scope", calleeBC, 0, false)

	// Inside the call c.bc should point at the callee.
	if tc.ctx.bc != calleeBC {
		t.Errorf("inside call: c.bc = %q, want %q", tc.ctx.bc.name, calleeBC.name)
	}

	_ = tc.ctx.callFramePop()

	// After return c.bc should be restored to the caller.
	if tc.ctx.bc != callerBC {
		t.Errorf("after pop: c.bc = %q, want %q", tc.ctx.bc.name, callerBC.name)
	}
}

// Test_callFramePush_SavesSymbols verifies that the caller's symbol table
// pointer is preserved and restored.
func Test_callFramePush_SavesSymbols(t *testing.T) {
	tc := newTestContext(t)
	callerSymbols := tc.ctx.symbols

	tc.ctx.callFramePush("callee", &ByteCode{}, 0, false)

	// Inside the call c.symbols is a NEW scope (child of the caller's scope).
	if tc.ctx.symbols == callerSymbols {
		t.Error("inside call: c.symbols was not replaced with a new scope")
	}

	_ = tc.ctx.callFramePop()

	if tc.ctx.symbols != callerSymbols {
		t.Error("after pop: c.symbols was not restored to the caller's scope")
	}
}

// Test_callFramePush_BoundaryTrue verifies that passing boundary=true produces
// a scope that is a scope boundary.  This prevents symbol lookups inside named
// functions from seeing the caller's local variables.
func Test_callFramePush_BoundaryTrue(t *testing.T) {
	tc := newTestContext(t)
	tc.ctx.callFramePush("fn", &ByteCode{}, 0, true /*boundary*/)

	if !tc.ctx.symbols.IsBoundary() {
		t.Error("boundary=true: new scope should be a boundary, got false")
	}
}

// Test_callFramePush_BoundaryFalse verifies that boundary=false produces a
// non-boundary scope.  Closures use this so they can read caller locals.
func Test_callFramePush_BoundaryFalse(t *testing.T) {
	tc := newTestContext(t)
	tc.ctx.callFramePush("closure", &ByteCode{}, 0, false /*boundary*/)

	if tc.ctx.symbols.IsBoundary() {
		t.Error("boundary=false: new scope should NOT be a boundary, got true")
	}
}

// Test_callFramePush_ResetsResult verifies that the callee starts with a clean
// result holder.  If the caller had already set a result the callee must not
// inherit it.
func Test_callFramePush_ResetsResult(t *testing.T) {
	tc := newTestContext(t)
	tc.ctx.result = 99
	tc.ctx.resultSet = true

	tc.ctx.callFramePush("fn", &ByteCode{}, 0, false)

	if tc.ctx.result != nil {
		t.Errorf("result after push: got %v, want nil", tc.ctx.result)
	}

	if tc.ctx.resultSet {
		t.Error("resultSet after push: got true, want false")
	}
}

// Test_callFramePush_ResetsDeferStack verifies that the callee starts with an
// empty defer stack so it never inherits the caller's deferred functions.
func Test_callFramePush_ResetsDeferStack(t *testing.T) {
	tc := newTestContext(t)
	// Simulate the caller having a deferred function registered.
	tc.ctx.deferStack = []deferStatement{{name: "caller-defer"}}

	tc.ctx.callFramePush("fn", &ByteCode{}, 0, false)

	if len(tc.ctx.deferStack) != 0 {
		t.Errorf("deferStack after push: got len %d, want 0", len(tc.ctx.deferStack))
	}
}

// Test_callFramePush_StepOverClearsSingleStep verifies the debugger "step over"
// behaviour: if singleStep=true AND stepOver=true before the call, singleStep
// is cleared so the callee runs at full speed.
func Test_callFramePush_StepOverClearsSingleStep(t *testing.T) {
	tc := newTestContext(t)
	tc.ctx.singleStep = true
	tc.ctx.stepOver = true

	tc.ctx.callFramePush("fn", &ByteCode{}, 0, false)

	if tc.ctx.singleStep {
		t.Error("singleStep was not cleared when stepOver=true")
	}
}

// Test_callFramePush_NoStepOverPreservesSingleStep verifies that when stepOver
// is false, singleStep is NOT cleared.  The debugger will continue single-
// stepping inside the callee.
func Test_callFramePush_NoStepOverPreservesSingleStep(t *testing.T) {
	tc := newTestContext(t)
	tc.ctx.singleStep = true
	tc.ctx.stepOver = false

	tc.ctx.callFramePush("fn", &ByteCode{}, 0, false)

	if !tc.ctx.singleStep {
		t.Error("singleStep was incorrectly cleared when stepOver=false")
	}
}

// Test_callFramePush_SetsFramePointerPastFrame verifies the frame-pointer
// convention described at the top of this file: after push, framePointer
// points to the slot ABOVE the stored frame, so the frame is at
// stack[framePointer-1].
func Test_callFramePush_SetsFramePointerPastFrame(t *testing.T) {
	tc := newTestContext(t)
	tc.ctx.callFramePush("fn", &ByteCode{}, 0, false)

	fp := tc.ctx.framePointer
	if fp == 0 {
		t.Fatal("framePointer was not advanced")
	}

	if _, ok := tc.ctx.stack[fp-1].(*CallFrame); !ok {
		t.Errorf("stack[framePointer-1] = %T, want *CallFrame", tc.ctx.stack[fp-1])
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 3: callFramePop — state restored after a function call
// ─────────────────────────────────────────────────────────────────────────────

// Test_callFramePop_RestoresAllContextFields verifies the full set of context
// fields that must be restored when a function returns.
func Test_callFramePop_RestoresAllContextFields(t *testing.T) {
	tc := newTestContext(t)

	// Set distinctive caller-state values in the context.
	callerBC := &ByteCode{name: "caller"}
	tc.ctx.bc = callerBC
	tc.ctx.programCounter = 77
	tc.ctx.module = "callerMod"
	tc.ctx.line = 12
	tc.ctx.pkg = "callerPkg"
	tc.ctx.blockDepth = 3
	tc.ctx.extensions = true

	callerSymbols := tc.ctx.symbols

	// Push a frame (saves all of the above).
	tc.ctx.callFramePush("callee", &ByteCode{name: "callee"}, 0, false)

	// Simulate some changes inside the callee.
	tc.ctx.programCounter = 999
	tc.ctx.module = "calleeMod"
	tc.ctx.line = 55

	// Pop the frame (should restore everything).
	if err := tc.ctx.callFramePop(); err != nil {
		t.Fatalf("callFramePop returned error: %v", err)
	}

	checks := []struct {
		name string
		got  any
		want any
	}{
		{"programCounter", tc.ctx.programCounter, 77},
		{"module", tc.ctx.module, "callerMod"},
		{"line", tc.ctx.line, 12},
		{"pkg", tc.ctx.pkg, "callerPkg"},
		{"blockDepth", tc.ctx.blockDepth, 3},
		{"extensions", tc.ctx.extensions, true},
	}

	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, c.got, c.want)
		}
	}

	if tc.ctx.symbols != callerSymbols {
		t.Error("symbols not restored to caller's scope")
	}

	if tc.ctx.bc != callerBC {
		t.Errorf("bc: got %q, want %q", tc.ctx.bc.name, callerBC.name)
	}
}

// Test_callFramePop_NoReturnValues verifies the empty-return case: when no
// values were pushed above the frame and c.resultSet is false, the stack is
// simply left empty after the pop.
func Test_callFramePop_NoReturnValues(t *testing.T) {
	tc := newTestContext(t)

	tc.ctx.callFramePush("fn", &ByteCode{}, 0, false)

	if err := tc.ctx.callFramePop(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tc.assertStackEmpty()
}

// Test_callFramePop_StackBasedReturn verifies that values pushed above the
// CallFrame during the callee's execution are moved back onto the caller's
// stack so the caller can read them as return values.
func Test_callFramePop_StackBasedReturn(t *testing.T) {
	tc := newTestContext(t)

	tc.ctx.callFramePush("fn", &ByteCode{}, 0, false)

	// Simulate the function pushing two return values above the frame.
	_ = tc.ctx.push("returnA")
	_ = tc.ctx.push("returnB")

	if err := tc.ctx.callFramePop(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Both return values should now be on the restored caller stack.
	tc.assertTopStack("returnB")
	tc.assertTopStack("returnA")
	tc.assertStackEmpty()
}

// Test_callFramePop_ResultHolderReturn verifies that when c.resultSet is true
// (the callee used a named-return or assignment-style return), the value in
// c.result is pushed onto the stack.
func Test_callFramePop_ResultHolderReturn(t *testing.T) {
	tc := newTestContext(t)

	tc.ctx.callFramePush("fn", &ByteCode{}, 0, false)

	// Simulate a single-value return via the result holder.
	tc.ctx.result = 42
	tc.ctx.resultSet = true

	if err := tc.ctx.callFramePop(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tc.assertTopStack(42)
}

// Test_callFramePop_StackReturnTakesPriorityOverResultHolder verifies the
// precedence rule: if there are items on the stack above the frame, they are
// used as the return values and the result holder is NOT pushed.  A function
// should only use one return mechanism at a time, but the stack-based path
// wins when both are present.
func Test_callFramePop_StackReturnTakesPriorityOverResultHolder(t *testing.T) {
	tc := newTestContext(t)

	tc.ctx.callFramePush("fn", &ByteCode{}, 0, false)

	// Put a value both on the stack and in the result holder.
	_ = tc.ctx.push("stack-return")
	tc.ctx.result = "holder-return"
	tc.ctx.resultSet = true

	if err := tc.ctx.callFramePop(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Only the stack value should be present.
	tc.assertTopStack("stack-return")
	tc.assertStackEmpty()
}

// Test_callFramePop_TruncatesTryStack verifies that try/catch entries created
// inside the callee are discarded when the frame is popped.  This prevents
// stale exception handlers from intercepting errors in the caller.
func Test_callFramePop_TruncatesTryStack(t *testing.T) {
	tc := newTestContext(t)

	// The caller has one try entry at the time of the call.
	tc.ctx.tryStack = []tryInfo{{addr: 10}}

	tc.ctx.callFramePush("fn", &ByteCode{}, 0, false)

	// The callee adds two more try entries.
	tc.ctx.tryStack = append(tc.ctx.tryStack, tryInfo{addr: 20})
	tc.ctx.tryStack = append(tc.ctx.tryStack, tryInfo{addr: 30})

	if len(tc.ctx.tryStack) != 3 {
		t.Fatalf("try stack before pop: got %d entries, want 3", len(tc.ctx.tryStack))
	}

	if err := tc.ctx.callFramePop(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Only the caller's original entry should remain.
	if len(tc.ctx.tryStack) != 1 {
		t.Errorf("try stack after pop: got %d entries, want 1", len(tc.ctx.tryStack))
	}

	if tc.ctx.tryStack[0].addr != 10 {
		t.Errorf("try stack[0].addr: got %d, want 10", tc.ctx.tryStack[0].addr)
	}
}

// Test_callFramePop_PreservesTryStackWhenNoExtras verifies that if the callee
// did NOT add any try entries, the caller's original try stack is left intact.
func Test_callFramePop_PreservesTryStackWhenNoExtras(t *testing.T) {
	tc := newTestContext(t)

	tc.ctx.tryStack = []tryInfo{{addr: 5}, {addr: 6}}

	tc.ctx.callFramePush("fn", &ByteCode{}, 0, false)

	// Callee adds nothing to the try stack.

	if err := tc.ctx.callFramePop(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(tc.ctx.tryStack) != 2 {
		t.Errorf("try stack after pop: got %d entries, want 2", len(tc.ctx.tryStack))
	}
}

// Test_callFramePop_ErrorOnEmptyStack verifies that popping when no call frame
// is on the stack returns an error.
//
// Implementation detail: callFramePop first resets the stack pointer to
// c.framePointer (0 for an empty context), then calls c.Pop().  Pop() detects
// the underflow and returns ErrStackUnderflow before the *CallFrame check is
// reached.  Therefore the error is ErrStackUnderflow, not ErrInvalidCallFrame.
func Test_callFramePop_ErrorOnEmptyStack(t *testing.T) {
	tc := newTestContext(t)
	// No callFramePush was done; the stack has no *CallFrame.

	err := tc.ctx.callFramePop()

	tc.assertError(err, errors.ErrStackUnderflow)
}

// Test_callFramePop_ErrorWhenNonFrameOnStack verifies that if a non-*CallFrame
// value sits at the frame-pointer position, ErrInvalidCallFrame is returned.
func Test_callFramePop_ErrorWhenNonFrameOnStack(t *testing.T) {
	tc := newTestContext(t)

	// Push a plain integer where a *CallFrame is expected.
	_ = tc.ctx.push(12345)
	tc.ctx.framePointer = tc.ctx.stackPointer

	err := tc.ctx.callFramePop()

	tc.assertError(err, errors.ErrInvalidCallFrame)
}

// Test_callFramePop_RestoresReceiverStack verifies that the receiver ("this")
// stack is correctly restored.  This matters when a method call pushes a
// receiver; after the call returns the receiver stack must be what it was
// before the call.
func Test_callFramePop_RestoresReceiverStack(t *testing.T) {
	tc := newTestContext(t)

	// Simulate the caller having one receiver already on the stack.
	tc.ctx.receiverStack = []this{{name: "callerReceiver", value: "x"}}

	tc.ctx.callFramePush("fn", &ByteCode{}, 0, false)

	// Inside the callee, add another receiver.
	tc.ctx.receiverStack = append(tc.ctx.receiverStack,
		this{name: "calleeReceiver", value: "y"})

	if err := tc.ctx.callFramePop(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Only the caller's receiver should remain.
	if len(tc.ctx.receiverStack) != 1 {
		t.Errorf("receiverStack length: got %d, want 1", len(tc.ctx.receiverStack))
	}

	if tc.ctx.receiverStack[0].name != "callerReceiver" {
		t.Errorf("receiverStack[0].name: got %q, want %q",
			tc.ctx.receiverStack[0].name, "callerReceiver")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 4: SetBreakOnReturn — debugger break-on-return flag
// ─────────────────────────────────────────────────────────────────────────────
//
// SetBreakOnReturn marks the current call frame so the debugger halts when
// the active function returns.  It is called by the debugger "step out"
// (finish) command.
//
// The fix for CALL-6 corrected both stack accesses from
// c.stack[c.framePointer] to c.stack[c.framePointer-1], matching the
// frame-pointer convention used by every other frame-reading function.

// Test_SetBreakOnReturn_SetsBreakOnReturnFlag verifies that after the
// CALL-6 fix, SetBreakOnReturn correctly marks the call frame so that
// when the function returns c.breakOnReturn is true.
//
// The test pushes a frame, calls SetBreakOnReturn, pops the frame, and
// confirms the flag was propagated from the stored CallFrame to the context.
func Test_SetBreakOnReturn_SetsBreakOnReturnFlag(t *testing.T) {
	tc := newTestContext(t)

	tc.ctx.callFramePush("fn", &ByteCode{}, 0, false)

	// Mark the frame so the debugger will halt when this function returns.
	tc.ctx.SetBreakOnReturn()

	// callFramePop restores the caller's context, including breakOnReturn.
	if err := tc.ctx.callFramePop(); err != nil {
		t.Fatalf("callFramePop: unexpected error: %v", err)
	}

	// After the fix, breakOnReturn must be true.
	if !tc.ctx.breakOnReturn {
		t.Error("breakOnReturn is false after SetBreakOnReturn + callFramePop: CALL-6 fix may not be applied")
	}
}

// Test_SetBreakOnReturn_FrameAtFPMinusOne verifies the frame-pointer
// convention: the *CallFrame is stored at stack[framePointer-1], and
// stack[framePointer] (one slot higher) holds no *CallFrame.  This is the
// slot that the pre-fix code was (incorrectly) reading.
func Test_SetBreakOnReturn_FrameAtFPMinusOne(t *testing.T) {
	tc := newTestContext(t)
	tc.ctx.callFramePush("fn", &ByteCode{}, 0, false)

	fp := tc.ctx.framePointer

	_, atCorrectSlot := tc.ctx.stack[fp-1].(*CallFrame)
	_, atHigherSlot := tc.ctx.stack[fp].(*CallFrame)

	if !atCorrectSlot {
		t.Errorf("stack[framePointer-1] = %T, want *CallFrame", tc.ctx.stack[fp-1])
	}

	// The slot ABOVE the frame holds the first callee item (or nil), never the frame.
	if atHigherSlot {
		t.Errorf("stack[framePointer] unexpectedly holds a *CallFrame")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 5: FormatFrames — human-readable call-stack display
// ─────────────────────────────────────────────────────────────────────────────
//
// FormatFrames builds a multi-line string showing where execution is and how
// it got there.  It is used by the interactive debugger's "where" command.

// Test_FormatFrames_NilContext verifies that calling FormatFrames on a nil
// *Context returns a safe placeholder string rather than panicking.
func Test_FormatFrames_NilContext(t *testing.T) {
	var c *Context

	got := c.FormatFrames(1)

	want := "<no context available>\n"
	if got != want {
		t.Errorf("FormatFrames(nil): got %q, want %q", got, want)
	}
}

// Test_FormatFrames_NoCallFrames verifies the output when there are no saved
// call frames — the function shows only the current execution point.
func Test_FormatFrames_NoCallFrames(t *testing.T) {
	tc := newTestContext(t)
	tc.ctx.module = "main"
	tc.ctx.line = 10
	tc.ctx.bc = &ByteCode{name: "main"}

	got := tc.ctx.FormatFrames(1)

	// Output must start with "Call frames:" and contain an "at:" line.
	if !strings.HasPrefix(got, "Call frames:\n") {
		t.Errorf("expected output to start with 'Call frames:\\n', got:\n%s", got)
	}

	if !strings.Contains(got, "at:") {
		t.Errorf("expected 'at:' in output, got:\n%s", got)
	}
}

// Test_FormatFrames_ShowsCallerAfterPush verifies that when a call frame is
// present on the stack, FormatFrames includes a "from:" line showing where the
// call originated.
func Test_FormatFrames_ShowsCallerAfterPush(t *testing.T) {
	tc := newTestContext(t)
	tc.ctx.module = "caller"
	tc.ctx.line = 20

	tc.ctx.callFramePush("callee", &ByteCode{name: "callee"}, 0, false)

	tc.ctx.module = "callee"
	tc.ctx.line = 5

	got := tc.ctx.FormatFrames(ShowAllCallFrames)

	if !strings.Contains(got, "from:") {
		t.Errorf("expected 'from:' line in output, got:\n%s", got)
	}
}

// Test_FormatFrames_IncludeSymbolTableNames verifies the IncludeSymbolTableNames
// constant: when maxDepth == IncludeSymbolTableNames, each frame appends the
// symbol-table name in parentheses.
func Test_FormatFrames_IncludeSymbolTableNames(t *testing.T) {
	tc := newTestContext(t)
	tc.ctx.bc = &ByteCode{name: "main"}

	got := tc.ctx.FormatFrames(IncludeSymbolTableNames)

	// The current table name should appear in parentheses.
	if !strings.Contains(got, "(") {
		t.Errorf("expected symbol table name in output, got:\n%s", got)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 6: GetFrame — retrieve frame information by depth
// ─────────────────────────────────────────────────────────────────────────────

// Test_GetFrame_NilContext verifies that GetFrame on a nil *Context returns
// safe sentinel values rather than panicking.
func Test_GetFrame_NilContext(t *testing.T) {
	var c *Context

	module, line, table := c.GetFrame(0)

	if module != "<none>" || line != 0 || table != "<none>" {
		t.Errorf("GetFrame(nil): got (%q, %d, %q), want (<none>, 0, <none>)",
			module, line, table)
	}
}

// Test_GetFrame_Depth0_ReturnsCurrentContext verifies that depth 0 returns the
// module, line, and symbol-table name of the currently executing context — not
// any of the saved frames.
func Test_GetFrame_Depth0_ReturnsCurrentContext(t *testing.T) {
	tc := newTestContext(t)
	tc.ctx.module = "activeModule"
	tc.ctx.line = 33

	module, line, table := tc.ctx.GetFrame(0)

	if module != "activeModule" {
		t.Errorf("module: got %q, want %q", module, "activeModule")
	}

	if line != 33 {
		t.Errorf("line: got %d, want %d", line, 33)
	}

	if table != tc.ctx.symbols.Name {
		t.Errorf("table: got %q, want %q", table, tc.ctx.symbols.Name)
	}
}

// Test_GetFrame_Depth1_ReturnsSavedFrame verifies that after pushing a call
// frame, depth 1 returns the MODULE and LINE that were active at the time of
// the push (the caller's location), not the current execution point.
func Test_GetFrame_Depth1_ReturnsSavedFrame(t *testing.T) {
	tc := newTestContext(t)
	tc.ctx.module = "callerMod"
	tc.ctx.line = 99

	// The push saves the current module/line into the CallFrame.
	tc.ctx.callFramePush("callee", &ByteCode{}, 0, false)

	// While inside the callee, overwrite the current module/line.
	tc.ctx.module = "callee"
	tc.ctx.line = 1

	module, line, _ := tc.ctx.GetFrame(1)

	if module != "callerMod" {
		t.Errorf("depth 1 module: got %q, want %q", module, "callerMod")
	}

	if line != 99 {
		t.Errorf("depth 1 line: got %d, want %d", line, 99)
	}
}

// Test_GetFrame_DepthBeyondFrames verifies that requesting a depth deeper than
// the actual call stack returns empty strings (graceful "not found" behavior).
func Test_GetFrame_DepthBeyondFrames(t *testing.T) {
	tc := newTestContext(t)
	// No frames pushed; depth 1 doesn't exist.

	module, line, table := tc.ctx.GetFrame(1)

	if module != "" || line != 0 || table != "" {
		t.Errorf("beyond frames: got (%q, %d, %q), want empty values", module, line, table)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 7: immutableValue helper
// ─────────────────────────────────────────────────────────────────────────────
//
// immutableValue returns true for values that must not be overwritten when
// package symbols are written back to the package on function return.

// Test_immutableValue_Immutable verifies that a *data.Immutable is recognized
// as immutable.
func Test_immutableValue_Immutable(t *testing.T) {
	imm := &data.Immutable{Value: 42}

	if !immutableValue(imm) {
		t.Error("*data.Immutable: expected true, got false")
	}
}

// Test_immutableValue_ByteCode verifies that a *ByteCode is recognized as
// immutable.  Function bodies stored in packages must not be overwritten by
// local symbol changes.
func Test_immutableValue_ByteCode(t *testing.T) {
	bc := &ByteCode{name: "someFunc"}

	if !immutableValue(bc) {
		t.Error("*ByteCode: expected true, got false")
	}
}

// Test_immutableValue_PlainInt verifies that a plain int is NOT immutable.
func Test_immutableValue_PlainInt(t *testing.T) {
	if immutableValue(42) {
		t.Error("int: expected false, got true")
	}
}

// Test_immutableValue_String verifies that a string is NOT immutable.
func Test_immutableValue_String(t *testing.T) {
	if immutableValue("hello") {
		t.Error("string: expected false, got true")
	}
}

// Test_immutableValue_Nil verifies that nil is NOT considered immutable.
func Test_immutableValue_Nil(t *testing.T) {
	if immutableValue(nil) {
		t.Error("nil: expected false, got true")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Section 8: updatePackageFromLocalSymbols — symbol writeback on return
// ─────────────────────────────────────────────────────────────────────────────
//
// When a function that was called with a package receiver modifies exported
// symbols, those changes must be written back to the package object when the
// function returns.  updatePackageFromLocalSymbols handles this.

// Test_updatePackage_NopWhenNotClone verifies that a regular (non-clone) table
// causes updatePackageFromLocalSymbols to do nothing.  Only clone tables
// produced by package-method calls should trigger the writeback.
func Test_updatePackage_NopWhenNotClone(t *testing.T) {
	tc := newTestContext(t)

	// A plain symbol table is not a clone; no writeback should happen.
	plain := symbols.NewChildSymbolTable("plain", tc.ctx.symbols)
	plain.SetAlways("ExportedVar", "original")

	updatePackageFromLocalSymbols(tc.ctx, plain)

	// The original value is unchanged (writeback was skipped).
	v, _ := plain.Get("ExportedVar")
	if v != "original" {
		t.Errorf("unexpected modification: got %v, want %q", v, "original")
	}
}

// Test_updatePackage_NopWhenPackageNameEmpty verifies that even a clone with
// an empty package name is skipped — there is no package to write back to.
func Test_updatePackage_NopWhenPackageNameEmpty(t *testing.T) {
	tc := newTestContext(t)

	// Build a clone (IsClone() returns true) but leave the package name empty.
	parent := symbols.NewChildSymbolTable("parent", tc.ctx.symbols)
	clone := parent.Clone(tc.ctx.symbols) // IsClone() == true, forPackage == ""

	clone.SetAlways("ExportedValue", 99)

	// Should return silently without panicking.
	updatePackageFromLocalSymbols(tc.ctx, clone)
}

// Test_updatePackage_WritesBackExportedSymbols verifies the full writeback
// path: exported (capitalized) symbols in a modified clone of a package's
// symbol table are written back to the package object.
func Test_updatePackage_WritesBackExportedSymbols(t *testing.T) {
	tc := newTestContext(t)

	// Create a package and register it in the global (root) scope so that
	// updatePackageFromLocalSymbols can find it via c.symbols.FindNextScope().
	pkg := data.NewPackage("util", "util")
	pkg.Set("Counter", 0)

	// Register the package in the root table (which is what FindNextScope
	// returns for the test's local symbol table).
	tc.ctx.symbols.Root().SetAlways("util", pkg)

	// Build a clone of a symbol table that is associated with the "util" package.
	packageTable := symbols.NewChildSymbolTable("package util", tc.ctx.symbols)
	packageTable.SetPackage("util")

	clone := packageTable.Clone(tc.ctx.symbols)
	clone.SetPackage("util")

	// Modify an exported symbol inside the clone (simulating a package method
	// modifying a package-level variable).
	clone.SetAlways("Counter", 42) // triggers modified=true

	updatePackageFromLocalSymbols(tc.ctx, clone)

	// The package object should now have the updated value.
	v, found := pkg.Get("Counter")
	if !found {
		t.Fatal("Counter not found in package after writeback")
	}

	if v != 42 {
		t.Errorf("Counter in package: got %v, want 42", v)
	}
}

// Test_updatePackage_SkipsNonExportedSymbols verifies that symbols whose names
// do NOT start with a capital letter are NOT written back to the package.
// Only exported (public) names should propagate.
func Test_updatePackage_SkipsNonExportedSymbols(t *testing.T) {
	tc := newTestContext(t)

	pkg := data.NewPackage("util", "util")
	pkg.Set("Counter", 0)
	pkg.Set("internal", "original")

	tc.ctx.symbols.Root().SetAlways("util", pkg)

	packageTable := symbols.NewChildSymbolTable("package util", tc.ctx.symbols)
	packageTable.SetPackage("util")

	clone := packageTable.Clone(tc.ctx.symbols)
	clone.SetPackage("util")

	// Only modify the unexported name.
	clone.SetAlways("internal", "modified")

	updatePackageFromLocalSymbols(tc.ctx, clone)

	// The unexported symbol must NOT have been written back.
	v, _ := pkg.Get("internal")
	if v == "modified" {
		t.Error("unexported symbol 'internal' was incorrectly written back to the package")
	}
}
