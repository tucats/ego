package bytecode

import (
	"reflect"
	"testing"

	"github.com/tucats/ego/functions"
	"github.com/tucats/ego/symbols"
)

func TestByteCode_Run(t *testing.T) {
	type fields struct {
		Name    string
		opcodes []I
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
				opcodes: []I{
					{Operation: Stop},
				},
			},
		},
		{
			name: "push int",
			fields: fields{
				opcodes: []I{
					{Operation: Push, Operand: 42},
					{Operation: Stop},
				},
				result: 42,
			},
		},
		{
			name: "drop 2 stack items",
			fields: fields{
				opcodes: []I{
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
			name: "push float",
			fields: fields{
				opcodes: []I{
					{Operation: Push, Operand: 3.14},
					{Operation: Stop},
				},
				result: 3.14,
			},
		},
		{
			name: "add int",
			fields: fields{
				opcodes: []I{
					{Operation: Push, Operand: 5},
					{Operation: Push, Operand: 7},
					{Operation: Add},
					{Operation: Stop},
				},
				result: 12,
			},
		},
		{
			name: "add float to int",
			fields: fields{
				opcodes: []I{
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
				opcodes: []I{
					{Operation: Push, Operand: 5},
					{Operation: Push, Operand: 8},
					{Operation: Sub},
					{Operation: Stop},
				},
				result: -3,
			},
		},
		{
			name: "div float by int",
			fields: fields{
				opcodes: []I{
					{Operation: Push, Operand: 10.0},
					{Operation: Push, Operand: 2},
					{Operation: Div},
					{Operation: Stop},
				},
				result: 5.0,
			},
		},
		{
			name: "mul float by float",
			fields: fields{
				opcodes: []I{
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
				opcodes: []I{
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
				opcodes: []I{
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
				opcodes: []I{
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
				opcodes: []I{
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
				opcodes: []I{
					{Operation: Push, Operand: false},
					{Operation: Push, Operand: false},
					{Operation: NotEqual},
					{Operation: Stop},
				},
				result: false,
			},
		},
		{
			name: "not equal float test",
			fields: fields{
				opcodes: []I{
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
				opcodes: []I{
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
				opcodes: []I{
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
				opcodes: []I{
					{Operation: Load, Operand: "len"},
					{Operation: Push, Operand: "fruitcake"},
					{Operation: Call, Operand: 1},
					{Operation: Stop},
				},
				result: 9,
			},
		},
		{
			name: "left(n, 5) of string constant",
			fields: fields{
				opcodes: []I{
					// Arguments are pushed in the order parsed
					{Operation: Load, Operand: "strings"},
					{Operation: Push, Operand: "Left"},
					{Operation: Member},
					{Operation: Push, Operand: "fruitcake"},
					{Operation: Push, Operand: 5},
					{Operation: Call, Operand: 2},
					{Operation: Stop},
				},
				result: "fruit",
			},
		},
		{
			name: "simple branch",
			fields: fields{
				opcodes: []I{
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
				opcodes: []I{

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
				opcodes: []I{

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
				Name:    tt.fields.Name,
				opcodes: tt.fields.opcodes,
				emitPos: tt.fields.emitPos,
			}
			b.emitPos = len(b.opcodes)
			s := symbols.NewSymbolTable(tt.name)
			c := NewContext(s, b)
			functions.AddBuiltins(c.symbols)

			err := c.Run()
			if err != nil && err.Error() == "stop" {
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
