package compiler

import (
	"github.com/tucats/ego/internal/cli/settings"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/language/bytecode"
	"github.com/tucats/ego/internal/language/symbols"
	"github.com/tucats/ego/internal/language/tokenizer"
)

// RunString is a convenience wrapper that tokenizes programText and then calls
// Run to compile and execute it immediately in the given symbol table.
func RunString(name string, s *symbols.SymbolTable, programText string) error {
	return Run(name, s, tokenizer.New(programText, true))
}

// CompileString tokenizes programText and compiles it, returning the resulting
// bytecode without executing it. The function depth is pre-set to 1 so that
// top-level statements are accepted without requiring a function body wrapper.
// This is the compile-only counterpart of RunString, intended for callers that
// need to run the bytecode in a custom way (e.g. under the interactive
// debugger).
//
// The extensions parameter selects whether language extensions (try/catch,
// print, throw, ?:, etc.) are permitted in the compiled unit. It is set
// directly on this compiler instance and inherited by any import sub-compiler
// (see compileImport). Earlier versions instead mutated the process-global
// ExtensionsEnabledSetting for the duration of the call so that nested
// import-time New() calls would observe it; that leaked the flag across every
// other goroutine compiling concurrently (a real cross-session hazard for the
// dashboard, which compiles user code per request -- CODE-M5) and forced
// extensions on regardless of the caller's intent. Passing the value
// explicitly keeps it local to this compilation and its imports.
func CompileString(name string, programText string, extensions bool) (*bytecode.ByteCode, error) {
	c := New(name).SetExtensionsEnabled(extensions)
	c.functionDepth = 1
	c.flags.fragment = true

	t := tokenizer.New(programText, true)

	bc, err := c.Compile(name, t)
	if err != nil {
		return nil, err
	}

	c.Close()

	return bc, nil
}

// Run compiles the token stream t and immediately executes the resulting
// bytecode in the supplied symbol table. Language extensions are always
// enabled for this mode (used for interactive/REPL sessions where the full
// extension set is expected).
//
// functionDepth is pre-set to 1 so that the compiler accepts top-level
// statements without requiring them to be wrapped in a function body.
func Run(name string, s *symbols.SymbolTable, t *tokenizer.Tokenizer) error {
	c := New(name).SetExtensionsEnabled(true)

	oldState := defs.True
	if !settings.GetBool(defs.ExtensionsEnabledSetting) {
		oldState = defs.False
	}

	defer settings.SetDefault(defs.ExtensionsEnabledSetting, oldState)

	settings.SetDefault(defs.ExtensionsEnabledSetting, defs.True)

	// Set the depth >0 so we will process all statements without requiring a function
	// body.
	c.functionDepth = 1
	c.flags.fragment = true

	bc, err := c.Compile(name, t)
	if err == nil {
		c.Close()

		err = bytecode.NewContext(s, bc).Run()
	}

	return err
}
