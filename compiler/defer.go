package compiler

import (
	"github.com/tucats/ego/bytecode"
	bc "github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/tokenizer"
)

// compileDefer compiles the "defer" statement. This compiles a statement,
// and attaches the resulting bytecode to the compilation unit's defer queue.
// Later, when a return is processed, this queue will be used to generate the
// appropriate deferred operations. The order of the "defer" statements determines
// the order in the queue, and therefore the order in which they are run when a
// return is executed.
func (c *Compiler) compileDefer() error {
	minDepth := 1
	if c.flags.exitEnabled {
		minDepth = 2
	}

	if c.functionDepth < minDepth {
		return c.compileError(errors.ErrDeferOutsideFunction)
	}

	if c.t.EndofStatement() {
		return c.compileError(errors.ErrMissingFunction)
	}

	// Is it a function constant?
	if c.t.IsNext(tokenizer.FuncToken) {
		c.b.Emit(bc.DeferStart, true)

		// Compile a function literal onto the stack.
		if err := c.compileFunctionDefinition(c.isLiteralFunction()); err != nil {
			return err
		}
	} else {
		c.b.Emit(bc.DeferStart, false)

		// Peek ahead to see if this is a legit function call. If the next token is not an
		// identifier, and it's not followed by a parenthesis or dot-notation identifier,
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
	if i.Operation != bytecode.Call {
		return c.compileError(errors.ErrInvalidFunctionCall)
	}

	argc, err := data.Int(i.Operand)
	if err != nil {
		return c.compileError(err)
	}

	// Drop the Call operation from the end of the bytecode
	// and replace with the "defer function" operation.
	c.b.Delete(lastBytecode - 1)
	c.b.Emit(bc.Defer, argc)

	return nil
}
