package compiler

import (
	bc "github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/tokenizer"
)

func (c *Compiler) compileGo() error {
	if c.t.AnyNext(tokenizer.SemicolonToken, tokenizer.EndOfTokens) {
		return c.compileError(errors.ErrMissingFunction)
	}

	// Is it a function constant?
	if c.t.IsNext(tokenizer.FuncToken) {
		// Compile a function literal onto the stack.
		isLiteral := c.isLiteralFunction()

		if err := c.compileFunctionDefinition(isLiteral); err != nil {
			return err
		}
	} else {
		// Let's peek ahead to see if this is a legit function call. If the next token is
		// not an identifier, and it's not followed by a parenthesis or dot-notation identifier,
		// then this is not a function call and we're done.
		if !c.t.Peek(1).IsIdentifier() || (c.t.Peek(2).IsNot(tokenizer.StartOfListToken) && c.t.Peek(2).IsNot(tokenizer.DotToken)) {
			return c.compileError(errors.ErrInvalidFunctionCall)
		}

		// Parse the function as an expression.
		if err := c.emitExpression(); err != nil {
			return err
		}
	}

	// Let's stop now and see if the stack looks right.
	lastBytecode := c.b.Mark()
	i := c.b.Instruction(lastBytecode - 1)

	argc, err := data.Int(i.Operand)
	if err != nil {
		return c.compileError(err)
	}

	// Drop the Call operation from the end of the bytecode
	// and replace with the "go function" operation.
	c.b.Delete(lastBytecode - 1)
	c.b.Emit(bc.Go, argc)

	return nil
}
