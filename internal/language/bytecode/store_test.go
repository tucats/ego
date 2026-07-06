package bytecode

// store_test.go — unit tests for storeByteCode, storeChanByteCode,
// storeGlobalByteCode, storeViaPointerByteCode, storeAlwaysByteCode,
// and storeBytecodeByteCode.
//
// # Functions under test
//
// storeByteCode (Store opcode):
//   - Pops a value from the stack and writes it into a named symbol-table
//     variable.  When the operand is a []any{name, value} pair, the value
//     comes from the array instead of the stack.
//   - Storing to "_" (defs.DiscardedVariable) silently discards the value.
//   - Storing to a name that starts with "_" (readonly prefix) succeeds only
//     if the variable already exists and holds symbols.UndefinedValue; the
//     value is then wrapped in data.Constant (making it immutable).
//   - A stack marker in the value position returns ErrFunctionReturnedVoid.
//   - Type compatibility is checked via c.checkType in strict mode.
//
// storeChanByteCode (StoreChan opcode):
//   - Pops one value from the stack and either receives from or sends to a
//     channel, depending on which side is the channel.
//   - If neither side is a channel, ErrInvalidChannel is returned.
//
// storeGlobalByteCode (StoreGlobal opcode):
//   - Pops a value and writes it directly into the root symbol table.
//   - Names starting with "_" cause complex types (*Map, *Array, *Struct)
//     to be deep-copied and marked read-only.
//
// storeViaPointerByteCode (StoreViaPointer opcode):
//   - Resolves a Go pointer from a named variable (or from the stack if the
//     operand is nil) and writes a value through it.
//   - Supports *any, *bool, *byte, *int32, *int, *int64, *float64, *float32,
//     *string, *data.Array, and **data.Channel pointer types.
//   - Nil pointer → ErrNilPointerReference.
//   - Immutable target → ErrReadOnlyValue.
//
// storeAlwaysByteCode (StoreAlways opcode):
//   - Writes unconditionally, bypassing readonly protection.
//   - Blocks redefinition of existing *ByteCode functions when
//     AllowFunctionRedefinitionSetting is not enabled.
//
// receiveChannelByteCode (ReceiveChannel opcode):
//   - Pops a *data.Channel from the stack and calls Receive().
//   - On a successful receive: pushes StackMarker("receive"), true, datum.
//   - On a closed+drained channel: pushes StackMarker("receive"), false, nil.
//   - Non-channel value on stack → ErrInvalidChannel.
//   - Stack marker on stack → ErrFunctionReturnedVoid.
//
// storeBytecodeByteCode (StoreBytecode opcode):
//   - Pops a *ByteCode from the stack, assigns the operand string as its name,
//     and stores it in the symbol table via SetAlways.
//   - Non-*ByteCode value → ErrInvalidType with the value's type as context.
//   - StackMarker → ErrFunctionReturnedVoid.
//   - Empty stack → ErrStackUnderflow (already decorated by Pop).
//   - NOTE: the compiler never emits this opcode; see STORE-6.
//
// valueCopyByteCode (ValueCopy opcode):
//   - BUG-43 fix. Pops the top-of-stack value, applies
//     copyStructForValueSemantics to it (an independent copy if it is a
//     *data.Struct, unchanged otherwise), and pushes the result back — an
//     in-place transform, so the stack depth is unaffected.
//   - Used by internal/language/compiler/defer.go's hoistDeferCallArguments
//     and hoistDeferReceiver to give their StoreAlways-based temp variables
//     the same struct-value-copy semantics that Store and CreateAndStore get
//     for free, without requiring the temp variable to not already exist
//     (which CreateAndStore's c.create(name) would enforce, breaking a
//     "defer" statement re-executed inside a loop).
//
// # Known issues documented here
//
//   - STORE-3 (RESOLVED): scalar pointer helpers checked d.(string) in strict/
//     relaxed mode instead of the correct target type.
//   - STORE-4 (RESOLVED): storeChanByteCode used x (nil) as error context.
//   - STORE-5: nil operand stores ByteCode under empty string key; see
//     Test_storeBytecodeByteCode_NilOperand_STORE5.
//   - STORE-6: StoreBytecode opcode is never emitted by the compiler; see
//     Test_storeBytecodeByteCode_OpcodeIsDeadCode_STORE6.
//
// # Test organization
//
// Section 1: storeByteCode
// Section 2: storeChanByteCode
// Section 3: storeGlobalByteCode
// Section 4: storeViaPointerByteCode
// Section 5: storeAlwaysByteCode
// Section 6: storeBytecodeByteCode
// Section 7: receiveChannelByteCode
// Section 8: valueCopyByteCode
//
// All tests use the newTestContext / withXxx / assertXxx helpers from
// testhelpers_test.go as required by the project testing standards.

import (
	"strings"
	"testing"

	"github.com/tucats/ego/internal/cli/settings"
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
)

// ─── Section 1: storeByteCode ────────────────────────────────────────────────

// Test_storeByteCode_NormalStore verifies the simplest happy path: a value is
// popped from the stack and written into an existing symbol-table variable.
//
// Setup:   variable "x" pre-exists with value 0; stack has 42 on top.
// Action:  storeByteCode with operand "x".
// Expect:  x == 42 in the symbol table; stack is empty.
func Test_storeByteCode_NormalStore(t *testing.T) {
	tc := newTestContext(t).
		withSymbol("x", 0).
		withStack(42)

	err := storeByteCode(tc.ctx, "x")

	tc.assertNoError(err)
	tc.assertSymbolValue("x", 42)
	tc.assertStackEmpty()
}

// Test_storeByteCode_StoreString verifies that a string value is stored
// correctly, confirming the function is not type-restricted to integers.
func Test_storeByteCode_StoreString(t *testing.T) {
	tc := newTestContext(t).
		withSymbol("greeting", "old").
		withStack("hello, world")

	err := storeByteCode(tc.ctx, "greeting")

	tc.assertNoError(err)
	tc.assertSymbolValue("greeting", "hello, world")
	tc.assertStackEmpty()
}

