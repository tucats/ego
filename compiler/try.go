package compiler

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/tokenizer"
)

// compileTry compiles the try statement which allows the program to catch error
// conditions instead of stopping execution on an error.
func (c *Compiler) compileTry() error {
	// Generate start of a try block.
	b1 := c.b.Mark()
	tryMarker := bytecode.NewStackMarker("try")

	c.b.Emit(bytecode.Try, 0)
	c.b.Emit(bytecode.Push, tryMarker)

	// Statement to try
	if err := c.compileRequiredBlock(); err != nil {
		return err
	}

	c.b.Emit(bytecode.DropToMarker, tryMarker)
	b2 := c.b.Mark()

	// The catch block is optional. If not found, patch up the try destination to here
	// and generate a pop of the try stack.
	if !c.t.IsNext(tokenizer.CatchToken) {
		_ = c.b.SetAddressHere(b1)
		c.b.Emit(bytecode.TryPop)

		return nil
	}

	// Need to generate a branch around the catch block for success cases.
	c.b.Emit(bytecode.Branch, 0)
	_ = c.b.SetAddressHere(b1)

	// Is there a named variable that will hold the error?

	if c.t.IsNext(tokenizer.StartOfListToken) {
		errName := c.t.Next()
		if !errName.IsIdentifier() {
			return c.compileError(errors.ErrInvalidSymbolName)
		}

		if !c.t.IsNext(tokenizer.EndOfListToken) {
			return c.compileError(errors.ErrMissingParenthesis)
		}

		c.b.Emit(bytecode.Load, defs.ErrorVariable)
		c.b.Emit(bytecode.StoreAlways, errName)
		c.DefineSymbol(errName.Spelling())
	}

	if err := c.compileRequiredBlock(); err != nil {
		return err
	}
	// Need extra PopScope because we're still running in the scope of the try{} block
	c.b.Emit(bytecode.PopScope)

	// This marks the end of the try/catch
	_ = c.b.SetAddressHere(b2)
	c.b.Emit(bytecode.TryPop)

	return nil
}
