package compiler

import (
	"strings"
	"testing"

	"github.com/tucats/ego/internal/language/bytecode"
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/language/symbols"
	"github.com/tucats/ego/internal/language/tokenizer"
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
			name: "function literal",
			fields: fields{
				functionDepth: 1,
				t:             tokenizer.New("func(){}()", true),
			},
			wantErr: false,
		},
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
			name: "function call",
			fields: fields{
				functionDepth: 1,
				t:             tokenizer.New("foo()", true),
			},
			wantErr: false,
		},
		{
			// fixed BUG-16 / FLOW-M4 fix: a single-argument named-function defer
			// must still compile cleanly once its argument is hoisted out
			// and captured eagerly.
			name: "function call with one argument",
			fields: fields{
				functionDepth: 1,
				t:             tokenizer.New("foo(42)", true),
			},
			wantErr: false,
		},
		{
			// Every argument in a multi-argument call must be hoisted, not
			// just the first one.
			name: "function call with multiple arguments",
			fields: fields{
				functionDepth: 1,
				t:             tokenizer.New("foo(1, 2, 3)", true),
			},
			wantErr: false,
		},
		{
			// The trailing "..." (variadic spread) form must still compile;
			// the slice itself is what gets hoisted/frozen.
			name: "function call with variadic argument",
			fields: fields{
				functionDepth: 1,
				t:             tokenizer.New("foo(items...)", true),
			},
			wantErr: false,
		},
		{
			// A dotted method call ("obj.Method(arg)") goes through the same
			// hoisting path as a plain function call.
			name: "method call with argument",
			fields: fields{
				functionDepth: 1,
				t:             tokenizer.New("obj.Method(42)", true),
			},
			wantErr: false,
		},
		{
			// A malformed argument expression must still be reported as an
			// error, exactly as it would be for a normal (non-deferred)
			// function call -- the hoisting step must propagate the
			// underlying expression-compile error rather than swallowing it.
			name: "function call with malformed argument expression",
			fields: fields{
				functionDepth: 1,
				t:             tokenizer.New("foo(1+)", true),
			},
			wantErr: true,
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

// TestCompiler_compileDefer_ArgumentsHoistedEagerly is a regression test for
// fixed BUG-16 / FLOW-M4. It doesn't just check that "defer foo(42)" compiles
// without error -- it inspects the actual emitted bytecode shape to prove
// that the argument expression is evaluated and frozen into a temporary
// variable *before* the deferred closure is built, rather than being left
// embedded inside the closure body where it would only run later.
//
// The expected instruction sequence for "defer foo(42)" is:
//
//	0: DeferStart true
//	1: Push 42                 <- the argument is evaluated HERE, eagerly
//	2: StoreAlways "$N"        <- ...and immediately frozen into a temp var
//	3: Push <closure ByteCode> <- the deferred closure is built AFTER that
//	4: Defer 0
//
// and, inside the closure's own bytecode, a Load of that same "$N" temp
// variable (never a Load of "42" or of anything resembling the original
// argument text) -- proving the closure only *reads* the already-computed
// value instead of re-evaluating the argument itself.
func TestCompiler_compileDefer_ArgumentsHoistedEagerly(t *testing.T) {
	c := &Compiler{
		b:             bytecode.New("test"),
		t:             tokenizer.New("foo(42)", true),
		functionDepth: 1,
	}

	if err := c.compileDefer(); err != nil {
		t.Fatalf("Compiler.compileDefer() unexpected error: %v", err)
	}

	ops := c.b.Opcodes()
	if len(ops) != 5 {
		t.Fatalf("expected 5 top-level instructions, got %d: %v", len(ops), ops)
	}

	if ops[0].Operation != bytecode.DeferStart {
		t.Errorf("instruction 0 = %v, want DeferStart", ops[0].Operation)
	}

	if ops[1].Operation != bytecode.Push {
		t.Fatalf("instruction 1 = %v, want Push (the eagerly-evaluated argument)", ops[1].Operation)
	}

	if n, ok := data.UnwrapConstant(ops[1].Operand).(int); !ok || n != 42 {
		t.Errorf("instruction 1 pushed %v, want the literal 42", ops[1].Operand)
	}

	if ops[2].Operation != bytecode.StoreAlways {
		t.Fatalf("instruction 2 = %v, want StoreAlways (freezing the argument into a temp variable)", ops[2].Operation)
	}

	tempName, ok := ops[2].Operand.(string)
	if !ok || !strings.HasPrefix(tempName, "$") {
		t.Fatalf("instruction 2 stored into %q, want a generated \"$N\" temp name", ops[2].Operand)
	}

	if ops[3].Operation != bytecode.Push {
		t.Fatalf("instruction 3 = %v, want Push (the deferred closure)", ops[3].Operation)
	}

	closure, ok := ops[3].Operand.(*bytecode.ByteCode)
	if !ok {
		t.Fatalf("instruction 3 pushed a %T, want *bytecode.ByteCode (the deferred closure)", ops[3].Operand)
	}

	if ops[4].Operation != bytecode.Defer {
		t.Errorf("instruction 4 = %v, want Defer", ops[4].Operation)
	}

	// Confirm the closure body reads the temp variable by name -- this is
	// what makes the fix actually take effect at run time -- and never
	// mentions the literal argument value directly.
	foundLoadOfTemp := false

	for _, inner := range closure.Opcodes() {
		if inner.Operation == bytecode.Load && inner.Operand == tempName {
			foundLoadOfTemp = true
		}

		if n, ok := data.UnwrapConstant(inner.Operand).(int); ok && n == 42 {
			t.Errorf("closure body still contains a literal Push of 42 at %v; the argument was not fully hoisted out", inner)
		}
	}

	if !foundLoadOfTemp {
		t.Errorf("closure body never loads the temp variable %q; expected it to reference the frozen argument", tempName)
	}
}
