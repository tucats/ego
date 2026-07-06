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
//	2: ValueCopy               <- BUG-43 fix: struct-value-copy semantics
//	3: StoreAlways "$N"        <- ...and immediately frozen into a temp var
//	4: Push <closure ByteCode> <- the deferred closure is built AFTER that
//	5: Defer 0
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
	if len(ops) != 6 {
		t.Fatalf("expected 6 top-level instructions, got %d: %v", len(ops), ops)
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

	if ops[2].Operation != bytecode.ValueCopy {
		t.Fatalf("instruction 2 = %v, want ValueCopy (BUG-43 fix: struct-value-copy semantics before freezing)", ops[2].Operation)
	}

	if ops[3].Operation != bytecode.StoreAlways {
		t.Fatalf("instruction 3 = %v, want StoreAlways (freezing the argument into a temp variable)", ops[3].Operation)
	}

	tempName, ok := ops[3].Operand.(string)
	if !ok || !strings.HasPrefix(tempName, "$") {
		t.Fatalf("instruction 3 stored into %q, want a generated \"$N\" temp name", ops[3].Operand)
	}

	if ops[4].Operation != bytecode.Push {
		t.Fatalf("instruction 4 = %v, want Push (the deferred closure)", ops[4].Operation)
	}

	closure, ok := ops[4].Operand.(*bytecode.ByteCode)
	if !ok {
		t.Fatalf("instruction 4 pushed a %T, want *bytecode.ByteCode (the deferred closure)", ops[4].Operand)
	}

	if ops[5].Operation != bytecode.Defer {
		t.Errorf("instruction 5 = %v, want Defer", ops[5].Operation)
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

// TestCompiler_compileDefer_ReceiverHoistedEagerly is a regression test for
// BUG-43. Like TestCompiler_compileDefer_ArgumentsHoistedEagerly above, it
// inspects the emitted top-level bytecode shape for "defer l.Log(42)" to
// prove that the RECEIVER ("l") is, like the argument, evaluated and frozen
// into its own temp variable before the deferred closure is built -- rather
// than being left embedded in the closure body, where it would be
// re-resolved from the live symbol table when the deferred call finally
// runs (the bug: mutating "l" between the "defer" statement and the
// function returning would then incorrectly change which receiver value
// the deferred call observes).
//
// The expected instruction sequence for "defer l.Log(42)" is:
//
//	0: DeferStart true
//	1: Push 42                 <- the argument, evaluated eagerly (BUG-16/FLOW-M4 fix)
//	2: ValueCopy               <- BUG-43 fix: struct-value-copy semantics
//	3: StoreAlways "$N"        <- ...and frozen into a temp var
//	4: Load "l"                <- the RECEIVER, evaluated eagerly (BUG-43 fix)
//	5: ValueCopy               <- BUG-43 fix: struct-value-copy semantics
//	6: StoreAlways "$M"        <- ...and frozen into its OWN temp var
//	7: Push <closure ByteCode> <- the deferred closure is built AFTER both of the above
//	8: Defer 0
//
// and, inside the closure's own bytecode, a Load of both temp variables
// ("$N" and "$M"), and never a Load of "l" itself -- proving the closure
// only reads the already-computed receiver value instead of re-resolving
// "l" from the live symbol table.
func TestCompiler_compileDefer_ReceiverHoistedEagerly(t *testing.T) {
	c := &Compiler{
		b:             bytecode.New("test"),
		t:             tokenizer.New("l.Log(42)", true),
		functionDepth: 1,
	}

	if err := c.compileDefer(); err != nil {
		t.Fatalf("Compiler.compileDefer() unexpected error: %v", err)
	}

	ops := c.b.Opcodes()
	if len(ops) != 9 {
		t.Fatalf("expected 9 top-level instructions, got %d: %v", len(ops), ops)
	}

	if ops[0].Operation != bytecode.DeferStart {
		t.Errorf("instruction 0 = %v, want DeferStart", ops[0].Operation)
	}

	if ops[1].Operation != bytecode.Push {
		t.Fatalf("instruction 1 = %v, want Push (the eagerly-evaluated argument)", ops[1].Operation)
	}

	if ops[2].Operation != bytecode.ValueCopy {
		t.Fatalf("instruction 2 = %v, want ValueCopy", ops[2].Operation)
	}

	if ops[3].Operation != bytecode.StoreAlways {
		t.Fatalf("instruction 3 = %v, want StoreAlways (freezing the argument)", ops[3].Operation)
	}

	argTempName, ok := ops[3].Operand.(string)
	if !ok || !strings.HasPrefix(argTempName, "$") {
		t.Fatalf("instruction 3 stored into %q, want a generated \"$N\" temp name", ops[3].Operand)
	}

	if ops[4].Operation != bytecode.Load || ops[4].Operand != "l" {
		t.Fatalf("instruction 4 = %v %v, want Load \"l\" (the eagerly-evaluated receiver)", ops[4].Operation, ops[4].Operand)
	}

	if ops[5].Operation != bytecode.ValueCopy {
		t.Fatalf("instruction 5 = %v, want ValueCopy", ops[5].Operation)
	}

	if ops[6].Operation != bytecode.StoreAlways {
		t.Fatalf("instruction 6 = %v, want StoreAlways (freezing the receiver)", ops[6].Operation)
	}

	receiverTempName, ok := ops[6].Operand.(string)
	if !ok || !strings.HasPrefix(receiverTempName, "$") {
		t.Fatalf("instruction 6 stored into %q, want a generated \"$N\" temp name", ops[6].Operand)
	}

	if receiverTempName == argTempName {
		t.Fatalf("receiver and argument were hoisted into the SAME temp name %q; they must be independent", receiverTempName)
	}

	if ops[7].Operation != bytecode.Push {
		t.Fatalf("instruction 7 = %v, want Push (the deferred closure)", ops[7].Operation)
	}

	closure, ok := ops[7].Operand.(*bytecode.ByteCode)
	if !ok {
		t.Fatalf("instruction 7 pushed a %T, want *bytecode.ByteCode (the deferred closure)", ops[7].Operand)
	}

	if ops[8].Operation != bytecode.Defer {
		t.Errorf("instruction 8 = %v, want Defer", ops[8].Operation)
	}

	// Confirm the closure body reads BOTH temp variables by name, and never
	// re-resolves the original receiver identifier "l".
	foundLoadOfArgTemp := false
	foundLoadOfReceiverTemp := false

	for _, inner := range closure.Opcodes() {
		if inner.Operation == bytecode.Load {
			switch inner.Operand {
			case argTempName:
				foundLoadOfArgTemp = true
			case receiverTempName:
				foundLoadOfReceiverTemp = true
			case "l":
				t.Errorf("closure body still contains a Load of \"l\" directly; the receiver was not fully hoisted out")
			}
		}
	}

	if !foundLoadOfArgTemp {
		t.Errorf("closure body never loads the argument temp variable %q", argTempName)
	}

	if !foundLoadOfReceiverTemp {
		t.Errorf("closure body never loads the receiver temp variable %q", receiverTempName)
	}
}

// TestCompiler_hoistDeferReceiver_BareFunctionCallIsNoOp verifies that a bare
// "defer namedFunc(args)" call (no dotted receiver at all) is left completely
// untouched by hoistDeferReceiver -- there is no receiver to freeze, and this
// case is FLOW-M4's own, distinct, still-open gap, not BUG-43's.
func TestCompiler_hoistDeferReceiver_BareFunctionCallIsNoOp(t *testing.T) {
	c := &Compiler{
		b:             bytecode.New("test"),
		t:             tokenizer.New("namedFunc(42)", true),
		functionDepth: 1,
	}

	if err := c.compileDefer(); err != nil {
		t.Fatalf("Compiler.compileDefer() unexpected error: %v", err)
	}

	ops := c.b.Opcodes()

	// Exactly one ValueCopy (for the argument) -- none for a nonexistent
	// receiver.
	valueCopyCount := 0

	for _, op := range ops {
		if op.Operation == bytecode.ValueCopy {
			valueCopyCount++
		}
	}

	if valueCopyCount != 1 {
		t.Errorf("expected exactly 1 ValueCopy instruction (for the argument only), got %d: %v", valueCopyCount, ops)
	}
}

// TestCompiler_hoistDeferReceiver_MultiLevelChain verifies that a multi-level
// dotted receiver chain ("wg.mu.Lock()") hoists the ENTIRE chain up to (but
// not including) the final method name -- not just the first identifier --
// so that "wg.mu" as a whole is frozen, matching Go's defer semantics for a
// nested field selector receiver.
func TestCompiler_hoistDeferReceiver_MultiLevelChain(t *testing.T) {
	c := &Compiler{
		b:             bytecode.New("test"),
		t:             tokenizer.New("wg.mu.Lock()", true),
		functionDepth: 1,
	}

	if err := c.compileDefer(); err != nil {
		t.Fatalf("Compiler.compileDefer() unexpected error: %v", err)
	}

	ops := c.b.Opcodes()

	// Expect: DeferStart, Load "wg", Member "mu", ValueCopy, StoreAlways "$N",
	// Push <closure>, Defer -- i.e. the receiver-chain compile (Load+Member)
	// happens in the top-level stream, followed immediately by ValueCopy and
	// StoreAlways.
	foundMember := false
	foundValueCopyAfterMember := false

	for i, op := range ops {
		if op.Operation == bytecode.Member && op.Operand == "mu" {
			foundMember = true

			if i+1 < len(ops) && ops[i+1].Operation == bytecode.ValueCopy {
				foundValueCopyAfterMember = true
			}
		}
	}

	if !foundMember {
		t.Fatalf("expected a top-level Member \"mu\" instruction (compiling the \"wg.mu\" receiver chain), got: %v", ops)
	}

	if !foundValueCopyAfterMember {
		t.Errorf("expected ValueCopy immediately after the \"wg.mu\" receiver chain compile, got: %v", ops)
	}
}
