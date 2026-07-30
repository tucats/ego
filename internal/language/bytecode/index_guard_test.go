package bytecode

// Regression tests for the INDEX-2 through INDEX-6 fixes. Each covers an index
// expression whose index came from an untrustworthy source -- a saved bytecode
// address, an instruction operand, or a saved frame pointer -- and which either
// had no range check or had one that admitted an out-of-range value.

import (
	"testing"

	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/symbols"
)

// INDEX-2: the guard was "mark > b.nextAddress", so a mark exactly one past the
// last emitted instruction was accepted. On a sealed bytecode that indexed past
// the end of the truncated instruction slice and panicked.
func TestSetAddressPastLastInstruction_INDEX2(t *testing.T) {
	b := New("index-2")
	b.Emit(Push, 1)
	b.Emit(Push, 2)
	b.Seal()

	// nextAddress names the slot after the last instruction; it is not a valid
	// mark and must be rejected rather than indexed.
	if err := b.SetAddress(b.nextAddress, 0); !errors.Equal(err, errors.ErrInvalidBytecodeAddress) {
		t.Errorf("SetAddress(nextAddress) error = %v, want ErrInvalidBytecodeAddress", err)
	}

	if err := b.SetAddress(-1, 0); !errors.Equal(err, errors.ErrInvalidBytecodeAddress) {
		t.Errorf("SetAddress(-1) error = %v, want ErrInvalidBytecodeAddress", err)
	}

	// A mark that does name an emitted instruction must still work.
	if err := b.SetAddress(1, 42); err != nil {
		t.Errorf("SetAddress(1, 42) unexpected error %v", err)
	}

	if operand := b.Instruction(1).Operand; operand != 42 {
		t.Errorf("instruction 1 operand = %v, want 42", operand)
	}
}

// INDEX-2: an empty bytecode has no instruction to patch, so mark 0 is invalid.
// This is the shape of the SetAddress(0, count) call in compiler/lvalue.go.
func TestSetAddressOnEmptyBytecode_INDEX2(t *testing.T) {
	b := New("index-2 empty")

	if err := b.SetAddress(0, 1); !errors.Equal(err, errors.ErrInvalidBytecodeAddress) {
		t.Errorf("SetAddress(0, 1) on empty bytecode error = %v, want ErrInvalidBytecodeAddress", err)
	}
}

// INDEX-3: Remove had no bounds check, and its negative-offset arithmetic added
// distance past the end instead of counting back from it, so Remove(-1) -- the
// documented way to drop the last instruction -- panicked.
func TestRemoveNegativeOffset_INDEX3(t *testing.T) {
	b := New("index-3")
	b.Emit(Push, 1)
	b.Emit(Push, 2)
	b.Emit(Push, 3)

	b.Remove(-1)

	if b.nextAddress != 2 {
		t.Fatalf("after Remove(-1), nextAddress = %d, want 2", b.nextAddress)
	}

	// The remaining instructions must be the first two, in order.
	for address, want := range []int{1, 2} {
		if operand := b.Instruction(address).Operand; operand != want {
			t.Errorf("after Remove(-1), instruction %d operand = %v, want %v", address, operand, want)
		}
	}
}

// INDEX-3: an out-of-range address is a no-op, matching Delete's convention in
// the same file, rather than panicking on the slice expressions.
func TestRemoveOutOfRange_INDEX3(t *testing.T) {
	b := New("index-3 range")
	b.Emit(Push, 1)
	b.Emit(Push, 2)

	for _, address := range []int{2, 99, -3, -99} {
		b.Remove(address)

		if b.nextAddress != 2 {
			t.Fatalf("Remove(%d) changed nextAddress to %d, want 2", address, b.nextAddress)
		}
	}
}

// INDEX-4: Patch trusted the window it was given. A window extending past the
// emitted instructions made the tail length negative, panicking inside make().
func TestPatchWindowPastEnd_INDEX4(t *testing.T) {
	b := New("index-4")
	b.Emit(Push, 1)
	b.Emit(Push, 2)

	before := b.nextAddress

	// deleteSize runs well past the end of the emitted instructions.
	b.Patch(1, 50, []instruction{{Operation: Push, Operand: 9}})

	if b.nextAddress != before {
		t.Errorf("out-of-range Patch changed nextAddress to %d, want %d", b.nextAddress, before)
	}

	// A negative start is equally invalid.
	b.Patch(-1, 1, []instruction{{Operation: Push, Operand: 9}})

	if b.nextAddress != before {
		t.Errorf("negative-start Patch changed nextAddress to %d, want %d", b.nextAddress, before)
	}

	// A valid window still patches.
	b.Patch(0, 1, []instruction{{Operation: Push, Operand: 7}})

	if operand := b.Instruction(0).Operand; operand != 7 {
		t.Errorf("after valid Patch, instruction 0 operand = %v, want 7", operand)
	}
}

