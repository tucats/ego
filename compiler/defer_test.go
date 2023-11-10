package compiler

import (
	"testing"

	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/tokenizer"
)

func TestCompiler_compileDefer(t *testing.T) {
	type fields struct {
		t             *tokenizer.Tokenizer
		symbols       *symbols.SymbolTable
		flags         flagSet
		functionDepth int
	}

	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		{
			name: "defer outside function",
			fields: fields{
				functionDepth: 0,
			},
			wantErr: true,
		},
		{
			name: "missing function",
			fields: fields{
				functionDepth: 1,
				t:             tokenizer.New("", true),
			},
			wantErr: true,
		},
		{
			name: "invalid function call",
			fields: fields{
				functionDepth: 1,
				t:             tokenizer.New("foo", true),
			},
			wantErr: true,
		},
		{
			name: "function literal",
			fields: fields{
				functionDepth: 1,
				t:             tokenizer.New("func(){}", true),
			},
			wantErr: false,
		},
		{
			name: "function call",
			fields: fields{
				functionDepth: 1,
				t:             tokenizer.New("foo()", true),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Compiler{
				b:             bytecode.New("test"),
				t:             tt.fields.t,
				s:             tt.fields.symbols,
				flags:         tt.fields.flags,
				functionDepth: tt.fields.functionDepth,
			}
			if err := c.compileDefer(); (err != nil) != tt.wantErr {
				t.Errorf("Compiler.compileDefer() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
