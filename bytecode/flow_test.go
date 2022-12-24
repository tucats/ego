package bytecode

import (
	"reflect"
	"testing"

	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
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

func Test_typeCast(t *testing.T) {
	name := "call to a type"
	tests := []struct {
		name string
		t    *datatypes.Type
		v    interface{}
		want interface{}
		err  *errors.EgoError
	}{
		{
			name: "cast int to string",
			t:    &datatypes.StringType,
			v:    55,
			want: "55",
		},
		{
			name: "cast bool to string",
			t:    &datatypes.StringType,
			v:    true,
			want: "true",
		},
	}

	for _, tt := range tests {
		ctx := &Context{
			stack:          make([]interface{}, 5),
			stackPointer:   0,
			running:        true,
			symbols:        symbols.NewSymbolTable("cast test"),
			programCounter: 1,
			bc: &ByteCode{
				instructions: make([]Instruction, 5),
				emitPos:      5,
			},
		}

		// Push the type on the stack that is to be used as the function pointer,
		// then the value to convert.
		_ = ctx.stackPush(tt.t)
		_ = ctx.stackPush(tt.v)

		err := callByteCode(ctx, 1)
		if !errors.Nil(err) {
			e1 := nilError
			e2 := nilError

			if tt.err != nil {
				e1 = tt.err.Error()
			}

			if err != nil {
				e2 = err.Error()
			}

			if e1 == e2 {
				return
			}

			t.Errorf("%s() error %v", name, err)
		} else if tt.err != nil {
			t.Errorf("%s() expected error not reported: %v", name, tt.err)
		}

		v, err := ctx.Pop()
		if !errors.Nil(err) {
			t.Errorf("%s() pop error: %v", name, err)
		}

		if !reflect.DeepEqual(v, tt.want) {
			t.Errorf("%s() got: %#v, want %#v", name, v, tt.want)
		}
	}
}

func Test_localCallandReturnByteCode(t *testing.T) {
	const (
		symbolTableName    = "local call test"
		uninterestingValue = "uninteresting value"
	)

	ctx := &Context{
		stack:          make([]interface{}, 5),
		stackPointer:   0,
		running:        true,
		symbols:        symbols.NewSymbolTable(symbolTableName),
		programCounter: 1,
		bc: &ByteCode{
			instructions: make([]Instruction, 5),
			emitPos:      5,
		},
	}

	// Push something on the stack so the SP isn't zero and we can test
	// to see this is still here later.
	_ = ctx.stackPush(uninterestingValue)

	marker := NewStackMarker("defer test")
	_ = ctx.stackPush(marker)

	e := localCallByteCode(ctx, 5)
	if !errors.Nil(e) {
		t.Errorf("localCallByteCode unexpected error %v", e)
	}

	if ctx.programCounter != 5 {
		t.Errorf("localCallByteCode wrong program counter %v", ctx.programCounter)
	}

	// Test context frame info. Frame pointer points to start of available stack space
	// in new local frame.
	if ctx.framePointer != 3 {
		t.Errorf("localCallByteCode wrong fp value %v", ctx.framePointer)
	}

	f := ctx.stack[ctx.framePointer-1]
	if fp, ok := f.(CallFrame); !ok {
		t.Error("localCallByteCode missing call frame on stack")
	} else {
		if fp.symbols.Name != symbolTableName {
			t.Errorf("localCallByteCode saved symbol table name wrong: %v", fp.symbols.Name)
		}
	}

	// Push another symbol table, and push a data value on the stack.
	ctx.symbols = symbols.NewChildSymbolTable("local call child", ctx.symbols)
	_ = ctx.stackPush(3.14)

	// Execute the return, which should detect that it's a local frame and
	// pop it off again.
	e = returnByteCode(ctx, false)
	if !errors.Nil(e) {
		t.Errorf("localCallByteCode unexpected return error: %v", e)
	}

	// Drop the stack contents that may have accumulated during the
	// local call.
	e = dropToMarkerByteCode(ctx, marker)
	if !errors.Nil(e) {
		t.Errorf("localCallByteCode unexpected dropToMarker error: %v", e)
	}

	// Fetch the value we had pushed on the stack as a marker that was
	// left over from the local call's stack.
	d, e := ctx.Pop()
	if !errors.Nil(e) {
		t.Errorf("localCallByteCode unexpected pop error: %v", e)
	}

	if datatypes.GetString(d) != uninterestingValue {
		t.Errorf("localCallByteCode wrong TOS value: %#v", d)
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