// Test_storeByteCode_DiscardVariable verifies that storing to the special
// discard variable "_" (defs.DiscardedVariable) silently drops the value
// without writing to the symbol table or returning an error.
//
// This is the standard Go blank-identifier pattern: `_ = someExpr`.
func Test_storeByteCode_DiscardVariable(t *testing.T) {
	tc := newTestContext(t).
		withStack(99)

	err := storeByteCode(tc.ctx, "_")

	// No error expected — the value was simply thrown away.
	tc.assertNoError(err)
	// Nothing should remain on the stack after the discard.
	tc.assertStackEmpty()
}

// Test_storeByteCode_ArrayOperand verifies the two-element array operand form
// []any{name, value}.  In this form the value comes directly from the array
// and the stack is NOT touched at all.
//
// This form is used by the compiler when the variable name and initial value
// are both known at compile time (e.g., package-level constant initialization).
func Test_storeByteCode_ArrayOperand(t *testing.T) {
	tc := newTestContext(t).
		withSymbol("msg", "")
	// Notice: no withStack call.  The value "hello" travels via the operand.

	err := storeByteCode(tc.ctx, []any{"msg", "hello"})

	tc.assertNoError(err)
	tc.assertSymbolValue("msg", "hello")
	// Stack must be empty because we never pushed anything onto it.
	tc.assertStackEmpty()
}

// Test_storeByteCode_ReadonlyFirstWrite verifies that a variable whose name
// starts with "_" (the readonly prefix, defs.ReadonlyVariablePrefix) can be
// written exactly once — specifically when it holds symbols.UndefinedValue.
//
// After a successful first write, the value is stored as a data.Constant
// (data.Immutable wrapper), making the variable permanently read-only.
//
// This is the mechanism that makes Ego's `const`-like variables work: the
// compiler creates the variable (giving it UndefinedValue), and the first
// Store writes the initial value wrapped in Constant.
func Test_storeByteCode_ReadonlyFirstWrite(t *testing.T) {
	tc := newTestContext(t)

	// Create "_answer" without setting a value so it holds UndefinedValue.
	// We call ctx.create directly because withSymbol would also call set,
	// which would replace UndefinedValue with the supplied value.
	if err := tc.ctx.create("_answer"); err != nil {
		t.Fatalf("create _answer: %v", err)
	}

	tc.withStack(42)

	err := storeByteCode(tc.ctx, "_answer")

	tc.assertNoError(err)
	// The value must be wrapped in data.Constant (== data.Immutable{Value: 42}).
	tc.assertSymbolValue("_answer", data.Constant(42))
	tc.assertStackEmpty()
}

// Test_storeByteCode_ReadonlyAlreadySet verifies that attempting to overwrite
// a "_"-prefixed variable that already has a real value returns ErrReadOnly.
//
// This protects Ego's readonly/const semantics: once a "_foo" variable has
// been set to something other than UndefinedValue, it cannot be changed.
func Test_storeByteCode_ReadonlyAlreadySet(t *testing.T) {
	// withSymbol creates "_answer" then sets it to 42 (not UndefinedValue).
	tc := newTestContext(t).
		withSymbol("_answer", 42).
		withStack(99) // attempt to overwrite with 99

	err := storeByteCode(tc.ctx, "_answer")

	tc.assertError(err, errors.ErrReadOnly)
}

// Test_storeByteCode_ReadonlyVariableNotExist verifies that attempting to
// store into a "_"-prefixed variable that does not exist at all returns
// ErrReadOnly (not ErrUnknownIdentifier).
//
// A readonly variable must be created by the compiler before it can be set —
// if it doesn't exist yet, the Store instruction rejects the write.
func Test_storeByteCode_ReadonlyVariableNotExist(t *testing.T) {
	tc := newTestContext(t).
		withStack(99)

	// "_nosuchvar" was never created; storeByteCode should reject it.
	err := storeByteCode(tc.ctx, "_nosuchvar")

	tc.assertError(err, errors.ErrReadOnly)
}

// Test_storeByteCode_StackMarker verifies that a StackMarker value on the
// stack (which indicates a function returned no value) causes storeByteCode
// to return ErrFunctionReturnedVoid rather than storing a nonsensical sentinel.
func Test_storeByteCode_StackMarker(t *testing.T) {
	tc := newTestContext(t).
		withSymbol("x", 0).
		withStack(NewStackMarker("results")) // push a marker instead of a real value

	err := storeByteCode(tc.ctx, "x")

	tc.assertError(err, errors.ErrFunctionReturnedVoid)
}

// Test_storeByteCode_EmptyStack verifies that an underflow error is returned
// when the stack is empty and a value is needed (plain-string operand form).
func Test_storeByteCode_EmptyStack(t *testing.T) {
	tc := newTestContext(t).
		withSymbol("x", 0)
	// No withStack call — stack is empty.

	err := storeByteCode(tc.ctx, "x")

	// The Pop call inside storeByteCode must return an error.
	if err == nil {
		t.Errorf("expected an error for empty stack, got nil")
	}
}

// Test_storeByteCode_StrictTypeCheckFails verifies that in strict type mode,
// storing a value of the wrong type into an existing typed variable returns an
// error from the type-compatibility check.
//
// Here "count" holds an int, and we try to store a string into it.
func Test_storeByteCode_StrictTypeCheckFails(t *testing.T) {
	tc := newTestContext(t).
		withSymbol("count", 0). // count is typed as int
		withStack("oops").      // wrong type: string
		withTypeStrictness(defs.StrictTypeEnforcement)

	err := storeByteCode(tc.ctx, "count")

	// A type error should be returned; the exact error key depends on the
	// type checker, but it must be non-nil.
	if err == nil {
		t.Errorf("expected a type-check error in strict mode, got nil")
	}
}

// ─── Section 2: storeChanByteCode ────────────────────────────────────────────

// Test_storeChanByteCode_ReceiveFromChannel verifies that when the stack holds
// a *data.Channel, storeChanByteCode receives one value from the channel and
// stores it in the named variable.
//
// This is the "<- ch" (receive) half of the channel operation.
func Test_storeChanByteCode_ReceiveFromChannel(t *testing.T) {
	// Build a buffered channel and pre-load it with the value 42.
	ch := data.NewChannel(1)
	if err := ch.Send(42); err != nil {
		t.Fatalf("channel Send: %v", err)
	}

	// Create the destination variable, then push the channel onto the stack.
	tc := newTestContext(t)
	if err := tc.ctx.create("result"); err != nil {
		t.Fatalf("create result: %v", err)
	}

	tc.withStack(ch)

	err := storeChanByteCode(tc.ctx, "result")

	tc.assertNoError(err)
	// result should now hold the value received from the channel.
	tc.assertSymbolValue("result", 42)
	tc.assertStackEmpty()
}

