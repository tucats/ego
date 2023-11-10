package compiler

import (
	"testing"

	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/tokenizer"
)

func TestCompiler_compileExit(t *testing.T) {
	type fields struct {
		t             *tokenizer.Tokenizer
		symbols       *symbols.SymbolTable
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
			name: "exit with no argument",
			fields: fields{
				t:             tokenizer.New("exit", true),
				symbols:       symbols.NewRootSymbolTable("test"),
				functionDepth: 1,
				constants:     []string{},
				b:             bytecode.New("test"),
			},
			wantErr: nil,
		},
		{
			name: "exit with integer argument",
			fields: fields{
				t:             tokenizer.New("exit 42", true),
				symbols:       symbols.NewRootSymbolTable("test"),
				functionDepth: 0,
				constants:     []string{},
				b:             bytecode.New("test"),
			},
			wantErr: nil,
		},
		{
			name: "exit with expression argument",
			fields: fields{
				t:             tokenizer.New("exit 2 + 2", true),
				symbols:       symbols.NewRootSymbolTable("test"),
				functionDepth: 0,
				constants:     []string{},
				b:             bytecode.New("test"),
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Compiler{
				t: tt.fields.t,
				s: tt.fields.symbols,
				flags: flagSet{
					extensionsEnabled: true,
					exitEnabled:       true,
				},
				functionDepth: tt.fields.functionDepth,
				constants:     tt.fields.constants,
				b:             tt.fields.b,
			}

			// The statement handler will have eaten the "exit" token, so
			// simulate that here.
			c.t.IsNext(tokenizer.ExitToken)

			if err := c.compileExit(); (err == nil) != (tt.wantErr == nil) {
				t.Errorf("Compiler.compileExit() %s, error = %v, wantErr %v", tt.name, err, tt.wantErr)
			} else {
				if err != nil && err.Error() != tt.wantErr.Error() {
					t.Errorf("Compiler.compileExit() %s, error = %v, wantErr %v", tt.name, err, tt.wantErr)
				}
			}
		})
	}
}
