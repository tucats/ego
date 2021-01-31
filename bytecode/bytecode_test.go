package bytecode

import (
	"reflect"
	"testing"

	"github.com/tucats/ego/symbols"
)

func TestByteCode_New(t *testing.T) {
	t.Run("test", func(t *testing.T) {
		b := New("testing")

		want := ByteCode{
			Name:    "testing",
			opcodes: make([]I, InitialOpcodeSize),
			Symbols: &symbols.SymbolTable{Symbols: map[string]interface{}{}},
			emitPos: 0,
		}
		if !reflect.DeepEqual(*b, want) {
			t.Error("new() did not return expected object")
		}
	})
}

func TestByteCode_Emit2(t *testing.T) {
	type fields struct {
		Name    string
		opcodes []I
		emitPos int
	}
	type args struct {
		emit []I
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   []I
	}{
		{
			name: "first emit",
			fields: fields{
				opcodes: []I{},
			},
			args: args{
				emit: []I{
					{Push, 33},
				},
			},
			want: []I{
				{Push, 33},
			},
		},
		{
			name: "multiple emit",
			fields: fields{
				opcodes: []I{},
			},
			args: args{
				emit: []I{
					{Push, 33},
					{Push, "stuff"},
					{Operation: Add},
					{Operation: Stop},
				},
			},
			want: []I{
				{Push, 33},
				{Push, "stuff"},
				{Operation: Add},
				{Operation: Stop},
			},
		},
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &ByteCode{
				Name:    tt.fields.Name,
				opcodes: tt.fields.opcodes,
				emitPos: tt.fields.emitPos,
			}
			for _, i := range tt.args.emit {
				b.Emit(i.Operation, i.Operand)
			}

			for n, i := range b.opcodes {
				if n < b.emitPos && i != tt.want[n] {
					t.Error("opcode mismatch")
				}
			}
		})
	}
}

func TestByteCode_Append(t *testing.T) {
	type fields struct {
		Name    string
		opcodes []I
		emitPos int
	}
	type args struct {
		a *ByteCode
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []I
		wantPos int
	}{
		{
			name: "simple append",
			fields: fields{
				opcodes: []I{
					{Push, 0},
					{Push, 0},
				},
				emitPos: 2,
			},
			args: args{
				a: &ByteCode{
					opcodes: []I{
						{Add, nil},
					},
					emitPos: 1,
				},
			},
			want: []I{
				{Push, 0},
				{Push, 0},
				{Add, nil},
			},
			wantPos: 3,
		},
		{
			name: "branch append",
			fields: fields{
				opcodes: []I{
					{Push, 11},
					{Push, 22},
				},
				emitPos: 2,
			},
			args: args{
				a: &ByteCode{
					opcodes: []I{
						{Branch, 2}, // Must be updated
						{Add, nil},
					},
					emitPos: 2,
				},
			},
			want: []I{
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
				Name:    tt.fields.Name,
				opcodes: tt.fields.opcodes,
				emitPos: tt.fields.emitPos,
			}
			b.Append(tt.args.a)
			if tt.wantPos != b.emitPos {
				t.Errorf("Append() wrong emitPos, got %d, want %d", b.emitPos, tt.wantPos)
			}
			// Check the slice of intentionally emitted opcodes (array may be larger)
			if !reflect.DeepEqual(tt.want, b.opcodes[:tt.wantPos]) {
				t.Errorf("Append() wrong array, got %v, want %v", b.opcodes, tt.want)
			}
		})
	}
}
