package compiler

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/tokenizer"
)

// Array compiles the array statement.
func (c *Compiler) Array() error {
	name := c.t.Next()
	if !tokenizer.IsSymbol(name) {
		c.t.Advance(-1)

		return c.NewError(InvalidSymbolError, name)
	}
	// See if it's a reserved word.
	if tokenizer.IsReserved(name, c.extensionsEnabled) {
		c.t.Advance(-1)

		return c.NewError(InvalidSymbolError, name)
	}

	name = c.Normalize(name)

	if !c.t.IsNext("[") {
		return c.NewError(MissingBracketError)
	}

	bc, err := c.Expression()
	if err != nil {
		return err
	}

	c.b.Append(bc)

	if !c.t.IsNext("]") {
		return c.NewError(MissingBracketError)
	}

	if c.t.IsNext("=") {
		bc, err = c.Expression()
		if err != nil {
			return nil
		}

		c.b.Append(bc)
		c.b.Emit(bytecode.MakeArray, 2)
	} else {
		c.b.Emit(bytecode.MakeArray, 1)
	}

	c.b.Emit(bytecode.SymbolCreate, name)
	c.b.Emit(bytecode.Store, name)

	return nil
}
