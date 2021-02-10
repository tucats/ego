package compiler

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/tokenizer"
)

// Try compiles the try statement which allows the program to catch error
// conditions instead of stopping execution on an error.
func (c *Compiler) Try() error {
	// Generate start of a try block.
	b1 := c.b.Mark()

	c.b.Emit(bytecode.Try, 0)

	// Statement to try
	err := c.Statement()
	if err != nil {
		return err
	}

	b2 := c.b.Mark()

	c.b.Emit(bytecode.Branch, 0)
	_ = c.b.SetAddressHere(b1)

	if !c.t.IsNext("catch") {
		return c.NewError(MissingCatchError)
	}

	// Is there a named variable that will hold the error?

	if c.t.IsNext("(") {
		errName := c.t.Next()
		if !tokenizer.IsSymbol(errName) {
			return c.NewError(InvalidSymbolError)
		}

		if !c.t.IsNext(")") {
			return c.NewError(MissingParenthesisError)
		}

		c.b.Emit(bytecode.SymbolCreate, errName)
		c.b.Emit(bytecode.Load, bytecode.ErrorVariableName)
		c.b.Emit(bytecode.StoreAlways, errName)
	}

	err = c.Statement()
	if err != nil {
		return err
	}
	// Need extra PopScope because we're still running in the scope of the try{} block
	c.b.Emit(bytecode.PopScope)

	// This marks the end of the try/catch
	_ = c.b.SetAddressHere(b2)
	c.b.Emit(bytecode.TryPop)

	return nil
}
