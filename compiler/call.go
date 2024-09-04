package compiler

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/tokenizer"
)

// compileFunctionCall generates code for the call statement. This is
// semantically the equivalent of invoking a function in an expression,
// and then discarding any result value.
//
// Note that the call statement is a language extension.
func (c *Compiler) compileFunctionCall() error {
	// Is this really panic, handled elsewhere?
	if c.flags.extensionsEnabled && c.t.Peek(1) == tokenizer.PanicToken {
		c.t.Advance(1)
		
		return c.compilePanic()
	}

	// Let's peek ahead to see if this is a legit function call. If the next token is
	// not an identifier, and it's not followed by a parenthesis or dot-notation identifier,
	// then this is not a function call and we're done.
	if !c.t.Peek(1).IsIdentifier() || (c.t.Peek(2) != tokenizer.StartOfListToken && c.t.Peek(2) != tokenizer.DotToken) {
		return c.error(errors.ErrInvalidFunctionCall)
	}

	// Parse the function as an expression. Place a marker on the stack before emitting
	// the expression for the call, so we can later discard unused return values.
	c.b.Emit(bytecode.Push, bytecode.NewStackMarker("call"))

	if err := c.emitExpression(); err != nil {
		return err
	}

	// We don't care about the result values, so flush to the marker.
	c.b.Emit(bytecode.DropToMarker, bytecode.NewStackMarker("call"))

	return nil
}
