package bytecode

import (
	"reflect"
	"testing"

	"github.com/tucats/ego/errors"
)

func TestByteCode_New(t *testing.T) {
	t.Run("test", func(t *testing.T) {
		b := New("testing")

		want := ByteCode{
			Name:         "testing",
			instructions: make([]instruction, initialOpcodeSize),
			nextAddress:  0,
		}
		if !reflect.DeepEqual(*b, want) {
			t.Error("new() did not return expected object")
		}
	})
}

func TestByteCode_Emit2(t *testing.T) {
	type fields struct {
		Name    string
		opcodes []instruction
		emitPos int
	}

	type args struct {
		emit []instruction
	}

	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []instruction
		emitPos int
	}{
		{
			name: "first emit",
			fields: fields{
				opcodes: []instruction{},
			},
			args: args{
				emit: []instruction{
					{Push, 33},
				},
			},
			want: []instruction{
				{Push, 33},
			},
			emitPos: 1,
		},
		{
			name: "multiple emit",
			fields: fields{
				opcodes: []instruction{},
			},
			args: args{
				emit: []instruction{
					{Push, 33},
					{Push, "stuff"},
					{Operation: Add},
					{Operation: Stop},
				},
			},
			want: []instruction{
				{Push, 33},
				{Push, "stuff"},
				{Operation: Add},
				{Operation: Stop},
			},
			emitPos: 4,
		},
		// TODO: Add test cases.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &ByteCode{
				Name:         tt.fields.Name,
				instructions: tt.fields.opcodes,
				nextAddress:  tt.fields.emitPos,
			}
			for _, i := range tt.args.emit {
				b.Emit(i.Operation, i.Operand)
			}

			b.Seal()

			if tt.emitPos > 0 && b.Mark() != tt.emitPos {
				t.Errorf("invalid emit position after emit: %d", b.Mark())
			}

			for n, i := range b.instructions {
				if n < b.nextAddress && i != tt.want[n] {
					t.Error("opcode mismatch")
				}
			}
		})
	}
}

func TestByteCode_EmitArrayOfOperands(t *testing.T) {
	t.Run("emit with array", func(t *testing.T) {
		b := &ByteCode{
			Name:         "emit with array of operands",
			instructions: []instruction{},
			nextAddress:  0,
		}

		b.Emit(ArgCheck, 10, 20, "foobar")

		// Some misc extra unit testing here. Append with a nil value
		b.Append(nil)

		// GetInstruction with invalid index
		i := b.Instruction(500)
		if i != nil {
			t.Errorf("expected nil, got: %v", i)
		}

		// GetInstruction with valid index
		i = b.Instruction(0)

		if !reflect.DeepEqual(i, &instruction{
			Operand:   []interface{}{10, 20, "foobar"},
			Operation: ArgCheck,
		}) {
			t.Errorf("incorrect instruction created: %v", b.instructions[0])
		}
	})
}

func TestByteCode_SetAddress(t *testing.T) {
	t.Run("emit with array", func(t *testing.T) {
		b := &ByteCode{
			Name:         "setAddress",
			instructions: []instruction{},
			nextAddress:  0,
		}

		savedMark := b.Mark()
		b.Emit(Branch, nil)

		b.Emit(Stop)
		e1 := b.SetAddressHere(savedMark)

		if e1 != nil {
			t.Errorf("Unexpected error from setAddressHere: %v", e1)
		}

		if !reflect.DeepEqual(b.instructions[0],
			instruction{
				Operation: Branch,
				Operand:   2,
			}) {
			t.Errorf("Incorrect bytecode fixup: %v", b.instructions[0])
		}

		e1 = b.SetAddressHere(500)
		if e1.Error() != errors.ErrInvalidBytecodeAddress.Error() {
			t.Errorf("Expected error not seen: %v", e1)
		}
	})
}

func TestByteCode_Append(t *testing.T) {
	type fields struct {
		Name    string
		opcodes []instruction
		emitPos int
	}

	type args struct {
		a *ByteCode
	}

	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []instruction
		wantPos int
	}{
		{
			name: "simple append",
			fields: fields{
				opcodes: []instruction{
					{Push, 0},
					{Push, 0},
				},
				emitPos: 2,
			},
			args: args{
				a: &ByteCode{
					instructions: []instruction{
						{Add, nil},
					},
					nextAddress: 1,
				},
			},
			want: []instruction{
				{Push, 0},
				{Push, 0},
				{Add, nil},
			},
			wantPos: 3,
		},
		{
			name: "branch append",
			fields: fields{
				opcodes: []instruction{
					{Push, 11},
					{Push, 22},
				},
				emitPos: 2,
			},
			args: args{
				a: &ByteCode{
					instructions: []instruction{
						{Branch, 2}, // Must be updated
						{Add, nil},
					},
					nextAddress: 2,
				},
			},
			want: []instruction{
				{Push, 11},
				{Push, 22},
				{Branch, 4}, // Updated from new offset
				{Add, nil},
			},
			wantPos: 4,
		},
		// TODO: Add test cases.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &ByteCode{
				Name:         tt.fields.Name,
				instructions: tt.fields.opcodes,
				nextAddress:  tt.fields.emitPos,
			}
			b.Append(tt.args.a)
			if tt.wantPos != b.nextAddress {
				t.Errorf("Append() wrong emitPos, got %d, want %d", b.nextAddress, tt.wantPos)
			}
			// Check the slice of intentionally emitted opcodes (array may be larger)
			if !reflect.DeepEqual(tt.want, b.instructions[:tt.wantPos]) {
				t.Errorf("Append() wrong array, got %v, want %v", b.instructions, tt.want)
			}
		})
	}
}