// Test_storeChanByteCode_SendToChannel verifies that when the named variable
// holds a *data.Channel, storeChanByteCode sends the stack value to that
// channel.
//
// This is the "ch <- value" (send) half of the channel operation.
func Test_storeChanByteCode_SendToChannel(t *testing.T) {
	ch := data.NewChannel(1) // buffered so Send doesn't block

	// Store the channel in the symbol table, push the value to send.
	tc := newTestContext(t).
		withSymbol("myChan", ch).
		withStack(77)

	err := storeChanByteCode(tc.ctx, "myChan")

	tc.assertNoError(err)
	tc.assertStackEmpty()

	// Verify the channel actually received the value.
	got, recvErr := ch.Receive()
	if recvErr != nil {
		t.Fatalf("channel Receive after send: %v", recvErr)
	}

	if got != 77 {
		t.Errorf("channel received %v (%T), want 77", got, got)
	}
}

// Test_storeChanByteCode_ReceiveIntoNewVar verifies that when the source is a
// channel and the destination variable does not yet exist, storeChanByteCode
// creates the variable automatically and stores the received value.
//
// This lets Ego code do `x <- ch` without having declared x first.
func Test_storeChanByteCode_ReceiveIntoNewVar(t *testing.T) {
	ch := data.NewChannel(1)
	if err := ch.Send("hello"); err != nil {
		t.Fatalf("channel Send: %v", err)
	}

	tc := newTestContext(t)
	tc.withStack(ch)
	// "newVar" is NOT pre-created in the symbol table.

	err := storeChanByteCode(tc.ctx, "newVar")

	tc.assertNoError(err)
	tc.assertSymbolValue("newVar", "hello")
}

// Test_storeChanByteCode_ReceiveToDiscard verifies that receiving from a
// channel into the discard variable "_" works without error and simply drops
// the received value.
func Test_storeChanByteCode_ReceiveToDiscard(t *testing.T) {
	ch := data.NewChannel(1)
	if err := ch.Send(123); err != nil {
		t.Fatalf("channel Send: %v", err)
	}

	tc := newTestContext(t)
	tc.withStack(ch)

	err := storeChanByteCode(tc.ctx, "_")

	tc.assertNoError(err)
	tc.assertStackEmpty()
}

// Test_storeChanByteCode_NeitherIsChannel verifies that when neither the stack
// value nor the named variable is a *data.Channel, the instruction returns
// ErrInvalidChannel.
//
// Both sides must be checked: if you push a plain integer and name a variable
// that holds a plain integer, there is nothing channel-like about the operation.
func Test_storeChanByteCode_NeitherIsChannel(t *testing.T) {
	tc := newTestContext(t).
		withSymbol("notAChan", 10).
		withStack(99) // also not a channel

	err := storeChanByteCode(tc.ctx, "notAChan")

	tc.assertError(err, errors.ErrInvalidChannel)
}

// Test_storeChanByteCode_NonChanDestVarNotFound verifies that when the stack
// value is NOT a channel and the named destination variable does not exist,
// ErrUnknownIdentifier is returned and the error message includes the variable
// name (not "<nil>").
//
// The STORE-4 fix changed .Context(x) to .Context(variableName) so the
// diagnostic now reads "unknown identifier: missing" instead of
// "unknown identifier: <nil>".
func Test_storeChanByteCode_NonChanDestVarNotFound(t *testing.T) {
	tc := newTestContext(t).
		withStack(99) // plain integer — not a channel

	// "missing" does not exist in the symbol table.
	err := storeChanByteCode(tc.ctx, "missing")

	tc.assertError(err, errors.ErrUnknownIdentifier)

	// After the STORE-4 fix the error context must be the variable name, not nil.
	if err != nil && !strings.Contains(err.Error(), "missing") {
		t.Errorf("error message %q should contain the variable name %q", err.Error(), "missing")
	}
}

// Test_storeChanByteCode_StackMarker verifies that a StackMarker on the stack
// (representing a void function return) returns ErrFunctionReturnedVoid.
func Test_storeChanByteCode_StackMarker(t *testing.T) {
	tc := newTestContext(t).
		withStack(NewStackMarker("results"))

	err := storeChanByteCode(tc.ctx, "x")

	tc.assertError(err, errors.ErrFunctionReturnedVoid)
}

// ─── Section 3: storeGlobalByteCode ──────────────────────────────────────────

// Test_storeGlobalByteCode_NormalStore verifies that a value is written into
// the root (global) symbol table, not just the local scope.
//
// The assertion reads the value back from the root table directly to confirm
// it landed at the global level.
func Test_storeGlobalByteCode_NormalStore(t *testing.T) {
	tc := newTestContext(t).
		withStack("global_value")

	err := storeGlobalByteCode(tc.ctx, "myGlobal")

	tc.assertNoError(err)
	tc.assertStackEmpty()

	// Read directly from the root table to verify it is truly global.
	v, found := tc.ctx.symbols.Root().Get("myGlobal")
	if !found {
		t.Errorf("myGlobal not found in root symbol table")
	} else if v != "global_value" {
		t.Errorf("myGlobal = %v, want %q", v, "global_value")
	}
}

// Test_storeGlobalByteCode_ReadonlyPrefixMap verifies that when a *data.Map is
// stored under a "_"-prefixed name, it is deep-copied and marked read-only.
//
// The original map is NOT modified; only the copy in the global table is
// marked read-only.  This protects global "constant" maps from mutation.
func Test_storeGlobalByteCode_ReadonlyPrefixMap(t *testing.T) {
	m := data.NewMap(data.StringType, data.IntType)
	_, _ = m.Set("key", 1)

	tc := newTestContext(t).
		withStack(m)

	err := storeGlobalByteCode(tc.ctx, "_myMap")

	tc.assertNoError(err)

	// Retrieve the stored copy from the root table.
	v, found := tc.ctx.symbols.Root().Get("_myMap")
	if !found {
		t.Fatalf("_myMap not found in root symbol table")
	}

	stored, ok := v.(*data.Map)
	if !ok {
		t.Fatalf("_myMap is %T, want *data.Map", v)
	}
	// The stored copy must be read-only.
	if !stored.IsReadonly() {
		t.Errorf("_myMap in global table was not marked read-only")
	}
}

