package bytecode

import (
	"reflect"
	"testing"

	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/functions"
	"github.com/tucats/ego/symbols"
)

func TestByteCode_Run(t *testing.T) {
	type fields struct {
		Name    string
		opcodes []instruction
		emitPos int
		result  interface{}
	}

	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{

		{
			name: "stop",
			fields: fields{
				opcodes: []instruction{
					{Operation: Stop},
				},
			},
		},
		{
			name: "push int",
			fields: fields{
				opcodes: []instruction{
					{Operation: Push, Operand: 42},
					{Operation: Stop},
				},
				result: 42,
			},
		},
		{
			name: "drop 2 stack items",
			fields: fields{
				opcodes: []instruction{
					{Operation: Push, Operand: 42},
					{Operation: Push, Operand: 43},
					{Operation: Push, Operand: 44},
					{Operation: Drop, Operand: 2},
					{Operation: Stop},
				},
				result: 42,
			},
		},
		{
			name: "push float64",
			fields: fields{
				opcodes: []instruction{
					{Operation: Push, Operand: 3.14},
					{Operation: Stop},
				},
				result: 3.14,
			},
		},
		{
			name: "add int",
			fields: fields{
				opcodes: []instruction{
					{Operation: Push, Operand: 5},
					{Operation: Push, Operand: 7},
					{Operation: Add},
					{Operation: Stop},
				},
				result: 12,
			},
		},
		{
			name: "add float64 to int",
			fields: fields{
				opcodes: []instruction{
					{Operation: Push, Operand: 3.14},
					{Operation: Push, Operand: 7},
					{Operation: Add},
					{Operation: Stop},
				},
				result: 10.14,
			},
		},
		{
			name: "sub int from  int",
			fields: fields{
				opcodes: []instruction{
					{Operation: Push, Operand: 5},
					{Operation: Push, Operand: 8},
					{Operation: Sub},
					{Operation: Stop},
				},
				result: -3,
			},
		},
		{
			name: "div float64 by int",
			fields: fields{
				opcodes: []instruction{
					{Operation: Push, Operand: 10.0},
					{Operation: Push, Operand: 2},
					{Operation: Div},
					{Operation: Stop},
				},
				result: 5.0,
			},
		},
		{
			name: "mul float64 by float64",
			fields: fields{
				opcodes: []instruction{
					{Operation: Push, Operand: 3.0},
					{Operation: Push, Operand: 4.0},
					{Operation: Mul},
					{Operation: Stop},
				},
				result: 12.0,
			},
		},
		{
			name: "equal int test",
			fields: fields{
				opcodes: []instruction{
					{Operation: Push, Operand: 5},
					{Operation: Push, Operand: 5},
					{Operation: Equal},
					{Operation: Stop},
				},
				result: true,
			},
		},
		{
			name: "equal mixed test",
			fields: fields{
				opcodes: []instruction{
					{Operation: Push, Operand: 5},
					{Operation: Push, Operand: 5.0},
					{Operation: Equal},
					{Operation: Stop},
				},
				result: true,
			},
		},
		{
			name: "not equal int test",
			fields: fields{
				opcodes: []instruction{
					{Operation: Push, Operand: 5},
					{Operation: Push, Operand: 5},
					{Operation: NotEqual},
					{Operation: Stop},
				},
				result: false,
			},
		},
		{
			name: "not equal string test",
			fields: fields{
				opcodes: []instruction{
					{Operation: Push, Operand: "fruit"},
					{Operation: Push, Operand: "fruit"},
					{Operation: NotEqual},
					{Operation: Stop},
				},
				result: false,
			},
		},
		{
			name: "not equal bool test",
			fields: fields{
				opcodes: []instruction{
					{Operation: Push, Operand: false},
					{Operation: Push, Operand: false},
					{Operation: NotEqual},
					{Operation: Stop},
				},
				result: false,
			},
		},
		{
			name: "not equal float64 test",
			fields: fields{
				opcodes: []instruction{
					{Operation: Push, Operand: 5.00000},
					{Operation: Push, Operand: 5.00001},
					{Operation: NotEqual},
					{Operation: Stop},
				},
				result: true,
			},
		},
		{
			name: "greater than test",
			fields: fields{
				opcodes: []instruction{
					{Operation: Push, Operand: 11.0},
					{Operation: Push, Operand: 5},
					{Operation: GreaterThan},
					{Operation: Stop},
				},
				result: true,
			},
		},
		{
			name: "greater than or equals test",
			fields: fields{
				opcodes: []instruction{
					{Operation: Push, Operand: "tom"},
					{Operation: Push, Operand: "tom"},
					{Operation: GreaterThanOrEqual},
					{Operation: Stop},
				},
				result: true,
			},
		},
		{
			name: "length of string constant",
			fields: fields{
				opcodes: []instruction{
					{Operation: Load, Operand: "len"},
					{Operation: Push, Operand: "fruitcake"},
					{Operation: Call, Operand: 1},
					{Operation: Stop},
				},
				result: 9,
			},
		},
		{
			name: "simple branch",
			fields: fields{
				opcodes: []instruction{
					{Operation: Push, Operand: "fruitcake"},
					{Operation: Branch, Operand: 3},
					{Operation: Push, Operand: "Left"},
					{Operation: Stop},
				},
				result: "fruitcake",
			},
		},
		{
			name: "if-true branch",
			fields: fields{
				opcodes: []instruction{

					// Use of "short-form" instruction initializer requires passing nil
					// for those instructions without an operand
					{Push, "stuff"},
					{Push, "fruitcake"},
					{Push, "fruitcake"},
					{Equal, nil},
					{BranchTrue, 6},
					{Push, .33333},
					{Stop, nil},
				},
				result: "stuff",
			},
		},
		{
			name: "if-false branch",
			fields: fields{
				opcodes: []instruction{

					// Use of "short-form" instruction initializer requires passing nil
					// for those instructions without an operand
					{Push, "stuff"},
					{Push, "cake"},
					{Push, "fruitcake"},
					{Equal, nil},
					{BranchTrue, 6},
					{Push, 42},
					{Stop, nil},
				},
				result: 42,
			},
		}, // TODO: Add test cases.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &ByteCode{
				name:         tt.fields.Name,
				instructions: tt.fields.opcodes,
				nextAddress:  tt.fields.emitPos,
			}
			b.nextAddress = len(b.instructions)
			s := symbols.NewSymbolTable(tt.name)
			c := NewContext(s, b)
			functions.AddBuiltins(c.symbols)

			err := c.Run()
			if errors.Equals(err, errors.ErrStop) {
				err = nil
			}
			if (err != nil) != tt.wantErr {
				t.Errorf("ByteCode.Run() error = %v, wantErr %v", err, tt.wantErr)
			}
			if c.running {
				t.Error("ByteCode Run() failed to stop interpreter")
			}
			if tt.fields.result != nil {
				v, err := c.Pop()
				if err != nil && !tt.wantErr {
					t.Error("ByteCode Run() unexpected " + err.Error())
				}
				if !reflect.DeepEqual(tt.fields.result, v) {
					t.Errorf("ByteCode Run() got %v, want %v ", v, tt.fields.result)
				}
			}
		})
	}
}
