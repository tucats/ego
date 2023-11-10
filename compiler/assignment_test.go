package compiler

import (
	"testing"

	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/tokenizer"
)

func TestCompiler_compileAssignment(t *testing.T) {
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
			name: "simple assignment",
			fields: fields{
				t:             tokenizer.New("x = 42", true),
				symbols:       symbols.NewRootSymbolTable("test"),
				functionDepth: 0,
				constants:     []string{},
				b:             bytecode.New("test"),
			},
			wantErr: nil,
		},
		{
			name: "auto-increment",
			fields: fields{
				t:             tokenizer.New("x++", true),
				symbols:       symbols.NewRootSymbolTable("test"),
				functionDepth: 0,
				constants:     []string{},
				b:             bytecode.New("test"),
			},
			wantErr: nil,
		},
		{
			name: "auto-decrement",
			fields: fields{
				t:             tokenizer.New("x--", true),
				symbols:       symbols.NewRootSymbolTable("test"),
				functionDepth: 0,
				constants:     []string{},
				b:             bytecode.New("test"),
			},
			wantErr: nil,
		},
		{
			name: "invalid auto-increment",
			fields: fields{
				t:             tokenizer.New("(x = 42; y++)", true),
				symbols:       symbols.NewRootSymbolTable("test"),
				functionDepth: 0,
				constants:     []string{},
				b:             bytecode.New("test"),
			},
			wantErr: errors.NewError(errors.ErrInvalidSymbolName),
		},
		{
			name: "invalid auto-decrement",
			fields: fields{
				t:             tokenizer.New("(x = 42; y--)", true),
				symbols:       symbols.NewRootSymbolTable("test"),
				functionDepth: 0,
				constants:     []string{},
				b:             bytecode.New("test"),
			},
			wantErr: errors.NewError(errors.ErrInvalidSymbolName),
		},
		{
			name: "missing assignment operator",
			fields: fields{
				t:             tokenizer.New("x 42", true),
				symbols:       symbols.NewRootSymbolTable("test"),
				functionDepth: 0,
				constants:     []string{},
				b:             bytecode.New("test"),
			},
			wantErr: errors.NewError(errors.ErrMissingAssignment),
		},
		{
			name: "missing expression",
			fields: fields{
				t:             tokenizer.New("x =", true),
				symbols:       symbols.NewRootSymbolTable("test"),
				functionDepth: 0,
				constants:     []string{},
				b:             bytecode.New("test"),
			},
			wantErr: errors.NewError(errors.ErrMissingExpression),
		},
		{
			name: "addition assignment",
			fields: fields{
				t:             tokenizer.New("x += 42", true),
				symbols:       symbols.NewRootSymbolTable("test"),
				functionDepth: 0,
				constants:     []string{},
				b:             bytecode.New("test"),
			},
			wantErr: nil,
		},
		{
			name: "subtraction assignment",
			fields: fields{
				t:             tokenizer.New("x -= 42", true),
				symbols:       symbols.NewRootSymbolTable("test"),
				functionDepth: 0,
				constants:     []string{},
				b:             bytecode.New("test"),
			},
			wantErr: nil,
		},
		{
			name: "multiplication assignment",
			fields: fields{
				t:             tokenizer.New("x *= 42", true),
				symbols:       symbols.NewRootSymbolTable("test"),
				functionDepth: 0,
				constants:     []string{},
				b:             bytecode.New("test"),
			},
			wantErr: nil,
		},
		{
			name: "division assignment",
			fields: fields{
				t:             tokenizer.New("x /= 42", true),
				symbols:       symbols.NewRootSymbolTable("test"),
				functionDepth: 0,
				constants:     []string{},
				b:             bytecode.New("test"),
			},
			wantErr: nil,
		},
		{
			name: "invalid assignment operator",
			fields: fields{
				t:             tokenizer.New("x % 42", true),
				symbols:       symbols.NewRootSymbolTable("test"),
				functionDepth: 0,
				constants:     []string{},
				b:             bytecode.New("test"),
			},
			wantErr: errors.NewError(errors.ErrMissingAssignment),
		},
		{
			name: "invalid constant expression",
			fields: fields{
				t:             tokenizer.New("(x = y)", true),
				symbols:       symbols.NewRootSymbolTable("test"),
				functionDepth: 0,
				constants:     []string{},
				b:             bytecode.New("test"),
			},
			wantErr: errors.NewError(errors.ErrInvalidSymbolName).Context("y"),
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
			if err := c.compileAssignment(); (err == nil) != (tt.wantErr == nil) {
				t.Errorf("Compiler.compileAssignment() %s, error = %v, wantErr %v", tt.name, err, tt.wantErr)
			} else {
				if err != nil && err.Error() != tt.wantErr.Error() {
					t.Errorf("Compiler.compileAssignment() %s, error = %v, wantErr %v", tt.name, err, tt.wantErr)
				}
			}
		})
	}
}
