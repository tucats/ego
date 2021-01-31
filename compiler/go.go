package compiler

import (
	"github.com/tucats/ego/bytecode"
	bc "github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/tokenizer"
)

func (c *Compiler) Go() error {

	fName := c.t.Next()
	if !tokenizer.IsSymbol(fName) {
		return c.NewError(InvalidSymbolError, fName)
	}
	c.b.Emit(bc.Push, fName)

	if !c.t.IsNext("(") {
		return c.NewError(MissingParenthesisError)
	}
	argc := 0
	for c.t.Peek(1) != ")" {
		err := c.conditional()
		if err != nil {
			return err
		}
		argc = argc + 1
		if c.t.AtEnd() {
			break
		}
		if c.t.Peek(1) == ")" {
			break
		}
		// Could be the "..." flatten operator
		if c.t.IsNext("...") {
			c.b.Emit(bytecode.Flatten)
			break
		}
		if c.t.Peek(1) != "," {
			return c.NewError(InvalidListError)
		}
		c.t.Advance(1)
	}

	// Ensure trailing parenthesis
	if c.t.AtEnd() || c.t.Peek(1) != ")" {
		return c.NewError(MissingParenthesisError)
	}
	c.t.Advance(1)

	// Call the function
	c.b.Emit(bc.Go, argc)
	return nil
}