// Test_storeGlobalByteCode_ReadonlyPrefixArray verifies that a *data.Array
// stored under a "_"-prefixed global name is marked read-only.
func Test_storeGlobalByteCode_ReadonlyPrefixArray(t *testing.T) {
	arr := data.NewArray(data.IntType, 3)

	tc := newTestContext(t).
		withStack(arr)

	err := storeGlobalByteCode(tc.ctx, "_myArr")

	tc.assertNoError(err)

	v, found := tc.ctx.symbols.Root().Get("_myArr")
	if !found {
		t.Fatalf("_myArr not found in root symbol table")
	}

	stored, ok := v.(*data.Array)
	if !ok {
		t.Fatalf("_myArr is %T, want *data.Array", v)
	}

	if !stored.IsReadonly() {
		t.Errorf("_myArr in global table was not marked read-only")
	}
}

// Test_storeGlobalByteCode_StackMarker verifies that a StackMarker on the
// stack returns ErrFunctionReturnedVoid without writing anything.
func Test_storeGlobalByteCode_StackMarker(t *testing.T) {
	tc := newTestContext(t).
		withStack(NewStackMarker("results"))

	err := storeGlobalByteCode(tc.ctx, "g")

	tc.assertError(err, errors.ErrFunctionReturnedVoid)
}

// Test_storeGlobalByteCode_PlainNameNotMarkedReadonly verifies that a variable
// whose name does NOT start with "_" is stored in the root table as-is,
// without any read-only wrapping, even if it is a *data.Map.
func Test_storeGlobalByteCode_PlainNameNotMarkedReadonly(t *testing.T) {
	m := data.NewMap(data.StringType, data.IntType)
	_, _ = m.Set("key", 99)

	tc := newTestContext(t).
		withStack(m)

	err := storeGlobalByteCode(tc.ctx, "writableMap")

	tc.assertNoError(err)

	v, found := tc.ctx.symbols.Root().Get("writableMap")
	if !found {
		t.Fatalf("writableMap not found in root symbol table")
	}

	stored, ok := v.(*data.Map)
	if !ok {
		t.Fatalf("writableMap is %T, want *data.Map", v)
	}

	if stored.IsReadonly() {
		t.Errorf("writableMap should NOT be read-only, but it is")
	}
}

// ─── Section 4: storeViaPointerByteCode ──────────────────────────────────────

// Test_storeViaPointerByteCode_AnyPointer verifies the core *any pointer path:
// a value is popped from the stack and written through a *any pointer that
// was previously stored in the symbol table.
//
// This is the most common case in Ego: `*p = newValue` where p holds a *any.
func Test_storeViaPointerByteCode_AnyPointer(t *testing.T) {
	// Set up a *any pointer that initially holds "original".
	var target any = "original"
	ptr := &target

	tc := newTestContext(t).
		withSymbol("p", ptr).
		withStack("updated")

	err := storeViaPointerByteCode(tc.ctx, "p")

	tc.assertNoError(err)
	tc.assertStackEmpty()

	// The underlying Go variable should now hold "updated".
	if target != "updated" {
		t.Errorf("*p = %v, want %q", target, "updated")
	}
}

// Test_storeViaPointerByteCode_NilOperandPopsPointerFromStack verifies that
// when the operand is nil, storeViaPointerByteCode pops the pointer from the
// stack rather than looking it up by name.
//
// Stack layout expected: bottom=[pointer], top=[value to store].
// After the call: the value has been written through the pointer.
func Test_storeViaPointerByteCode_NilOperandPopsPointerFromStack(t *testing.T) {
	var target any = "before"
	ptr := &target

	// Push value first (bottom), then pointer on top (popped last, i.e. first).
	// Wait — storeViaPointerByteCode pops the VALUE after resolving dest.
	// With nil operand: dest is popped first (it's on top), then value is popped.
	//
	// Reading the code: when i==nil, dest is c.Pop() (pops top of stack).
	// Then value is c.PopWithoutUnwrapping() (pops the next item).
	// So we push: value first (goes to bottom), then pointer (goes to top).
	tc := newTestContext(t).
		withStack("newvalue", ptr) // "newvalue" is below ptr on stack

	err := storeViaPointerByteCode(tc.ctx, nil)

	tc.assertNoError(err)
	tc.assertStackEmpty()

	if target != "newvalue" {
		t.Errorf("*ptr = %v, want %q", target, "newvalue")
	}
}

// Test_storeViaPointerByteCode_InvalidIdentifier verifies that a name starting
// with "_" (e.g. the discard variable or a readonly variable) is rejected with
// ErrInvalidIdentifier.
//
// Readonly variables cannot be modified through pointer indirection — the
// instruction prevents this by rejecting any "_"-prefixed name up front.
func Test_storeViaPointerByteCode_InvalidIdentifier(t *testing.T) {
	tc := newTestContext(t).
		withStack("value")

	err := storeViaPointerByteCode(tc.ctx, "_")

	tc.assertError(err, errors.ErrInvalidIdentifier)
}

// Test_storeViaPointerByteCode_UnknownVariable verifies that referencing a
// variable that does not exist in any symbol table scope returns
// ErrUnknownIdentifier.
func Test_storeViaPointerByteCode_UnknownVariable(t *testing.T) {
	tc := newTestContext(t).
		withStack("value")

	err := storeViaPointerByteCode(tc.ctx, "nosuchvar")

	tc.assertError(err, errors.ErrUnknownIdentifier)
}

// Test_storeViaPointerByteCode_NilPointer verifies that when the named
// variable holds a nil value (a nil pointer) the instruction returns
// ErrNilPointerReference rather than panicking.
func Test_storeViaPointerByteCode_NilPointer(t *testing.T) {
	// Store nil for "p" — this simulates a nil *any pointer.
	tc := newTestContext(t).
		withSymbol("p", nil).
		withStack("value")

	err := storeViaPointerByteCode(tc.ctx, "p")

	tc.assertError(err, errors.ErrNilPointerReference)
}