// INDEX-5: the marker scan started at stackPointer-(count-1), which is at or
// above the top of the stack for a small count. Because the stack slice can be
// exactly full, that read past the end of the slice.
func TestStackCheckFullStack_INDEX5(t *testing.T) {
	tc := newTestContext(t)

	// Fill the stack so that stackPointer == len(stack), the state in which the
	// old scan indexed past the end.
	tc.withStack(1, 2, 3)
	tc.ctx.stack = tc.ctx.stack[:tc.ctx.stackPointer]

	// No marker is present, so this must report ErrReturnValueCount rather than
	// panicking on the scan.
	err := stackCheckByteCode(tc.ctx, 1)
	if !errors.Equal(err, errors.ErrReturnValueCount) {
		t.Errorf("stackCheckByteCode(1) error = %v, want ErrReturnValueCount", err)
	}

	// A zero count made the old scan start one *above* the stack pointer.
	err = stackCheckByteCode(tc.ctx, 0)
	if !errors.Equal(err, errors.ErrReturnValueCount) {
		t.Errorf("stackCheckByteCode(0) error = %v, want ErrReturnValueCount", err)
	}
}

// INDEX-5: a marker on the stack is still found once the scan is clamped, so
// the fix does not break the case the scan exists to handle.
func TestStackCheckFindsMarker_INDEX5(t *testing.T) {
	tc := newTestContext(t)
	tc.withStack(NewStackMarker("test"), 1, 2)

	if err := stackCheckByteCode(tc.ctx, 2); err != nil {
		t.Errorf("stackCheckByteCode(2) unexpected error %v", err)
	}
}

// INDEX-6: the frame walks checked framePointer > 0 but not the length of the
// stack. callFramePop truncates c.stack, so a saved fp can name a position that
// no longer exists -- turning a stack trace into a panic.
func TestFrameWalkStalePointer_INDEX6(t *testing.T) {
	tc := newTestContext(t)
	tc.withStack(1, 2, 3)

	// Simulate a frame pointer left behind after the stack was truncated.
	tc.ctx.stack = tc.ctx.stack[:2]
	tc.ctx.stackPointer = 2
	tc.ctx.framePointer = 9

	// None of these may panic; each must degrade to a "no frames" answer.
	if got := tc.ctx.frameAt(tc.ctx.framePointer); got != nil {
		t.Errorf("frameAt(9) = %v, want nil", got)
	}

	if text := tc.ctx.FormatFrames(IncludeSymbolTableNames); text == "" {
		t.Error("FormatFrames returned an empty string, want a header line")
	}

	module, line, _ := tc.ctx.GetFrame(1)
	if module != "" || line != 0 {
		t.Errorf("GetFrame(1) = (%q, %d), want (\"\", 0)", module, line)
	}

	tc.ctx.SetBreakOnReturn()
}

// INDEX-6: frameAt returns the frame when the pointer is valid, so the walks
// still work for a real call frame.
func TestFrameAtValidPointer_INDEX6(t *testing.T) {
	tc := newTestContext(t)

	frame := &CallFrame{
		Module:  "testmod",
		Line:    17,
		symbols: symbols.NewSymbolTable("frame table"),
	}

	tc.withStack(frame)
	tc.ctx.framePointer = tc.ctx.stackPointer

	got := tc.ctx.frameAt(tc.ctx.framePointer)
	if got == nil {
		t.Fatal("frameAt returned nil for a valid frame pointer")
	}

	if got.Module != "testmod" || got.Line != 17 {
		t.Errorf("frameAt returned (%q, %d), want (\"testmod\", 17)", got.Module, got.Line)
	}

	// SetBreakOnReturn must reach the frame through the same accessor.
	tc.ctx.SetBreakOnReturn()

	if !frame.breakOnReturn {
		t.Error("SetBreakOnReturn did not mark the call frame")
	}
}
