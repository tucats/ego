package compiler

import (
	bc "github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/tokenizer"
)

func (c *Compiler) compileGo() *errors.EgoError {
	if c.t.AnyNext(tokenizer.SemicolonToken, tokenizer.EndOfTokens) {
		return c.newError(errors.ErrMissingFunction)
	}

	fName := c.t.Next()
	if !tokenizer.IsSymbol(fName) {
		return c.newError(errors.ErrInvalidSymbolName, fName)
	}

	// Is it a function constant?
	if fName == tokenizer.FuncToken {
		fName = datatypes.GenerateName()

		// Compile a function literal onto the stack.
		err := c.compileFunctionDefinition(true)
		if err != nil {
			return err
		}

		c.b.Emit(bc.StoreBytecode, fName)
	}

	c.b.Emit(bc.Push, fName)

	if !c.t.IsNext(tokenizer.StartOfListToken) {
		return c.newError(errors.ErrMissingParenthesis)
	}

	argc := 0

	for c.t.Peek(1) != tokenizer.EndOfListToken {
		err := c.conditional()
		if !errors.Nil(err) {
			return err
		}

		argc = argc + 1

		if c.t.AtEnd() {
			break
		}

		if c.t.Peek(1) == tokenizer.EndOfListToken {
			break
		}

		// Could be the "..." flatten operator
		if c.t.IsNext(tokenizer.VariadicToken) {
			c.b.Emit(bc.Flatten)

			break
		}

		if c.t.Peek(1) != tokenizer.CommaToken {
			return c.newError(errors.ErrInvalidList)
		}

		c.t.Advance(1)
	}

	// Ensure trailing parenthesis
	if c.t.AtEnd() || c.t.Peek(1) != tokenizer.EndOfListToken {
		return c.newError(errors.ErrMissingParenthesis)
	}

	c.t.Advance(1)
	c.b.Emit(bc.Go, argc)

	return nil
}
