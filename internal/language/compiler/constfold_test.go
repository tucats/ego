package compiler

// Unit tests for PERFORMANCE.md Finding 17's const-folding tier (see
// docs/internals/GLOBALS.md): emitLoadName's new fold-check branch, and the
// shadowing-hazard guard (c.nonConstLocalNames) that keeps it safe.

import (
	"testing"

	"github.com/tucats/ego/internal/language/bytecode"
	"github.com/tucats/ego/internal/language/tokenizer"
)

// lastEmittedOp returns the operation and operand of the last instruction
// emitted into b, so a test can assert whether emitLoadName chose Push (a
// successful fold), LoadRegister (a register hit), or Load (the unchanged
// name-based fallback).
func lastEmittedOp(t *testing.T, b *bytecode.ByteCode) (bytecode.Opcode, any) {
	t.Helper()

	ops := b.Opcodes()
	if len(ops) == 0 {
		t.Fatal("no instructions emitted")
	}

	last := ops[len(ops)-1]

	return last.Operation, last.Operand
}

func TestEmitLoadName_ConstFold(t *testing.T) {
	tests := []struct {
		name               string
		constFoldEnabled   bool
		constantValues     map[string]any
		nonConstLocalNames map[string]bool
		loadName           string
		wantOp             bytecode.Opcode
		wantOperand        any
	}{
		{
			name:             "folds a known constant to a literal Push",
			constFoldEnabled: true,
			constantValues:   map[string]any{"MaxIter": 42},
			loadName:         "MaxIter",
			wantOp:           bytecode.Push,
			wantOperand:      42,
		},
		{
			name:               "does not fold when the name was locally shadowed",
			constFoldEnabled:   true,
			constantValues:     map[string]any{"MaxIter": 42},
			nonConstLocalNames: map[string]bool{"MaxIter": true},
			loadName:           "MaxIter",
			wantOp:             bytecode.Load,
			wantOperand:        "MaxIter",
		},
		{
			name:             "does not fold when const-folding is disabled",
			constFoldEnabled: false,
			constantValues:   map[string]any{"MaxIter": 42},
			loadName:         "MaxIter",
			wantOp:           bytecode.Load,
			wantOperand:      "MaxIter",
		},
		{
			name:             "falls back to Load for a name with no folded value",
			constFoldEnabled: true,
			constantValues:   map[string]any{},
			loadName:         "someVar",
			wantOp:           bytecode.Load,
			wantOperand:      "someVar",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Compiler{
				constantValues:     tt.constantValues,
				nonConstLocalNames: tt.nonConstLocalNames,
			}
			c.flags.constFold = tt.constFoldEnabled

			b := bytecode.New("test")
			c.emitLoadName(b, tt.loadName)

			gotOp, gotOperand := lastEmittedOp(t, b)
			if gotOp != tt.wantOp {
				t.Errorf("emitLoadName() operation = %v, want %v", gotOp, tt.wantOp)
			}

			if gotOperand != tt.wantOperand {
				t.Errorf("emitLoadName() operand = %v, want %v", gotOperand, tt.wantOperand)
			}
		})
	}
}

// TestCompileConst_FoldsSimpleValues verifies that compileConst populates
// c.constantValues with the correct folded value for a variety of constant
// shapes, including a same-block const-referencing-const chain.
func TestCompileConst_FoldsSimpleValues(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   map[string]any
	}{
		{
			name:   "single integer constant",
			source: "foo = 42",
			want:   map[string]any{"foo": 42},
		},
		{
			name:   "string constant",
			source: `foo = "hello"`,
			want:   map[string]any{"foo": "hello"},
		},
		{
			name:   "same-block const-referencing-const chain",
			source: "(a = 1; b = a + 1; c = b + 1)",
			want:   map[string]any{"a": 1, "b": 2, "c": 3},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := New("test")
			c.t = tokenizer.New(tt.source, true)

			if err := c.compileConst(); err != nil {
				t.Fatalf("compileConst() unexpected error: %v", err)
			}

			for name, want := range tt.want {
				got, ok := c.constantValues[name]
				if !ok {
					t.Errorf("constantValues[%q] not folded, want %v", name, want)

					continue
				}

				if got != want {
					t.Errorf("constantValues[%q] = %v, want %v", name, got, want)
				}
			}
		})
	}
}

// TestCompileConst_ShadowingEndToEnd verifies the full pipeline: a package-level
// const is folded, but once a name-based local declaration of the same name is
// compiled (recorded into c.nonConstLocalNames at the sites in lvalue.go/var.go/
// function.go), a later reference to that name via emitLoadName correctly falls
// back to Load instead of the (now unsafe) folded literal.
func TestCompileConst_ShadowingEndToEnd(t *testing.T) {
	c := New("test")
	c.t = tokenizer.New("MaxIter = 5", true)

	if err := c.compileConst(); err != nil {
		t.Fatalf("compileConst() unexpected error: %v", err)
	}

	if _, ok := c.constantValues["MaxIter"]; !ok {
		t.Fatal("constantValues[\"MaxIter\"] not folded before shadowing")
	}

	// Before any shadowing local is declared, a reference still folds.
	b := bytecode.New("test")
	c.emitLoadName(b, "MaxIter")

	if op, _ := lastEmittedOp(t, b); op != bytecode.Push {
		t.Errorf("reference before shadowing: operation = %v, want Push", op)
	}

	// Simulate what lvalue.go/var.go/function.go do when a name-based (non-
	// register) local declaration of the same name is compiled.
	c.nonConstLocalNames["MaxIter"] = true

	// After shadowing, the SAME name must fall back to Load everywhere in this
	// compilation unit -- the coarse, whole-unit exclusion documented in
	// docs/internals/GLOBALS.md Section 5.3.
	b2 := bytecode.New("test")
	c.emitLoadName(b2, "MaxIter")

	if op, operand := lastEmittedOp(t, b2); op != bytecode.Load || operand != "MaxIter" {
		t.Errorf("reference after shadowing: operation = %v, operand = %v, want Load \"MaxIter\"", op, operand)
	}
}
