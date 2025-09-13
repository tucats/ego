package compiler

import (
	"testing"

	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/tokenizer"
)

func TestCompiler_error(t *testing.T) {
	type fields struct {
		t                 *tokenizer.Tokenizer
		s                 *symbols.SymbolTable
		flags             flagSet
		functionDepth     int
		constants         []string
		b                 *bytecode.ByteCode
		activePackageName string
		sourceFile        string
	}

	type args struct {
		err  error
		args []any
	}

	tests := []struct {
		name   string
		fields fields
		args   args
		want   *errors.Error
	}{
		{
			name: "no token",
			fields: fields{
				t: tokenizer.New("test", true),
			},
			args: args{
				err: errors.Message("test error"),
			},
			want: errors.New(errors.Message("test error")),
		},
		{
			name: "with token",
			fields: fields{
				t: tokenizer.New("test", true),
			},
			args: args{
				err:  errors.Message("test error"),
				args: []any{"test token"},
			},
			want: errors.New(errors.Message("test error")).Context("test token"),
		},
		{
			name: "with package name",
			fields: fields{
				t: tokenizer.New("test", true),
			},
			args: args{
				err:  errors.Message("test error"),
				args: []any{"test token"},
			},
			want: errors.New(errors.Message("test error")).Context("test token"),
		},
		{
			name: "with source file",
			fields: fields{
				t:          tokenizer.New("test", true),
				sourceFile: "test.ego",
			},
			args: args{
				err:  errors.Message("test error"),
				args: []any{"test token"},
			},
			want: errors.New(errors.Message("test error")).Context("test token").In("test.ego"),
		},
		{
			name: "with line and position",
			fields: fields{
				t: tokenizer.New("test", true),
			},
			args: args{
				err:  errors.Message("test error"),
				args: []any{"test token"},
			},
			want: errors.New(errors.Message("test error")).Context("test token"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Compiler{
				t:                 tt.fields.t,
				s:                 tt.fields.s,
				flags:             tt.fields.flags,
				functionDepth:     tt.fields.functionDepth,
				constants:         tt.fields.constants,
				b:                 tt.fields.b,
				activePackageName: tt.fields.activePackageName,
				sourceFile:        tt.fields.sourceFile,
			}
			if got := c.compileError(tt.args.err, tt.args.args...); !errors.Equal(got, tt.want) {
				t.Errorf("Compiler.error() = %v, want %v", got, tt.want)
			}
		})
	}
}
