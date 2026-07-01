package compiler

import (
	"testing"

	"github.com/tucats/ego/internal/language/bytecode"
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/symbols"
	"github.com/tucats/ego/internal/language/tokenizer"
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
			wantErr: errors.New(errors.ErrInvalidSymbolName).Context("1"),
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
			wantErr: errors.New(errors.ErrMissingEqual),
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
			wantErr: errors.New(errors.ErrInvalidConstant).Context("bar"),
		},
		{
			// BUG-20: a bare "iota" reference is legal inside a const(...) block --
			// it should compile like any other constant expression, not be rejected
			// as an unknown symbol.
			name: "iota basic",
			fields: fields{
				t:             tokenizer.New("(foo = iota)", true),
				symbols:       symbols.NewRootSymbolTable("test"),
				functionDepth: 0,
				constants:     []string{},
				b:             bytecode.New("test"),
			},
			wantErr: nil,
		},
		{
			// BUG-20: a ConstSpec may omit "= expr" to repeat the previous spec's
			// expression (here, "= iota"), which is how Go's classic
			// "const ( A = iota; B; C )" idiom is written.
			name: "iota repeat",
			fields: fields{
				t:             tokenizer.New("(foo = iota; bar; baz)", true),
				symbols:       symbols.NewRootSymbolTable("test"),
				functionDepth: 0,
				constants:     []string{},
				b:             bytecode.New("test"),
			},
			wantErr: nil,
		},
		{
			// BUG-20: iota also works in an expression, not just by itself.
			name: "iota in expression",
			fields: fields{
				t:             tokenizer.New("(foo = 1 << iota; bar; baz)", true),
				symbols:       symbols.NewRootSymbolTable("test"),
				functionDepth: 0,
				constants:     []string{},
				b:             bytecode.New("test"),
			},
			wantErr: nil,
		},
		{
			// BUG-20: iota is legal even in the non-parenthesized single-constant
			// form; Go treats it the same as a one-element const block (iota is 0).
			name: "iota standalone",
			fields: fields{
				t:             tokenizer.New("foo = iota", true),
				symbols:       symbols.NewRootSymbolTable("test"),
				functionDepth: 0,
				constants:     []string{},
				b:             bytecode.New("test"),
			},
			wantErr: nil,
		},
		{
			// The repeat-previous-expression shorthand is only valid inside a
			// parenthesized block and only when there is a previous expression to
			// repeat. A single (non-list) constant with no "=" at all is still an
			// error, exactly as before this change.
			name: "missing equal sign, non-list form",
			fields: fields{
				t:             tokenizer.New("foo 42", true),
				symbols:       symbols.NewRootSymbolTable("test"),
				functionDepth: 0,
				constants:     []string{},
				b:             bytecode.New("test"),
			},
			wantErr: errors.New(errors.ErrMissingEqual),
		},
		{
			// The very first spec in a block has nothing to repeat, so omitting
			// "= expr" there is still an error.
			name: "missing equal sign, first spec in block",
			fields: fields{
				t:             tokenizer.New("(foo; bar = 1)", true),
				symbols:       symbols.NewRootSymbolTable("test"),
				functionDepth: 0,
				constants:     []string{},
				b:             bytecode.New("test"),
			},
			wantErr: errors.New(errors.ErrMissingEqual),
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
				if err != nil && !errors.SameBaseError(err, tt.wantErr) {
					t.Errorf("Compiler.compileConst() %s, error = %v, wantErr %v", tt.name, err, tt.wantErr)
				}
			}
		})
	}
}

// pushedIntValues walks the compiled bytecode looking for Push instructions and
// returns their operands, in order. This lets a test check *what value* each
// ConstSpec actually pushed onto the stack (0, 1, 2, ... for iota) rather than
// just whether compilation succeeded.
func pushedIntValues(t *testing.T, b *bytecode.ByteCode) []int {
	t.Helper()

	var values []int

	for _, i := range b.Opcodes() {
		if i.Operation == bytecode.Push {
			// Integer literals (including the iota counter) are wrapped in a
			// data.Immutable marker by data.Constant() before being pushed, so we
			// have to unwrap that marker to get back to the plain int.
			if n, ok := data.UnwrapConstant(i.Operand).(int); ok {
				values = append(values, n)
			}
		}
	}

	return values
}

// TestCompiler_compileConst_IotaValues verifies BUG-20 end-to-end at the
// bytecode level: it checks that "iota" is actually assigned the successive
// integer values 0, 1, 2, ... (not just that compilation succeeds), for both
// the explicit form (every spec spells out "= iota") and the Go idiom where
// later specs omit "= expr" and repeat the first spec's expression.
func TestCompiler_compileConst_IotaValues(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   []int
	}{
		{
			name:   "explicit iota on every spec",
			source: "(a = iota; b = iota; c = iota)",
			want:   []int{0, 1, 2},
		},
		{
			name:   "classic repeat idiom",
			source: "(a = iota; b; c)",
			want:   []int{0, 1, 2},
		},
		{
			name:   "iota inside a larger expression, repeated",
			source: "(a = 1 << iota; b; c)",
			// Each spec repeats the whole "1 << iota" expression, so every Push
			// sequence includes both the literal "1" and the current iota value:
			// a -> Push 1, Push 0 (then shift); b -> Push 1, Push 1; c -> Push 1, Push 2.
			want: []int{1, 0, 1, 1, 1, 2},
		},
		{
			name:   "iota does not have to be the first spec's expression",
			source: "(a = 42; b = iota; c)",
			// "a" is a plain literal (no iota reference, so no Push for its value
			// shows up as an "iota position" -- but it still emits its own Push 42).
			// "b" explicitly uses iota at position 1. "c" repeats "= iota" and is at
			// position 2.
			want: []int{42, 1, 2},
		},
		{
			name:   "standalone (non-block) const with iota",
			source: "single = iota",
			want:   []int{0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Compiler{
				t:         tokenizer.New(tt.source, true),
				s:         symbols.NewRootSymbolTable("test"),
				constants: []string{},
				b:         bytecode.New("test"),
			}

			if err := c.compileConst(); err != nil {
				t.Fatalf("Compiler.compileConst() unexpected error: %v", err)
			}

			got := pushedIntValues(t, c.b)
			if len(got) != len(tt.want) {
				t.Fatalf("pushed values = %v, want %v", got, tt.want)
			}

			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Errorf("pushed value[%d] = %d, want %d (all values: got=%v want=%v)", i, got[i], tt.want[i], got, tt.want)
				}
			}
		})
	}
}

// TestCompiler_compileConst_IotaScopeRestored verifies that c.iota is restored
// to its "not in a const block" sentinel value (-1) once compileConst() returns,
// so that a bare "iota" identifier appearing later in the same compilation --
// outside of any const declaration -- is not mistakenly treated as the special
// counter. This matches Go, where referencing iota outside a const declaration
// is a compile error ("undefined: iota").
func TestCompiler_compileConst_IotaScopeRestored(t *testing.T) {
	c := &Compiler{
		t:         tokenizer.New("(a = iota; b)", true),
		s:         symbols.NewRootSymbolTable("test"),
		constants: []string{},
		b:         bytecode.New("test"),
		iota:      -1,
	}

	if err := c.compileConst(); err != nil {
		t.Fatalf("Compiler.compileConst() unexpected error: %v", err)
	}

	if c.iota != -1 {
		t.Errorf("c.iota after compileConst() = %d, want -1 (sentinel for \"not in a const block\")", c.iota)
	}
}