// Test_storeViaPointerByteCode_ImmutableTarget verifies that writing through
// a *any that points to a data.Immutable value returns ErrReadOnlyValue.
//
// This prevents `*p = newValue` from silently bypassing a constant's
// immutability: the instruction detects the Immutable wrapper and refuses.
func Test_storeViaPointerByteCode_ImmutableTarget(t *testing.T) {
	// The *any points to a data.Immutable (a readonly constant).
	var target any = data.Constant(42) // data.Immutable{Value: 42}
	ptr := &target

	tc := newTestContext(t).
		withSymbol("p", ptr).
		withStack(99) // attempt to overwrite a constant

	err := storeViaPointerByteCode(tc.ctx, "p")

	tc.assertError(err, errors.ErrReadOnlyValue)
}

// Test_storeViaPointerByteCode_DirectImmutablePointer verifies the case where
// the variable itself holds a *data.Immutable (not a *any wrapping an
// Immutable) and returns ErrReadOnlyValue via the switch's explicit case.
func Test_storeViaPointerByteCode_DirectImmutablePointer(t *testing.T) {
	imm := data.Constant(7)
	immPtr := &imm

	tc := newTestContext(t).
		withSymbol("c", immPtr).
		withStack(99)

	err := storeViaPointerByteCode(tc.ctx, "c")

	tc.assertError(err, errors.ErrReadOnlyValue)
}

// Test_storeViaPointerByteCode_StackMarker verifies that a StackMarker in the
// value position (popped after the pointer is resolved) returns
// ErrFunctionReturnedVoid rather than writing a nonsensical sentinel.
func Test_storeViaPointerByteCode_StackMarker(t *testing.T) {
	var target any = "before"
	ptr := &target

	tc := newTestContext(t).
		withSymbol("p", ptr).
		withStack(NewStackMarker("results")) // marker instead of a value

	err := storeViaPointerByteCode(tc.ctx, "p")

	tc.assertError(err, errors.ErrFunctionReturnedVoid)
}

// Test_storeViaPointerByteCode_BoolPointer_NoEnforcement verifies that a bool
// value is correctly stored through a *bool pointer in NoTypeEnforcement mode,
// where the runtime coerces any compatible value to bool.
func Test_storeViaPointerByteCode_BoolPointer_NoEnforcement(t *testing.T) {
	b := false
	bPtr := &b

	tc := newTestContext(t).
		withSymbol("flag", bPtr).
		withStack(true).
		withTypeStrictness(defs.NoTypeEnforcement)

	err := storeViaPointerByteCode(tc.ctx, "flag")

	tc.assertNoError(err)

	if !b {
		t.Errorf("*flag = false, want true")
	}
}

// Test_storeViaPointerByteCode_StringPointer_NoEnforcement verifies that
// a string value is stored through a *string pointer correctly.
func Test_storeViaPointerByteCode_StringPointer_NoEnforcement(t *testing.T) {
	s := "old"
	sPtr := &s

	tc := newTestContext(t).
		withSymbol("sp", sPtr).
		withStack("new").
		withTypeStrictness(defs.NoTypeEnforcement)

	err := storeViaPointerByteCode(tc.ctx, "sp")

	tc.assertNoError(err)

	if s != "new" {
		t.Errorf("*sp = %q, want %q", s, "new")
	}
}

// Test_storeViaPointerByteCode_Int64Pointer_NoEnforcement verifies that an
// int64 value is stored through a *int64 pointer correctly in no-enforcement
// mode (uses the coerce path).
func Test_storeViaPointerByteCode_Int64Pointer_NoEnforcement(t *testing.T) {
	var n int64 = 0
	nPtr := &n

	tc := newTestContext(t).
		withSymbol("n64", nPtr).
		withStack(int64(99)).
		withTypeStrictness(defs.NoTypeEnforcement)

	err := storeViaPointerByteCode(tc.ctx, "n64")

	tc.assertNoError(err)

	if n != 99 {
		t.Errorf("*n64 = %d, want 99", n)
	}
}

// Test_storeViaPointerByteCode_Float32Pointer_NoEnforcement verifies that a
// float32 value stored through *float32 works correctly in no-enforcement mode.
func Test_storeViaPointerByteCode_Float32Pointer_NoEnforcement(t *testing.T) {
	var f float32 = 0
	fPtr := &f

	tc := newTestContext(t).
		withSymbol("fp", fPtr).
		withStack(float32(3.14)).
		withTypeStrictness(defs.NoTypeEnforcement)

	err := storeViaPointerByteCode(tc.ctx, "fp")

	tc.assertNoError(err)

	if f != float32(3.14) {
		t.Errorf("*fp = %v, want 3.14", f)
	}
}

// Test_storeViaPointerByteCode_Float32Pointer_StrictMode verifies that storing
// a correctly-typed float32 value through a *float32 pointer succeeds in strict
// type-enforcement mode.
//
// This test was originally written to document the STORE-3 bug (where the
// strict/relaxed branch incorrectly checked d.(string) instead of d.(float32),
// causing ErrInvalidVarType even for a matching type).  After the STORE-3 fix
// the check uses d.(float32), so the correctly-typed value is accepted.
func Test_storeViaPointerByteCode_Float32Pointer_StrictMode(t *testing.T) {
	var f float32 = 0
	fPtr := &f

	tc := newTestContext(t).
		withSymbol("fp", fPtr).
		withStack(float32(1.5)).
		withTypeStrictness(defs.StrictTypeEnforcement)

	err := storeViaPointerByteCode(tc.ctx, "fp")

	tc.assertNoError(err)

	if f != float32(1.5) {
		t.Errorf("*fp = %v, want float32(1.5)", f)
	}
}

// Test_storeViaPointerByteCode_Float32Pointer_RelaxedMode verifies that a
// correctly-typed float32 value also succeeds in relaxed type enforcement.
func Test_storeViaPointerByteCode_Float32Pointer_RelaxedMode(t *testing.T) {
	var f float32 = 0
	fPtr := &f

	tc := newTestContext(t).
		withSymbol("fp", fPtr).
		withStack(float32(2.5)).
		withTypeStrictness(defs.RelaxedTypeEnforcement)

	err := storeViaPointerByteCode(tc.ctx, "fp")

	tc.assertNoError(err)

	if f != float32(2.5) {
		t.Errorf("*fp = %v, want float32(2.5)", f)
	}
}

