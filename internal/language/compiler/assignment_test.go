package compiler

import (
	"testing"

	"github.com/tucats/ego/internal/language/bytecode"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/symbols"
	"github.com/tucats/ego/internal/language/tokenizer"
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
		// --- BUG-06 regression tests: ++ and -- on qualified lvalues ----------
		//
		// Before the fix, any lvalue with more than 2 store instructions (i.e.
		// an array index or struct field) immediately returned ErrInvalidAuto.
		// These cases must now compile without error.
		{
			// a[0]++ must compile: the lvalue a[0] has a Load+"Push 0"+StoreIndex
			// structure that the new qualified-lvalue path handles.
			name: "array element auto-increment (BUG-06)",
			fields: fields{
				t:             tokenizer.New("a[0]++", true),
				symbols:       symbols.NewRootSymbolTable("test"),
				functionDepth: 0,
				constants:     []string{},
				b:             bytecode.New("test"),
			},
			wantErr: nil,
		},
		{
			// a[0]-- mirrors the ++ case above but uses Sub instead of Add.
			name: "array element auto-decrement (BUG-06)",
			fields: fields{
				t:             tokenizer.New("a[0]--", true),
				symbols:       symbols.NewRootSymbolTable("test"),
				functionDepth: 0,
				constants:     []string{},
				b:             bytecode.New("test"),
			},
			wantErr: nil,
		},
		{
			// s.field++ must compile: the struct field lvalue has a Load+"Push field"+
			// StoreIndex structure that the new qualified-lvalue path handles.
			name: "struct field auto-increment (BUG-06)",
			fields: fields{
				t:             tokenizer.New("s.field++", true),
				symbols:       symbols.NewRootSymbolTable("test"),
				functionDepth: 0,
				constants:     []string{},
				b:             bytecode.New("test"),
			},
			wantErr: nil,
		},
		{
			// s.field-- mirrors the struct ++ case but uses Sub.
			name: "struct field auto-decrement (BUG-06)",
			fields: fields{
				t:             tokenizer.New("s.field--", true),
				symbols:       symbols.NewRootSymbolTable("test"),
				functionDepth: 0,
				constants:     []string{},
				b:             bytecode.New("test"),
			},
			wantErr: nil,
		},
		// --- end BUG-06 regression tests -------------------------------------

		// --- BUG-07 regression tests: two-value channel receive ---------------
		//
		// Before the fix, "v, ok := <-ch" produced "incorrect number of return
		// values" at runtime because the storeLValue's StackCheck 2 found only
		// one item on the stack (the channel object).  These compile-time tests
		// verify that the assignment compiles without error.  Full correctness
		// (the actual values received) is covered by run_test.go.
		{
			// v, ok := <-ch  — two-value receive must compile without error.
			name: "two-value channel receive (BUG-07)",
			fields: fields{
				t:             tokenizer.New("v, ok := <-ch", true),
				symbols:       symbols.NewRootSymbolTable("test"),
				functionDepth: 0,
				constants:     []string{},
				b:             bytecode.New("test"),
			},
			wantErr: nil,
		},
		// --- end BUG-07 regression tests -------------------------------------
		{
			name: "invalid auto-increment",
			fields: fields{
				t:             tokenizer.New("(x = 42; y++)", true),
				symbols:       symbols.NewRootSymbolTable("test"),
				functionDepth: 0,
				constants:     []string{},
				b:             bytecode.New("test"),
			},
			wantErr: errors.New(errors.ErrInvalidSymbolName),
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
			wantErr: errors.New(errors.ErrInvalidSymbolName),
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
			wantErr: errors.New(errors.ErrMissingAssignment),
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
			wantErr: errors.New(errors.ErrMissingExpression),
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
			wantErr: errors.New(errors.ErrMissingAssignment),
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
			wantErr: errors.New(errors.ErrInvalidSymbolName).Context("y"),
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
				if err != nil && !errors.SameBaseError(err, tt.wantErr) {
					t.Errorf("Compiler.compileAssignment() %s, error = %v, wantErr %v", tt.name, err, tt.wantErr)
				}
			}
		})
	}
}
