package compiler

import (
	"testing"

	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/tokenizer"
)

func TestCompiler_compileConst(t *testing.T) {
	type fields struct {
		t             *tokenizer.Tokenizer
		symbols       *symbols.SymbolTable
		flags         flagSet
		functionDepth int
		constants     []string
		b             *bytecode.ByteCode
	}

	tests := []struct {
		name    string
		fields  fields
		wantErr *errors.Error
	}{
		{
			name: "empty list",
			fields: fields{
				t:             tokenizer.New("()", true),
				symbols:       symbols.NewRootSymbolTable("test"),
				functionDepth: 0,
				constants:     []string{},
				b:             bytecode.New("test"),
			},
			wantErr: nil,
		},
		{
			name: "single constant",
			fields: fields{
				t:             tokenizer.New("foo = 42", true),
				symbols:       symbols.NewRootSymbolTable("test"),
				functionDepth: 0,
				constants:     []string{},
				b:             bytecode.New("test"),
			},
			wantErr: nil,
		},
		{
			name: "multiple constants",
			fields: fields{
				t:             tokenizer.New("(foo = 42; bar = 3.14)", true),
				symbols:       symbols.NewRootSymbolTable("test"),
				functionDepth: 0,
				constants:     []string{},
				b:             bytecode.New("test"),
			},
			wantErr: nil,
		},
		{
			name: "invalid constant name",
			fields: fields{
				t:             tokenizer.New("(1 = 42)", true),
				symbols:       symbols.NewRootSymbolTable("test"),
				functionDepth: 0,
				constants:     []string{},
				b:             bytecode.New("test"),
			},
			wantErr: errors.NewError(errors.ErrInvalidSymbolName).Context("1"),
		},
		{
			name: "missing equal sign",
			fields: fields{
				t:             tokenizer.New("(foo 42)", true),
				symbols:       symbols.NewRootSymbolTable("test"),
				functionDepth: 0,
				constants:     []string{},
				b:             bytecode.New("test"),
			},
			wantErr: errors.NewError(errors.ErrMissingEqual),
		},
		{
			name: "invalid constant expression",
			fields: fields{
				t:             tokenizer.New("(foo = bar)", true),
				symbols:       symbols.NewRootSymbolTable("test"),
				functionDepth: 0,
				constants:     []string{},
				b:             bytecode.New("test"),
			},
			wantErr: errors.NewError(errors.ErrInvalidConstant).Context("bar"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Compiler{
				t:             tt.fields.t,
				s:             tt.fields.symbols,
				flags:         tt.fields.flags,
				functionDepth: tt.fields.functionDepth,
				constants:     tt.fields.constants,
				b:             tt.fields.b,
			}
			if err := c.compileConst(); (err == nil) != (tt.wantErr == nil) {
				t.Errorf("Compiler.compileConst() %s, error = %v, wantErr %v", tt.name, err, tt.wantErr)
			} else {
				if err != nil && err.Error() != tt.wantErr.Error() {
					t.Errorf("Compiler.compileConst() %s, error = %v, wantErr %v", tt.name, err, tt.wantErr)
				}
			}
		})
	}
}
