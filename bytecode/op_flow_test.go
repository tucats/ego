package bytecode

import (
	"testing"

	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
)

func Test_stopByteCode(t *testing.T) {
	ctx := &Context{running: true}

	if e := stopByteCode(ctx, nil); !e.Equal(errors.ErrStop) {
		t.Errorf("stopByteCode unexpected error %v", e)
	}

	if ctx.running {
		t.Errorf("stopByteCode did not turn off running flag")
	}
}

func Test_panicByteCode(t *testing.T) {
	ctx := &Context{
		stack:        []interface{}{"test"},
		stackPointer: 1,
		running:      true,
	}

	// Need to do a temporary override of this value to ensure that
	// the panic only returns an error rather than abending.
	settings.Set(defs.RuntimePanicsSetting, "false")

	e := panicByteCode(ctx, nil)

	if e.GetContext() != "panic" {
		t.Errorf("panicByteCode wrong context %v", e)
	}
}

func Test_branchFalseByteCode(t *testing.T) {
	ctx := &Context{
		stack:          make([]interface{}, 5),
		stackPointer:   0,
		running:        true,
		programCounter: 1,
		bc: &ByteCode{
			instructions: make([]Instruction, 5),
			emitPos:      5,
		},
	}

	// Test if TOS is false
	_ = ctx.stackPush(false)

	e := branchFalseByteCode(ctx, 2)
	if !errors.Nil(e) {
		t.Errorf("branchFalseByteCode unexpected error %v", e)
	}

	if ctx.programCounter != 2 {
		t.Errorf("branchFalseByteCode wrong program counter %v", ctx.programCounter)
	}

	// Test if TOS is true
	_ = ctx.stackPush(true)

	e = branchFalseByteCode(ctx, 1)
	if !errors.Nil(e) {
		t.Errorf("branchFalseByteCode unexpected error %v", e)
	}

	if ctx.programCounter != 2 {
		t.Errorf("branchFalseByteCode wrong program counter %v", ctx.programCounter)
	}

	// Test if target is invalid
	_ = ctx.stackPush(true)

	e = branchTrueByteCode(ctx, 20)
	if !e.Equal(errors.ErrInvalidBytecodeAddress) {
		t.Errorf("branchFalseByteCode unexpected error %v", e)
	}

	if ctx.programCounter != 2 {
		t.Errorf("branchFalseByteCode wrong program counter %v", ctx.programCounter)
	}
}

func Test_branchTrueByteCode(t *testing.T) {
	ctx := &Context{
		stack:          make([]interface{}, 5),
		stackPointer:   0,
		running:        true,
		programCounter: 1,
		bc: &ByteCode{
			instructions: make([]Instruction, 5),
			emitPos:      5,
		},
	}

	// Test if TOS is false
	_ = ctx.stackPush(false)

	e := branchTrueByteCode(ctx, 2)
	if !errors.Nil(e) {
		t.Errorf("branchTrueByteCode unexpected error %v", e)
	}

	if ctx.programCounter != 1 {
		t.Errorf("branchTrueByteCode wrong program counter %v", ctx.programCounter)
	}

	// Test if TOS is true
	_ = ctx.stackPush(true)

	e = branchTrueByteCode(ctx, 2)
	if !errors.Nil(e) {
		t.Errorf("branchTrueByteCode unexpected error %v", e)
	}

	if ctx.programCounter != 2 {
		t.Errorf("branchTrueByteCode wrong program counter %v", ctx.programCounter)
	}

	// Test if target is invalid
	_ = ctx.stackPush(true)

	e = branchTrueByteCode(ctx, 20)
	if !e.Equal(errors.ErrInvalidBytecodeAddress) {
		t.Errorf("branchTrueByteCode unexpected error %v", e)
	}

	if ctx.programCounter != 2 {
		t.Errorf("branchTrueByteCode wrong program counter %v", ctx.programCounter)
	}
}
