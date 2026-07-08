package compiler

import (
	"testing"

	"github.com/tucats/ego/internal/language/bytecode"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/symbols"
	"github.com/tucats/ego/internal/language/tokenizer"
)

// TestCompiler_compileThrow exercises compileThrow directly, mirroring the
// pattern used by TestCompiler_compileExit (exit_test.go). The extension
// gate itself (c.flags.extensionsEnabled) is checked by the caller in
// statement.go, not by compileThrow -- see
// TestCompileThrow_RequiresExtensions below for that behavior.
func TestCompiler_compileThrow(t *testing.T) {
	tests := []struct {
		name    string
		source  string
		wantErr bool
	}{
		{
			name:    "throw with an error-returning expression",
			source:  "throw myErr",
			wantErr: false,
		},
		{
			name:    "throw with no expression",
			source:  "throw",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Compiler{
				t: tokenizer.New(tt.source, true),
				s: symbols.NewRootSymbolTable("test"),
				flags: flagSet{
					extensionsEnabled: true,
				},
				functionDepth: 1,
				constants:     []string{},
				b:             bytecode.New("test"),
			}

			// The statement handler will have eaten the "throw" token, so
			// simulate that here, matching compileExit's test setup.
			c.t.IsNext(tokenizer.ThrowToken)
			c.DefineGlobalSymbol("myErr")

			err := c.compileThrow()
			if (err != nil) != tt.wantErr {
				t.Errorf("compileThrow() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestCompileThrow_RequiresExtensions verifies the extension gate in
// statement.go: "throw" is rejected as an unrecognized statement when
// language extensions are disabled, and accepted when they are enabled --
// exactly like "try"/"catch".
func TestCompileThrow_RequiresExtensions(t *testing.T) {
	const source = `
		package main
		func main() {
			var e error
			throw e
		}
	`

	t.Run("extensions disabled", func(t *testing.T) {
		c := New("test").SetExtensionsEnabled(false)
		defer c.Close()

		_, err := c.Compile("test", tokenizer.New(source, true))
		if err == nil {
			t.Error("expected an error compiling \"throw\" with extensions disabled, got nil")
		}

		if !errors.Equals(err, errors.ErrUnrecognizedStatement) {
			t.Errorf("expected ErrUnrecognizedStatement, got %v", err)
		}
	})

	t.Run("extensions enabled", func(t *testing.T) {
		c := New("test").SetExtensionsEnabled(true)
		defer c.Close()

		_, err := c.Compile("test", tokenizer.New(source, true))
		if err != nil {
			t.Errorf("unexpected error compiling \"throw\" with extensions enabled: %v", err)
		}
	})
}
