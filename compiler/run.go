package compiler

import (
	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/tokenizer"
)

// Given a string, compile and execute it immediately.
func RunString(name string, s *symbols.SymbolTable, programText string) error {
	return Run(name, s, tokenizer.New(programText, true))
}

// Given a token stream, compile and execute it immediately. Note that language
// extensions are always enabled for this kind of execution mode.
func Run(name string, s *symbols.SymbolTable, t *tokenizer.Tokenizer) error {
	c := New(name).ExtensionsEnabled(true)

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
		err = bytecode.NewContext(s, bc).Run()
	}

	return err
}
