package compiler

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/tokenizer"
)

// Call handles the call statement. This is really the same as
// invoking a function in an expression, except there is no
// result value.
func (c *Compiler) Call() error {
	// Let's peek ahead to see if this is a legit function call
	if !tokenizer.IsSymbol(c.t.Peek(1)) || (c.t.Peek(2) != "->" && c.t.Peek(2) != "(" && c.t.Peek(2) != ".") {
		return c.NewError(InvalidFunctionCall)
	}

	c.b.Emit(bytecode.Push, bytecode.StackMarker{Desc: "call"})
	// Parse the function as an expression
	bc, err := c.Expression()
	if err != nil {
		return err
	}
	c.b.Append(bc)

	// We don't care about the result values, so flush to the marker.
	c.b.Emit(bytecode.DropToMarker)

	return nil
}
