package compiler

import (
	"github.com/tucats/ego/bytecode"
	bc "github.com/tucats/ego/bytecode"
)

func (c *Compiler) functionCall() error {

	// Note, caller already consumed the opening paren
	argc := 0
	c.b.Emit(bytecode.This, nil)

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
	c.b.Emit(bc.Call, argc)
	return nil
}

// functionOrReference compiles a function call. The value of the
// function has been pushed to the top of the stack.
func (c *Compiler) functionOrReference() error {

	// Get the atom
	err := c.reference()
	if err != nil {
		return err
	}
	// Peek ahead to see if it's the start of a function call...
	if c.t.IsNext("(") {
		return c.functionCall()
	}
	return nil
}
