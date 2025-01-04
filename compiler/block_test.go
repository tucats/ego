package compiler

import (
	"testing"

	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/tokenizer"
)

func TestCompiler_compileBlock(t *testing.T) {
	type fields struct {
		t             *tokenizer.Tokenizer
		symbols       *symbols.SymbolTable
		flags         flagSet
		functionDepth int
		constants     []string
		b             *bytecode.ByteCode
		blockDepth    int
	}

	tests := []struct {
		name    string
		fields  fields
		wantErr *errors.Error
	}{
		{
			name: "empty block",
			fields: fields{
				t:             tokenizer.New(" }", true),
				symbols:       symbols.NewRootSymbolTable("test"),
				functionDepth: 1,
				constants:     []string{},
				b:             bytecode.New("test"),
				blockDepth:    0,
			},
			wantErr: nil,
		},
		{
			name: "single statement block",
			fields: fields{
				t:             tokenizer.New("foo = 42}", true),
				symbols:       symbols.NewRootSymbolTable("test"),
				functionDepth: 1,
				constants:     []string{},
				b:             bytecode.New("test"),
				blockDepth:    0,
			},
			wantErr: nil,
		},
		{
			name: "multiple statement block",
			fields: fields{
				t:             tokenizer.New("foo = 42; bar = 3.14}", true),
				symbols:       symbols.NewRootSymbolTable("test"),
				functionDepth: 1,
				constants:     []string{},
				b:             bytecode.New("test"),
				blockDepth:    0,
			},
			wantErr: nil,
		},
		{
			name: "missing end of block",
			fields: fields{
				t:             tokenizer.New("foo = 42", true),
				symbols:       symbols.NewRootSymbolTable("test"),
				functionDepth: 1,
				constants:     []string{},
				b:             bytecode.New("test"),
				blockDepth:    0,
			},
			wantErr: errors.New(errors.ErrMissingEndOfBlock),
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
				blockDepth:    tt.fields.blockDepth,
			}
			if err := c.compileBlock(); (err == nil) != (tt.wantErr == nil) {
				t.Errorf("Compiler.compileBlock() %s, error = %v, wantErr %v", tt.name, err, tt.wantErr)
			} else {
				if err != nil && !errors.SameBaseError(err, tt.wantErr) {
					t.Errorf("Compiler.compileBlock() %s, error = %v, wantErr %v", tt.name, err, tt.wantErr)
				}
			}
		})
	}
}