// Test_storeViaPointerByteCode_Float32Pointer_StrictMode_WrongType verifies
// that storing a value of the wrong type (float64) through *float32 in strict
// mode returns ErrInvalidVarType.
//
// In strict mode the value must already be exactly float32; float64 is a
// distinct type even though it is numerically compatible.
func Test_storeViaPointerByteCode_Float32Pointer_StrictMode_WrongType(t *testing.T) {
	var f float32 = 0
	fPtr := &f

	tc := newTestContext(t).
		withSymbol("fp", fPtr).
		withStack(float64(1.5)). // wrong type: float64, not float32
		withTypeStrictness(defs.StrictTypeEnforcement)

	err := storeViaPointerByteCode(tc.ctx, "fp")

	tc.assertError(err, errors.ErrInvalidVarType)
}

// Test_storeViaPointerByteCode_BoolPointer_StrictMode verifies that a bool
// value is accepted through *bool in strict mode (STORE-3 fix coverage).
func Test_storeViaPointerByteCode_BoolPointer_StrictMode(t *testing.T) {
	b := false
	bPtr := &b

	tc := newTestContext(t).
		withSymbol("flag", bPtr).
		withStack(true).
		withTypeStrictness(defs.StrictTypeEnforcement)

	err := storeViaPointerByteCode(tc.ctx, "flag")

	tc.assertNoError(err)

	if !b {
		t.Errorf("*flag = false, want true")
	}
}

// Test_storeViaPointerByteCode_IntPointer_StrictMode verifies that an int
// value is accepted through *int in strict mode (STORE-3 fix coverage).
func Test_storeViaPointerByteCode_IntPointer_StrictMode(t *testing.T) {
	n := 0
	nPtr := &n

	tc := newTestContext(t).
		withSymbol("n", nPtr).
		withStack(42).
		withTypeStrictness(defs.StrictTypeEnforcement)

	err := storeViaPointerByteCode(tc.ctx, "n")

	tc.assertNoError(err)
	
	if n != 42 {
		t.Errorf("*n = %d, want 42", n)
	}
}

// Test_storeViaPointerByteCode_UnsupportedPointerType verifies that an
// unknown (unsupported) pointer type stored in the variable slot returns
// ErrNotAPointer rather than panicking.
func Test_storeViaPointerByteCode_UnsupportedPointerType(t *testing.T) {
	// *complex128 is not in the switch statement.
	var c complex128 = 0
	cPtr := &c

	tc := newTestContext(t).
		withSymbol("cp", cPtr).
		withStack(complex(1, 2))

	err := storeViaPointerByteCode(tc.ctx, "cp")

	tc.assertError(err, errors.ErrNotAPointer)
}

// ─── Section 5: storeAlwaysByteCode ──────────────────────────────────────────

// Test_storeAlwaysByteCode_NormalStore verifies that storeAlwaysByteCode
// writes a value into the symbol table, creating the variable if it does not
// exist (unlike storeByteCode, which requires the variable to already exist).
func Test_storeAlwaysByteCode_NormalStore(t *testing.T) {
	tc := newTestContext(t).
		withStack("hello")

	// "fresh" does not exist yet; storeAlways must create it.
	err := storeAlwaysByteCode(tc.ctx, "fresh")

	tc.assertNoError(err)
	tc.assertSymbolValue("fresh", "hello")
	tc.assertStackEmpty()
}

// Test_storeAlwaysByteCode_OverwritesExisting verifies that storeAlwaysByteCode
// can overwrite a variable that already holds a value, even if the original
// value would be protected under normal Store semantics.
func Test_storeAlwaysByteCode_OverwritesExisting(t *testing.T) {
	tc := newTestContext(t).
		withSymbol("x", 10).
		withStack(99)

	err := storeAlwaysByteCode(tc.ctx, "x")

	tc.assertNoError(err)
	tc.assertSymbolValue("x", 99)
}

// Test_storeAlwaysByteCode_ArrayOperand verifies the two-element array operand
// form []any{name, value}: the value comes from the array, not the stack.
func Test_storeAlwaysByteCode_ArrayOperand(t *testing.T) {
	tc := newTestContext(t)
	// No stack push — the value travels via the operand array.

	err := storeAlwaysByteCode(tc.ctx, []any{"pi", 3.14159})

	tc.assertNoError(err)
	tc.assertSymbolValue("pi", 3.14159)
	tc.assertStackEmpty()
}

// Test_storeAlwaysByteCode_ReadonlyPrefixMap verifies that storing a *data.Map
// under a "_"-prefixed name via storeAlwaysByteCode marks the map read-only.
//
// This is how the runtime initializes global read-only (const-like) map
// collections in Ego programs.
func Test_storeAlwaysByteCode_ReadonlyPrefixMap(t *testing.T) {
	m := data.NewMap(data.StringType, data.StringType)
	_, _ = m.Set("color", "blue")

	tc := newTestContext(t).
		withStack(m)

	err := storeAlwaysByteCode(tc.ctx, "_colorMap")

	tc.assertNoError(err)

	v, found := tc.ctx.symbols.Get("_colorMap")
	if !found {
		t.Fatalf("_colorMap not found in symbol table")
	}

	stored, ok := v.(*data.Map)
	if !ok {
		t.Fatalf("_colorMap is %T, want *data.Map", v)
	}

	if !stored.IsReadonly() {
		t.Errorf("_colorMap was not marked read-only")
	}
}

// Test_storeAlwaysByteCode_ReadonlyPrefixArray verifies that a *data.Array
// stored under a "_"-prefixed name is marked read-only.
func Test_storeAlwaysByteCode_ReadonlyPrefixArray(t *testing.T) {
	arr := data.NewArray(data.StringType, 2)
	_ = arr.Set(0, "alpha")
	_ = arr.Set(1, "beta")

	tc := newTestContext(t).
		withStack(arr)

	err := storeAlwaysByteCode(tc.ctx, "_names")

	tc.assertNoError(err)

	v, found := tc.ctx.symbols.Get("_names")
	if !found {
		t.Fatalf("_names not found in symbol table")
	}

	stored, ok := v.(*data.Array)
	if !ok {
		t.Fatalf("_names is %T, want *data.Array", v)
	}

	if !stored.IsReadonly() {
		t.Errorf("_names was not marked read-only")
	}
}

