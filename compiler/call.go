package compiler

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/tokenizer"
)

// compileFunctionCall handles the call statement. This is really the same as
// invoking a function in an expression, except there is no
// result value.
func (c *Compiler) compileFunctionCall() *errors.EgoError {
	// Let's peek ahead to see if this is a legit function call
	if !c.t.Peek(1).IsIdentifier() || (c.t.Peek(2) != tokenizer.ChannelSendToken && c.t.Peek(2) != tokenizer.StartOfListToken && c.t.Peek(2) != tokenizer.DotToken) {
		return c.newError(errors.ErrInvalidFunctionCall)
	}

	c.b.Emit(bytecode.Push, bytecode.NewStackMarker("call"))
	// Parse the function as an expression
	bc, err := c.Expression()
	if !errors.Nil(err) {
		return err
	}

	c.b.Append(bc)

	// We don't care about the result values, so flush to the marker.
	c.b.Emit(bytecode.DropToMarker, bytecode.NewStackMarker("call"))

	return nil
}
