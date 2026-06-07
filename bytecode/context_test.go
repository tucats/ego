package bytecode

import (
	"reflect"
	"testing"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/tokenizer"
)

func TestNewContext(t *testing.T) {
	s := symbols.NewSymbolTable("context test")
	b := New("context test")

	s.SetAlways("foo", 1)

	b.Emit(AtLine, 100)
	b.Emit(Load, "foo")
	b.Emit(Push, 2)
	b.Emit(Add)
	b.Emit(Return, 1)

	// Create a new context to test and validate the initial state
	c := NewContext(s, b)

	if c.symbols != s {
		t.Error("Symbol table not set")
	}

	if c.bc != b {
		t.Error("bytecode not set")
	}

	if len(c.stack) != initialStackSize {
		t.Error("stack not sized correctly")
	}

	if c.stackPointer != 0 {
		t.Error("stack not empty before run")
	}

	e := c.setConstant("xyzzy", "frobozz")
	if e != nil {
		t.Errorf("Unexpected constant set error: %v", e)
	}

	if !c.isConstant("xyzzy") {
		t.Error("symbol not seen as constant")
	}

	if c.isConstant("zork") {
		t.Error("unknown symbol was constant")
	}

	r, found := c.get("xyzzy")
	if !found {
		t.Error("Failed to find previously created constant")
	}

	// Note that the value of a constant is an immutable object. So
	// unwrap it for the comparison.
	if !reflect.DeepEqual(data.UnwrapConstant(r), "frobozz") {
		t.Errorf("Retrieval of constant has wrong value: %#v", r)
	}
	// Change some of the attributes of the context and validate
	if !c.fullSymbolScope {
		t.Error("failed to initialize full symbol scope")
	}

	c.SetFullSymbolScope(false)

	if c.fullSymbolScope {
		t.Error("failed to clear full symbol scope")
	}

	c.SetDebug(true)

	if !c.debugging {
		t.Error("Failed to set debugging flag")
	}

	c.SetDebug(false)

	if c.debugging {
		t.Error("Failed to clear debugging flag")
	}

	c.EnableConsoleOutput(false)

	if c.captureBuffer == nil {
		t.Error("Failed to set non-empty console output")
	}

	c.captureBuffer.WriteString("foobar")

	o := c.GetOutput()
	if o != "foobar" {
		t.Errorf("Incorrect captured console text: %v", o)
	}

	c.EnableConsoleOutput(true)

	if c.captureBuffer != nil {
		t.Error("Failed to clear console output state")
	}

	// Now run the short segment of bytecode, and see what the ending
	// context state looks like.
	e = c.Run()
	if !errors.Nil(e) && e.Error() != errors.ErrStop.Error() {
		t.Errorf("Failed to run bytecode: %v", e)
	}

	_, e = c.Pop()

	if errors.Nil(e) {
		t.Errorf("Expected stack underflow not found")
	}

	r = c.Result()
	if !reflect.DeepEqual(r, 3) {
		t.Errorf("Wrong final result: %v", r)
	}

	if c.stackPointer != 0 {
		t.Error("stack not empty after run")
	}

	if c.GetLine() != 100 {
		t.Errorf("Incorrect line number: %v", c.GetLine())
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Section: GetName
// ─────────────────────────────────────────────────────────────────────────────

func Test_Context_GetName_WithBytecode(t *testing.T) {
	ctx := NewContext(symbols.NewSymbolTable("test"), New("myFunc"))

	if ctx.GetName() != "myFunc" {
		t.Errorf("GetName: got %q, want \"myFunc\"", ctx.GetName())
	}
}

func Test_Context_GetName_NilBytecode(t *testing.T) {
	ctx := NewContext(symbols.NewSymbolTable("test"), nil)

	if ctx.GetName() != defs.Main {
		t.Errorf("GetName with nil bc: got %q, want %q", ctx.GetName(), defs.Main)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Section: GetModuleName — CONTEXT-1 fix
// ─────────────────────────────────────────────────────────────────────────────

func Test_Context_GetModuleName_WithBytecode(t *testing.T) {
	ctx := NewContext(symbols.NewSymbolTable("test"), New("myModule"))

	if ctx.GetModuleName() != "myModule" {
		t.Errorf("GetModuleName: got %q, want \"myModule\"", ctx.GetModuleName())
	}
}

// Test_Context_GetModuleName_NilBytecode verifies that GetModuleName does not
// panic when the context was created with a nil bytecode object.
// Before the CONTEXT-1 fix this panicked with a nil pointer dereference.
func Test_Context_GetModuleName_NilBytecode(t *testing.T) {
	ctx := NewContext(symbols.NewSymbolTable("test"), nil)

	got := ctx.GetModuleName() // must not panic
	if got != defs.Main {
		t.Errorf("GetModuleName with nil bc: got %q, want %q", got, defs.Main)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Section: NewContext edge cases
// ─────────────────────────────────────────────────────────────────────────────

func Test_NewContext_NilSymbolTable_CreatesEmpty(t *testing.T) {
	ctx := NewContext(nil, New("test"))

	if ctx.symbols == nil {
		t.Error("NewContext with nil symbol table should create an empty table")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Section: SetExtensions
// ─────────────────────────────────────────────────────────────────────────────

func Test_Context_SetExtensions_True(t *testing.T) {
	ctx := NewContext(symbols.NewSymbolTable("test"), New("test"))
	ctx.SetExtensions(true)

	if !ctx.extensions {
		t.Error("SetExtensions(true) did not set the extensions flag")
	}
}

func Test_Context_SetExtensions_False(t *testing.T) {
	ctx := NewContext(symbols.NewSymbolTable("test"), New("test"))
	ctx.SetExtensions(true)
	ctx.SetExtensions(false)

	if ctx.extensions {
		t.Error("SetExtensions(false) did not clear the extensions flag")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Section: SetTrace / Tracing
// ─────────────────────────────────────────────────────────────────────────────

func Test_Context_SetTrace_EnablesTracing(t *testing.T) {
	ctx := NewContext(symbols.NewSymbolTable("test"), New("test"))
	ctx.SetTrace(true)

	if !ctx.Tracing() {
		t.Error("SetTrace(true) should make Tracing() return true")
	}
}

func Test_Context_SetTrace_DisablesTracing(t *testing.T) {
	ctx := NewContext(symbols.NewSymbolTable("test"), New("test"))
	ctx.SetTrace(true)
	ctx.SetTrace(false)

	if ctx.tracing {
		t.Error("SetTrace(false) should clear the tracing flag")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Section: SetPC
// ─────────────────────────────────────────────────────────────────────────────

func Test_Context_SetPC_UpdatesProgramCounter(t *testing.T) {
	ctx := NewContext(symbols.NewSymbolTable("test"), New("test"))
	ctx.SetPC(42)

	if ctx.programCounter != 42 {
		t.Errorf("SetPC(42): programCounter = %d, want 42", ctx.programCounter)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Section: SetGlobal
// ─────────────────────────────────────────────────────────────────────────────

func Test_Context_SetGlobal_StoresInRootTable(t *testing.T) {
	// NewChildSymbolTable(name, nil) creates a true root (isRoot=true, parent=nil).
	// NewSymbolTable always parents to the package-level RootSymbolTable, so
	// Root() would traverse past our test table to the package root.
	root := symbols.NewChildSymbolTable("root", nil)
	child := symbols.NewChildSymbolTable("child", root)

	root.SetAlways("globalVar", 0)

	ctx := NewContext(child, New("test"))

	if err := ctx.SetGlobal("globalVar", 99); err != nil {
		t.Fatalf("SetGlobal returned unexpected error: %v", err)
	}

	v, found := root.Get("globalVar")
	if !found {
		t.Fatal("SetGlobal: variable not found in root table after set")
	}

	if v != 99 {
		t.Errorf("SetGlobal: root value = %v, want 99", v)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Section: ClearOutput
// ─────────────────────────────────────────────────────────────────────────────

func Test_Context_ClearOutput_ResetsCapture(t *testing.T) {
	ctx := NewContext(symbols.NewSymbolTable("test"), New("test"))
	ctx.EnableConsoleOutput(false)
	ctx.captureBuffer.WriteString("some output")
	ctx.ClearOutput()

	if got := ctx.GetOutput(); got != "" {
		t.Errorf("ClearOutput: buffer not empty after clear, got %q", got)
	}
}

func Test_Context_ClearOutput_WhenNotCapturing_IsNoOp(t *testing.T) {
	ctx := NewContext(symbols.NewSymbolTable("test"), New("test"))
	ctx.ClearOutput() // captureBuffer is nil — must not panic
}

// ─────────────────────────────────────────────────────────────────────────────
// Section: GetOutput
// ─────────────────────────────────────────────────────────────────────────────

func Test_Context_GetOutput_WhenNotCapturing_ReturnsEmpty(t *testing.T) {
	ctx := NewContext(symbols.NewSymbolTable("test"), New("test"))

	if got := ctx.GetOutput(); got != "" {
		t.Errorf("GetOutput without capture active: got %q, want empty", got)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Section: SetTokenizer / GetTokenizer
// ─────────────────────────────────────────────────────────────────────────────

func Test_Context_SetTokenizer_NilRoundtrip(t *testing.T) {
	ctx := NewContext(symbols.NewSymbolTable("test"), New("test"))
	ctx.SetTokenizer(nil)

	if ctx.GetTokenizer() != nil {
		t.Error("GetTokenizer: expected nil after SetTokenizer(nil)")
	}
}

func Test_Context_SetTokenizer_NonNilRoundtrip(t *testing.T) {
	ctx := NewContext(symbols.NewSymbolTable("test"), New("test"))
	tok := tokenizer.New("x := 1", false)
	ctx.SetTokenizer(tok)

	if ctx.GetTokenizer() != tok {
		t.Error("GetTokenizer: did not return the stored tokenizer")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Section: GetSource
// ─────────────────────────────────────────────────────────────────────────────

func Test_Context_GetSource_InitiallyEmpty(t *testing.T) {
	ctx := NewContext(symbols.NewSymbolTable("test"), New("test"))

	if got := ctx.GetSource(); got != "" {
		t.Errorf("GetSource: expected empty on fresh context, got %q", got)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Section: AppendSymbols
// ─────────────────────────────────────────────────────────────────────────────

func Test_Context_AppendSymbols_CopiesAllSymbols(t *testing.T) {
	ctx := NewContext(symbols.NewSymbolTable("test"), New("test"))

	extra := symbols.NewSymbolTable("extra")
	extra.SetAlways("alpha", 1)
	extra.SetAlways("beta", "two")

	ctx.AppendSymbols(extra)

	v, found := ctx.get("alpha")
	if !found || v != 1 {
		t.Errorf("AppendSymbols: alpha = %v (found=%v), want 1 (true)", v, found)
	}

	v, found = ctx.get("beta")
	if !found || v != "two" {
		t.Errorf("AppendSymbols: beta = %v (found=%v), want \"two\" (true)", v, found)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Section: SetByteCode
// ─────────────────────────────────────────────────────────────────────────────

func Test_Context_SetByteCode_UpdatesBC(t *testing.T) {
	ctx := NewContext(symbols.NewSymbolTable("test"), New("original"))
	b2 := New("replacement")
	ctx.SetByteCode(b2)

	if ctx.bc != b2 {
		t.Error("SetByteCode: bc field was not updated to the new bytecode")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Section: SetSingleStep / SingleStep
// ─────────────────────────────────────────────────────────────────────────────

func Test_Context_SingleStep_FalseByDefault(t *testing.T) {
	ctx := NewContext(symbols.NewSymbolTable("test"), New("test"))

	if ctx.SingleStep() {
		t.Error("SingleStep: expected false for a fresh context")
	}
}

func Test_Context_SetSingleStep_True(t *testing.T) {
	ctx := NewContext(symbols.NewSymbolTable("test"), New("test"))
	ctx.SetSingleStep(true)

	if !ctx.SingleStep() {
		t.Error("SetSingleStep(true): SingleStep() should return true")
	}
}

func Test_Context_SetSingleStep_False(t *testing.T) {
	ctx := NewContext(symbols.NewSymbolTable("test"), New("test"))
	ctx.SetSingleStep(true)
	ctx.SetSingleStep(false)

	if ctx.SingleStep() {
		t.Error("SetSingleStep(false): SingleStep() should return false")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Section: SetStepOver
// ─────────────────────────────────────────────────────────────────────────────

func Test_Context_SetStepOver_True(t *testing.T) {
	ctx := NewContext(symbols.NewSymbolTable("test"), New("test"))
	ctx.SetStepOver(true)

	if !ctx.stepOver {
		t.Error("SetStepOver(true): stepOver flag was not set")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Section: SetDebug — CONTEXT-2 fix
// ─────────────────────────────────────────────────────────────────────────────

// Test_Context_SetDebug_True_SetsBothFlags verifies that SetDebug(true) enables
// both the debugging flag and single-step mode (CONTEXT-2 fix).
func Test_Context_SetDebug_True_SetsBothFlags(t *testing.T) {
	ctx := NewContext(symbols.NewSymbolTable("test"), New("test"))
	ctx.SetDebug(true)

	if !ctx.debugging {
		t.Error("SetDebug(true): debugging flag not set")
	}

	if !ctx.singleStep {
		t.Error("SetDebug(true): singleStep flag not set")
	}
}

// Test_Context_SetDebug_False_ClearsBothFlags verifies that SetDebug(false)
// clears both the debugging flag and single-step mode (CONTEXT-2 fix).
// Before the fix, SetDebug always wrote singleStep = true regardless of b,
// so SetDebug(false) left singleStep enabled.
func Test_Context_SetDebug_False_ClearsBothFlags(t *testing.T) {
	ctx := NewContext(symbols.NewSymbolTable("test"), New("test"))
	ctx.SetDebug(true)
	ctx.SetDebug(false)

	if ctx.debugging {
		t.Error("SetDebug(false): debugging flag not cleared")
	}

	if ctx.singleStep {
		t.Error("SetDebug(false): singleStep flag not cleared (CONTEXT-2)")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Section: Sandboxed
// ─────────────────────────────────────────────────────────────────────────────

func Test_Context_Sandboxed_True_SetsSandboxedIO(t *testing.T) {
	ctx := NewContext(symbols.NewSymbolTable("test"), New("test"))
	ctx.Sandboxed(true)

	if !ctx.sandboxedIO.Load() {
		t.Error("Sandboxed(true): sandboxedIO should be set")
	}
}

func Test_Context_Sandboxed_False_ClearsSandboxedIO(t *testing.T) {
	ctx := NewContext(symbols.NewSymbolTable("test"), New("test"))
	ctx.Sandboxed(true)
	ctx.Sandboxed(false)

	if ctx.sandboxedIO.Load() {
		t.Error("Sandboxed(false): sandboxedIO should be cleared")
	}
}

func Test_Context_Sandboxed_NilReceiver_ReturnsNil(t *testing.T) {
	var ctx *Context
	result := ctx.Sandboxed(true) // must not panic

	if result != nil {
		t.Error("Sandboxed on nil receiver should return nil")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Section: Pop / PopWithoutUnwrapping
// ─────────────────────────────────────────────────────────────────────────────

func Test_Context_Pop_UnwrapsImmutable(t *testing.T) {
	ctx := NewContext(symbols.NewSymbolTable("test"), New("test"))
	_ = ctx.push(data.Immutable{Value: 42})

	v, err := ctx.Pop()
	if err != nil {
		t.Fatalf("Pop returned unexpected error: %v", err)
	}

	if v != 42 {
		t.Errorf("Pop: expected unwrapped 42, got %v (%T)", v, v)
	}
}

func Test_Context_PopWithoutUnwrapping_PreservesImmutable(t *testing.T) {
	ctx := NewContext(symbols.NewSymbolTable("test"), New("test"))
	imm := data.Immutable{Value: 42}
	_ = ctx.push(imm)

	v, err := ctx.PopWithoutUnwrapping()
	if err != nil {
		t.Fatalf("PopWithoutUnwrapping returned unexpected error: %v", err)
	}

	if !reflect.DeepEqual(v, imm) {
		t.Errorf("PopWithoutUnwrapping: expected Immutable{42}, got %v (%T)", v, v)
	}
}

func Test_Context_PopWithoutUnwrapping_Underflow(t *testing.T) {
	ctx := NewContext(symbols.NewSymbolTable("test"), New("test"))

	_, err := ctx.PopWithoutUnwrapping()
	if err == nil {
		t.Error("PopWithoutUnwrapping on empty stack: expected underflow error, got nil")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Section: GetSymbols
// ─────────────────────────────────────────────────────────────────────────────

func Test_Context_GetSymbols_ReturnsTable(t *testing.T) {
	s := symbols.NewSymbolTable("test")
	ctx := NewContext(s, New("test"))

	if ctx.GetSymbols() == nil {
		t.Error("GetSymbols: returned nil for a properly initialized context")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Section: push — stack growth and MaxStackSize tracking
// ─────────────────────────────────────────────────────────────────────────────

func Test_Context_push_GrowsStackBeyondInitialSize(t *testing.T) {
	ctx := NewContext(symbols.NewSymbolTable("test"), New("test"))

	for i := 0; i <= initialStackSize; i++ {
		if err := ctx.push(i); err != nil {
			t.Fatalf("push(%d) returned error: %v", i, err)
		}
	}

	if len(ctx.stack) <= initialStackSize {
		t.Errorf("stack did not grow: len = %d, want > %d", len(ctx.stack), initialStackSize)
	}

	if ctx.stackPointer != initialStackSize+1 {
		t.Errorf("stackPointer = %d after %d pushes, want %d",
			ctx.stackPointer, initialStackSize+1, initialStackSize+1)
	}
}

func Test_Context_push_UpdatesMaxStackSize(t *testing.T) {
	ctx := NewContext(symbols.NewSymbolTable("test"), New("test"))
	_ = ctx.push(1)
	_ = ctx.push(2)
	_ = ctx.push(3)

	if int(MaxStackSize) < 3 {
		t.Errorf("MaxStackSize = %d after three pushes, want >= 3", MaxStackSize)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Section: unwrapConstant
// ─────────────────────────────────────────────────────────────────────────────

func Test_Context_unwrapConstant_ImmutableValue(t *testing.T) {
	const greeting = "hello"

	ctx := NewContext(symbols.NewSymbolTable("test"), New("test"))

	got := ctx.unwrapConstant(data.Immutable{Value: greeting})
	if got != greeting {
		t.Errorf("unwrapConstant(Immutable): got %v, want \"hello\"", got)
	}
}

func Test_Context_unwrapConstant_PlainValue_Unchanged(t *testing.T) {
	ctx := NewContext(symbols.NewSymbolTable("test"), New("test"))

	got := ctx.unwrapConstant(42)
	if got != 42 {
		t.Errorf("unwrapConstant(plain): value changed: got %v, want 42", got)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Section: popSymbolTable
// ─────────────────────────────────────────────────────────────────────────────

func Test_Context_popSymbolTable_Normal(t *testing.T) {
	root := symbols.NewSymbolTable("root")
	child := symbols.NewChildSymbolTable("child", root)
	ctx := NewContext(root, New("test"))
	ctx.symbols = child // simulate a pushed scope

	if err := ctx.popSymbolTable(); err != nil {
		t.Fatalf("popSymbolTable: unexpected error: %v", err)
	}

	if ctx.symbols != root {
		t.Errorf("popSymbolTable: symbols = %q, want root table", ctx.symbols.Name)
	}
}

func Test_Context_popSymbolTable_OnRoot_ReturnsError(t *testing.T) {
	// NewChildSymbolTable(name, nil) creates a true root (isRoot=true).
	// NewSymbolTable would parent to the package-level RootSymbolTable, making
	// IsRoot() false and allowing an unintended pop.
	root := symbols.NewChildSymbolTable("root", nil)
	ctx := NewContext(root, New("test"))

	if err := ctx.popSymbolTable(); err == nil {
		t.Error("popSymbolTable on root: expected error, got nil")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Section: checkType
// ─────────────────────────────────────────────────────────────────────────────

// Test_Context_checkType_DynamicMode_NoCheck verifies that in dynamic mode any
// value is accepted for an existing symbol without type inspection.
func Test_Context_checkType_DynamicMode_NoCheck(t *testing.T) {
	tc := newTestContext(t).withTypeStrictness(defs.NoTypeEnforcement)
	tc.ctx.setAlways("x", 42) // int symbol

	got, err := tc.ctx.checkType("x", "hello") // string — different type
	if err != nil {
		t.Fatalf("checkType dynamic mode: unexpected error: %v", err)
	}

	if got != "hello" {
		t.Errorf("checkType dynamic mode: value changed, got %v, want \"hello\"", got)
	}
}

// Test_Context_checkType_NilValue_NoCheck verifies that a nil value always
// bypasses the type check regardless of type enforcement mode.
func Test_Context_checkType_NilValue_NoCheck(t *testing.T) {
	tc := newTestContext(t).withTypeStrictness(defs.StrictTypeEnforcement)
	tc.ctx.setAlways("x", 42)

	got, err := tc.ctx.checkType("x", nil)
	if err != nil {
		t.Fatalf("checkType nil value: unexpected error: %v", err)
	}

	if got != nil {
		t.Errorf("checkType nil value: got %v, want nil", got)
	}
}

// Test_Context_checkType_NoSymbol_NoCheck verifies that a non-existent symbol
// skips the type check since there is no existing type to enforce against.
func Test_Context_checkType_NoSymbol_NoCheck(t *testing.T) {
	tc := newTestContext(t).withTypeStrictness(defs.StrictTypeEnforcement)

	got, err := tc.ctx.checkType("nonexistent", 42)
	if err != nil {
		t.Fatalf("checkType non-existent symbol: unexpected error: %v", err)
	}

	if got != 42 {
		t.Errorf("checkType non-existent symbol: got %v, want 42", got)
	}
}

// Test_Context_checkType_StrictMode_MatchingTypes_OK verifies that assigning a
// value of the same type passes the strict type check.
func Test_Context_checkType_StrictMode_MatchingTypes_OK(t *testing.T) {
	tc := newTestContext(t).withTypeStrictness(defs.StrictTypeEnforcement)
	tc.ctx.setAlways("x", 42)

	got, err := tc.ctx.checkType("x", 99)
	if err != nil {
		t.Fatalf("checkType strict matching: unexpected error: %v", err)
	}

	if got != 99 {
		t.Errorf("checkType strict matching: got %v, want 99", got)
	}
}

// Test_Context_checkType_StrictMode_MismatchedTypes_Error verifies that
// assigning a value of a different type in strict mode returns an error.
func Test_Context_checkType_StrictMode_MismatchedTypes_Error(t *testing.T) {
	tc := newTestContext(t).withTypeStrictness(defs.StrictTypeEnforcement)
	tc.ctx.setAlways("x", 42) // int

	_, err := tc.ctx.checkType("x", "hello") // string ≠ int
	if err == nil {
		t.Error("checkType strict mismatch: expected error, got nil")
	}
}

// Test_Context_checkType_Immutable_UnwrappedBeforeCheck verifies that an
// Immutable-wrapped value is unwrapped before the type check is applied, and
// that the unwrapped value is returned on success.
func Test_Context_checkType_Immutable_UnwrappedBeforeCheck(t *testing.T) {
	tc := newTestContext(t).withTypeStrictness(defs.StrictTypeEnforcement)
	tc.ctx.setAlways("x", 42) // int

	got, err := tc.ctx.checkType("x", data.Immutable{Value: 99})
	if err != nil {
		t.Fatalf("checkType Immutable int → int: unexpected error: %v", err)
	}

	if got != 99 {
		t.Errorf("checkType Immutable: expected unwrapped 99, got %v", got)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Section: Symbol helpers (set / setAlways / delete / create)
// ─────────────────────────────────────────────────────────────────────────────

func Test_Context_set_UpdatesExistingSymbol(t *testing.T) {
	ctx := NewContext(symbols.NewSymbolTable("test"), New("test"))
	ctx.setAlways("n", 0)

	if err := ctx.set("n", 5); err != nil {
		t.Fatalf("set: unexpected error: %v", err)
	}

	v, found := ctx.get("n")
	if !found || v != 5 {
		t.Errorf("set: got (%v, found=%v), want (5, true)", v, found)
	}
}

func Test_Context_setAlways_CreatesOrUpdatesSymbol(t *testing.T) {
	ctx := NewContext(symbols.NewSymbolTable("test"), New("test"))
	ctx.setAlways("flag", true)

	v, found := ctx.get("flag")
	if !found || v != true {
		t.Errorf("setAlways: got (%v, found=%v), want (true, true)", v, found)
	}
}

func Test_Context_delete_RemovesSymbol(t *testing.T) {
	ctx := NewContext(symbols.NewSymbolTable("test"), New("test"))
	ctx.setAlways("tmp", 1)

	if err := ctx.delete("tmp"); err != nil {
		t.Fatalf("delete: unexpected error: %v", err)
	}

	_, found := ctx.get("tmp")
	if found {
		t.Error("delete: symbol still found after deletion")
	}
}

func Test_Context_create_CreatesUndefinedSymbol(t *testing.T) {
	ctx := NewContext(symbols.NewSymbolTable("test"), New("test"))

	if err := ctx.create("newVar"); err != nil {
		t.Fatalf("create: unexpected error: %v", err)
	}

	_, found := ctx.get("newVar")
	if !found {
		t.Error("create: symbol not visible after creation")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Section: Result
// ─────────────────────────────────────────────────────────────────────────────

func Test_Context_Result_InitiallyNil(t *testing.T) {
	ctx := NewContext(symbols.NewSymbolTable("test"), New("test"))

	if ctx.Result() != nil {
		t.Errorf("Result: expected nil for fresh context, got %v", ctx.Result())
	}
}