// Test_storeAlwaysByteCode_ReadonlyPrefixStruct verifies that a *data.Struct
// stored under a "_"-prefixed name is marked read-only.
func Test_storeAlwaysByteCode_ReadonlyPrefixStruct(t *testing.T) {
	st := data.NewStructFromMap(map[string]any{"x": 1, "y": 2})

	tc := newTestContext(t).
		withStack(st)

	err := storeAlwaysByteCode(tc.ctx, "_point")

	tc.assertNoError(err)

	v, found := tc.ctx.symbols.Get("_point")
	if !found {
		t.Fatalf("_point not found in symbol table")
	}

	stored, ok := v.(*data.Struct)
	if !ok {
		t.Fatalf("_point is %T, want *data.Struct", v)
	}

	if !stored.IsReadonly() {
		t.Errorf("_point was not marked read-only")
	}
}

// Test_storeAlwaysByteCode_StackMarker verifies that a StackMarker on the
// stack returns ErrFunctionReturnedVoid.
func Test_storeAlwaysByteCode_StackMarker(t *testing.T) {
	tc := newTestContext(t).
		withStack(NewStackMarker("results"))

	err := storeAlwaysByteCode(tc.ctx, "x")

	tc.assertError(err, errors.ErrFunctionReturnedVoid)
}

// Test_storeAlwaysByteCode_FuncRedefinitionBlocked verifies that attempting to
// overwrite an existing *ByteCode function definition is rejected with
// ErrFunctionAlreadyExists when AllowFunctionRedefinitionSetting is disabled.
//
// This guards against accidental function redefinition in non-interactive mode:
// if a compiled program contains two top-level functions with the same name,
// the second definition would silently replace the first without this check.
func Test_storeAlwaysByteCode_FuncRedefinitionBlocked(t *testing.T) {
	// Ensure the interactive/redefinition setting is OFF for this test.
	saved := settings.Get(defs.AllowFunctionRedefinitionSetting)
	settings.Set(defs.AllowFunctionRedefinitionSetting, "false")
	defer settings.Set(defs.AllowFunctionRedefinitionSetting, saved)

	existingFn := &ByteCode{name: "myFunc"}
	newFn := &ByteCode{name: "myFunc"}

	// Store the first function via setAlways (bypasses the check that
	// storeAlwaysByteCode performs, so we can seed the symbol table).
	tc := newTestContext(t)
	tc.ctx.setAlways("myFunc", existingFn)

	// Now push the second definition and try to overwrite via storeAlways.
	tc.withStack(newFn)

	err := storeAlwaysByteCode(tc.ctx, "myFunc")

	tc.assertError(err, errors.ErrFunctionAlreadyExists)
}

// Test_storeAlwaysByteCode_FuncRedefinitionAllowed verifies that when
// AllowFunctionRedefinitionSetting is enabled (interactive / REPL mode),
// overwriting an existing *ByteCode function succeeds without error.
//
// The REPL allows re-typing the same function during a session.
func Test_storeAlwaysByteCode_FuncRedefinitionAllowed(t *testing.T) {
	// Enable the interactive/redefinition setting for this test.
	saved := settings.Get(defs.AllowFunctionRedefinitionSetting)
	settings.Set(defs.AllowFunctionRedefinitionSetting, "true")
	defer settings.Set(defs.AllowFunctionRedefinitionSetting, saved)

	existingFn := &ByteCode{name: "myFunc"}
	newFn := &ByteCode{name: "myFunc"}

	tc := newTestContext(t)
	tc.ctx.setAlways("myFunc", existingFn)
	tc.withStack(newFn)

	err := storeAlwaysByteCode(tc.ctx, "myFunc")

	tc.assertNoError(err)
}

// ─── Section 7: receiveChannelByteCode ───────────────────────────────────────

// Test_receiveChannelByteCode_SuccessfulReceive verifies the happy path: a
// value is available in the channel, so the opcode pushes [marker, true, datum].
//
// Setup:   a buffered channel with one item (42) already sent into it.
// Action:  receiveChannelByteCode with nil operand (operand is unused).
// Expect:  three items on the stack (bottom → top): StackMarker("receive"),
//
//	true, 42.  No error.
func Test_receiveChannelByteCode_SuccessfulReceive(t *testing.T) {
	ch := data.NewChannel(1)
	_ = ch.Send(42)

	tc := newTestContext(t).withStack(ch)

	err := receiveChannelByteCode(tc.ctx, nil)

	tc.assertNoError(err)

	// Datum (42) is on top.
	tc.assertTopStack(42)

	// ok (true) is next.
	tc.assertTopStack(true)

	// A StackMarker("receive") is at the bottom of the pushed group.
	// Pop it and verify it is a stack marker with the right label.
	raw, popErr := tc.ctx.Pop()
	if popErr != nil {
		t.Fatalf("unexpected pop error: %v", popErr)
	}

	if !isStackMarker(raw, "receive") {
		t.Errorf("expected StackMarker(\"receive\"), got %T %v", raw, raw)
	}
}

// Test_receiveChannelByteCode_ClosedChannel verifies behavior when the channel
// is closed and already drained.  This is the normal "loop finished" condition
// in for-range over a channel, not an error.
//
// Setup:   a channel that has been closed without sending any items.
// Action:  receiveChannelByteCode.
// Expect:  stack has [StackMarker("receive"), false, nil]; no error returned.
func Test_receiveChannelByteCode_ClosedChannel(t *testing.T) {
	ch := data.NewChannel(1)
	ch.Close()

	tc := newTestContext(t).withStack(ch)

	err := receiveChannelByteCode(tc.ctx, nil)

	tc.assertNoError(err)

	// Datum is nil (zero value for a closed channel).
	tc.assertTopStack(nil)

	// ok is false — the channel was closed.
	tc.assertTopStack(false)

	// Consume the marker.
	raw, popErr := tc.ctx.Pop()
	if popErr != nil {
		t.Fatalf("unexpected pop error: %v", popErr)
	}

	if !isStackMarker(raw, "receive") {
		t.Errorf("expected StackMarker(\"receive\"), got %T %v", raw, raw)
	}
}

// Test_receiveChannelByteCode_NonChannelValue verifies that passing a non-channel
// value (e.g. an integer) returns ErrInvalidChannel.
//
// Setup:   integer 99 on the stack instead of a channel.
// Action:  receiveChannelByteCode.
// Expect:  ErrInvalidChannel; stack is empty (the integer was popped first).
func Test_receiveChannelByteCode_NonChannelValue(t *testing.T) {
	tc := newTestContext(t).withStack(99)

	err := receiveChannelByteCode(tc.ctx, nil)

	tc.assertError(err, errors.ErrInvalidChannel)
}

