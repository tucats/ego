package compiler

import (
	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/tokenizer"
)

// RunString is a convenience wrapper that tokenizes programText and then calls
// Run to compile and execute it immediately in the given symbol table.
func RunString(name string, s *symbols.SymbolTable, programText string) error {
	return Run(name, s, tokenizer.New(programText, true))
}

// CompileString tokenizes programText and compiles it, returning the resulting
// bytecode without executing it. Extensions are enabled and the function depth
// is pre-set to 1 so that top-level statements are accepted without requiring
// a function body wrapper. This is the compile-only counterpart of RunString,
// intended for callers that need to run the bytecode in a custom way (e.g.
// under the interactive debugger).
func CompileString(name string, programText string) (*bytecode.ByteCode, error) {
	oldState := defs.True
	if !settings.GetBool(defs.ExtensionsEnabledSetting) {
		oldState = defs.False
	}

	defer settings.SetDefault(defs.ExtensionsEnabledSetting, oldState)

	settings.SetDefault(defs.ExtensionsEnabledSetting, defs.True)

	c := New(name).SetExtensionsEnabled(true)
	c.functionDepth = 1

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

	bc, err := c.Compile(name, t)
	if err == nil {
		c.Close()

		err = bytecode.NewContext(s, bc).Run()
	}

	return err
}
