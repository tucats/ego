package compiler

import (
	"github.com/tucats/ego/app-cli/persistence"
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/tokenizer"
)

// Given a string, compile and execute it immediately.
func RunString(name string, s *symbols.SymbolTable, stmt string) *errors.EgoError {
	return Run(name, s, tokenizer.New(stmt))
}

// Given a token stream, compile and execute it immediately.
func Run(name string, s *symbols.SymbolTable, t *tokenizer.Tokenizer) *errors.EgoError {
	c := New(name)
	c.ExtensionsEnabled(true)

	oldState := "true"
	if !persistence.GetBool(defs.ExtensionsEnabledSetting) {
		oldState = "false"
	}

	defer persistence.SetDefault(defs.ExtensionsEnabledSetting, oldState)

	persistence.SetDefault(defs.ExtensionsEnabledSetting, "true")

	// Set the depth >0 so we will process all statements without requiring a function
	// body.
	c.functionDepth = 1

	bc, err := c.Compile(name, t)
	if errors.Nil(err) {
		err = bytecode.NewContext(s, bc).Run()
	}

	return err
}