// Test_receiveChannelByteCode_StackMarker verifies that a stack marker on top
// of the stack (which would indicate a void return) produces ErrFunctionReturnedVoid.
//
// Setup:   a StackMarker on the stack instead of a channel.
// Action:  receiveChannelByteCode.
// Expect:  ErrFunctionReturnedVoid.
func Test_receiveChannelByteCode_StackMarker(t *testing.T) {
	tc := newTestContext(t).withStack(NewStackMarker("void"))

	err := receiveChannelByteCode(tc.ctx, nil)

	tc.assertError(err, errors.ErrFunctionReturnedVoid)
}

// Test_receiveChannelByteCode_StackLayout verifies the exact push order
// so that the surrounding storeLValue machinery (StackCheck 2 → Store v →
// Store ok → DropToMarker) can consume the values correctly.
//
// The ReceiveChannel opcode must produce (bottom to top):
//
//	StackMarker("receive"), ok (bool), datum (any)
//
// This test sends two items through a channel and receives the first one,
// verifying the complete three-item group is in the right order.
func Test_receiveChannelByteCode_StackLayout(t *testing.T) {
	ch := data.NewChannel(2)
	_ = ch.Send("hello")
	_ = ch.Send("world")

	tc := newTestContext(t).withStack(ch)

	err := receiveChannelByteCode(tc.ctx, nil)

	tc.assertNoError(err)

	// Stack pointer should now be at 3 (marker + ok + datum).
	if tc.ctx.stackPointer != 3 {
		t.Errorf("expected stackPointer=3, got %d", tc.ctx.stackPointer)
	}

	// datum is at stackPointer (index 2, top): "hello"
	tc.assertTopStack("hello")

	// ok is at index 1: true
	tc.assertTopStack(true)

	// marker is at index 0: StackMarker("receive")
	raw, _ := tc.ctx.Pop()
	if !isStackMarker(raw, "receive") {
		t.Errorf("expected StackMarker(\"receive\"), got %T %v", raw, raw)
	}
}

// ─── Section 8: valueCopyByteCode ────────────────────────────────────────────
//
// valueCopyByteCode is the BUG-43 fix: internal/language/compiler/defer.go's
// hoistDeferCallArguments and hoistDeferReceiver emit this opcode immediately
// before StoreAlways when freezing a deferred call's arguments/receiver into
// a synthetic temp variable, so that a struct value gets the same
// independent-copy semantics as an ordinary "tempName := value" short
// variable declaration (which compiles to CreateAndStore, and which already
// gets this for free via copyStructForValueSemantics — see BUG-26).

// Test_valueCopyByteCode_StructIsIndependentCopy verifies the core fix: for a
// *data.Struct, the value pushed back is a different object than the one
// popped, and mutating the field on the ORIGINAL after the copy does not
// affect the copy sitting on the stack — exactly the guarantee BUG-43 needed
// for a deferred receiver like "l" in "defer l.Log(msg)".
func Test_valueCopyByteCode_StructIsIndependentCopy(t *testing.T) {
	original := data.NewStructFromMap(map[string]any{"prefix": "A"})

	tc := newTestContext(t).withStack(original)

	err := valueCopyByteCode(tc.ctx, nil)
	tc.assertNoError(err)

	copied, popErr := tc.ctx.Pop()
	if popErr != nil {
		t.Fatalf("Pop() after valueCopyByteCode: %v", popErr)
	}

	copiedStruct, ok := copied.(*data.Struct)
	if !ok {
		t.Fatalf("expected *data.Struct on stack, got %T", copied)
	}

	if copiedStruct == original {
		t.Fatal("valueCopyByteCode did not copy the struct — same pointer returned")
	}

	// Mutate the ORIGINAL after the copy; the copy must be unaffected. This is
	// exactly BUG-43's reproducer: "l.prefix = \"B\"" after "defer l.Log(...)"
	// must not change what the deferred call sees.
	original.SetAlways("prefix", "B")

	if v, _ := copiedStruct.Get("prefix"); v != "A" {
		t.Errorf("copy was affected by mutating the original: prefix = %v, want \"A\"", v)
	}
}

// Test_valueCopyByteCode_NonStructIsUnchanged verifies that non-struct values
// (a plain string here) pass through valueCopyByteCode completely unchanged
// — no error, same value, stack depth unaffected. This matters because
// hoistDeferCallArguments/hoistDeferReceiver emit ValueCopy unconditionally
// for every hoisted value, including ordinary scalar arguments.
func Test_valueCopyByteCode_NonStructIsUnchanged(t *testing.T) {
	tc := newTestContext(t).withStack("deferred")

	err := valueCopyByteCode(tc.ctx, nil)
	tc.assertNoError(err)

	tc.assertTopStack("deferred")
	tc.assertStackEmpty()
}

// Test_valueCopyByteCode_ArrayIsAliased verifies that a *data.Array is left
// aliased (the very same pointer) rather than being deep-copied — matching
// Go's own defer semantics, where a slice argument's header is copied but
// its backing array is shared, so element mutations after "defer" remain
// visible when the deferred call finally runs.
func Test_valueCopyByteCode_ArrayIsAliased(t *testing.T) {
	original := data.NewArrayFromInterfaces(data.IntType, 1, 2, 3)

	tc := newTestContext(t).withStack(original)

	err := valueCopyByteCode(tc.ctx, nil)
	tc.assertNoError(err)

	result, popErr := tc.ctx.Pop()
	if popErr != nil {
		t.Fatalf("Pop() after valueCopyByteCode: %v", popErr)
	}

	resultArray, ok := result.(*data.Array)
	if !ok {
		t.Fatalf("expected *data.Array on stack, got %T", result)
	}

	if resultArray != original {
		t.Error("valueCopyByteCode unexpectedly copied the array — arrays must stay aliased, like Go slices")
	}
}

// Test_valueCopyByteCode_EmptyStack verifies that an empty stack produces the
// ordinary Pop() underflow error rather than panicking.
func Test_valueCopyByteCode_EmptyStack(t *testing.T) {
	tc := newTestContext(t)

	err := valueCopyByteCode(tc.ctx, nil)

	if err == nil {
		t.Error("expected an error popping an empty stack, got nil")
	}
}
