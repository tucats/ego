package compiler

import (
	"reflect"
	"testing"

	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/tokenizer"
)

func TestCompile(t *testing.T) {
	tests := []struct {
		name    string
		arg     string
		want    []bytecode.Instruction
		wantErr bool
	}{
		{
			name: "Simple block",
			arg:  "{ print ; print } ",
			want: []bytecode.Instruction{
				{Operation: bytecode.AtLine, Operand: 1},
				{Operation: bytecode.PushScope, Operand: nil},
				{Operation: bytecode.AtLine, Operand: 1},
				{Operation: bytecode.Newline, Operand: nil},
				{Operation: bytecode.AtLine, Operand: 1},
				{Operation: bytecode.Newline, Operand: nil},
				{Operation: bytecode.PopScope, Operand: nil},
			},
			wantErr: false,
		},
		{
			name: "Simple print",
			arg:  "print 1",
			want: []bytecode.Instruction{
				{Operation: bytecode.AtLine, Operand: 1},
				{Operation: bytecode.Push, Operand: 1},
				{Operation: bytecode.Print, Operand: nil},
				{Operation: bytecode.Newline, Operand: nil},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens := tokenizer.New(tt.arg)
			c := New(tt.name)
			// Make sure PRINT verb works for these tests, and that "free form"
			// statements are permitted without requiring a main()
			c.extensionsEnabled = true
			c.SetInteractive(true)

			bc, err := c.Compile("unit test", tokens)
			if (!errors.Nil(err)) != tt.wantErr {
				t.Errorf("Compile() error = %v, wantErr %v", err, tt.wantErr)

				return
			}
			opcodes := bc.Opcodes()
			if !reflect.DeepEqual(opcodes, tt.want) {
				t.Errorf("Compile() = %v, want %v", bytecode.Format(opcodes), bytecode.Format(tt.want))
			}
		})
	}
}
