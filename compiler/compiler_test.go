package compiler

import (
	"reflect"
	"testing"

	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/tokenizer"
)

func TestCompile(t *testing.T) {
	tests := []struct {
		name    string
		arg     string
		want    []bytecode.I
		wantErr bool
	}{
		{
			name: "for index loop with break",
			arg:  "for i := 0; i < 10; i = i + 1 { if i == 3 { break }; print i }",
			want: []bytecode.I{
				{Operation: bytecode.AtLine, Operand: 1},
				{Operation: bytecode.PushScope, Operand: nil},
				{Operation: bytecode.Push, Operand: 0},
				{Operation: bytecode.SymbolCreate, Operand: "i"},
				{Operation: bytecode.Store, Operand: "i"},
				{Operation: bytecode.Load, Operand: "i"},
				{Operation: bytecode.Push, Operand: 10},
				{Operation: bytecode.LessThan, Operand: nil},
				{Operation: bytecode.BranchFalse, Operand: 33},
				{Operation: bytecode.AtLine, Operand: 1},
				{Operation: bytecode.PushScope, Operand: nil},
				{Operation: bytecode.AtLine, Operand: 1},
				{Operation: bytecode.Load, Operand: "bool"},
				{Operation: bytecode.Load, Operand: "i"},
				{Operation: bytecode.Push, Operand: 3},
				{Operation: bytecode.Equal, Operand: nil},
				{Operation: bytecode.Call, Operand: 1},
				{Operation: bytecode.BranchFalse, Operand: 23},
				{Operation: bytecode.AtLine, Operand: 1},
				{Operation: bytecode.PushScope, Operand: nil},
				{Operation: bytecode.AtLine, Operand: 1},
				{Operation: bytecode.Branch, Operand: 33},
				{Operation: bytecode.PopScope, Operand: nil},
				{Operation: bytecode.AtLine, Operand: 1},
				{Operation: bytecode.Load, Operand: "i"},
				{Operation: bytecode.Print, Operand: nil},
				{Operation: bytecode.Newline, Operand: nil},
				{Operation: bytecode.PopScope, Operand: nil},
				{Operation: bytecode.Load, Operand: "i"},
				{Operation: bytecode.Push, Operand: 1},
				{Operation: bytecode.Add, Operand: nil},
				{Operation: bytecode.Store, Operand: "i"},
				{Operation: bytecode.Branch, Operand: 5},
				{Operation: bytecode.PopScope, Operand: nil},
			},
		},
		{
			name: "Simple block",
			arg:  "{ print ; print } ",
			want: []bytecode.I{
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
			name: "store to _",
			arg:  "_ = 3",
			want: []bytecode.I{
				{Operation: bytecode.AtLine, Operand: 1},
				{Operation: bytecode.Push, Operand: 3},
				{Operation: bytecode.Drop, Operand: 1},
			},
			wantErr: false,
		},
		{
			name: "Simple print",
			arg:  "print 1",
			want: []bytecode.I{
				{Operation: bytecode.AtLine, Operand: 1},
				{Operation: bytecode.Push, Operand: 1},
				{Operation: bytecode.Print, Operand: nil},
				{Operation: bytecode.Newline, Operand: nil},
			},
			wantErr: false,
		},
		{
			name: "Simple if else",
			arg:  "if false print 1 else print 2",
			want: []bytecode.I{
				{Operation: bytecode.AtLine, Operand: 1},
				{Operation: bytecode.Load, Operand: "bool"},
				{Operation: bytecode.Push, Operand: false},
				{Operation: bytecode.Call, Operand: 1},
				{Operation: bytecode.BranchFalse, Operand: 10},
				{Operation: bytecode.AtLine, Operand: 1},
				{Operation: bytecode.Push, Operand: 1},
				{Operation: bytecode.Print, Operand: nil},
				{Operation: bytecode.Newline, Operand: nil},
				{Operation: bytecode.Branch, Operand: 14},
				{Operation: bytecode.AtLine, Operand: 1},
				{Operation: bytecode.Push, Operand: 2},
				{Operation: bytecode.Print, Operand: nil},
				{Operation: bytecode.Newline, Operand: nil},
			},
		},
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens := tokenizer.New(tt.arg)
			c := New()
			// Make sure PRINT verb works for these tests.
			c.extensionsEnabled = true
			bc, err := c.Compile("unit test", tokens)
			if (err != nil) != tt.wantErr {
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
