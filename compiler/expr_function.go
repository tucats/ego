package compiler

import (
	bc "github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/errors"
)

func (c *Compiler) functionCall() *errors.EgoError {
	// Note, caller already consumed the opening paren
	argc := 0

	for c.t.Peek(1) != ")" {
		err := c.conditional()
		if !errors.Nil(err) {
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
			c.b.Emit(bc.Flatten)

			break
		}

		if c.t.Peek(1) != "," {
			return c.NewError(errors.InvalidListError)
		}

		c.t.Advance(1)
	}

	// Ensure trailing parenthesis
	if c.t.AtEnd() || c.t.Peek(1) != ")" {
		return c.NewError(errors.MissingParenthesisError)
	}

	c.t.Advance(1)
	// Call the function
	c.b.Emit(bc.Call, argc)

	return nil
}

// functionOrReference compiles a function call. The value of the
// function has been pushed to the top of the stack.
func (c *Compiler) functionOrReference() *errors.EgoError {
	// Get the atom
	err := c.reference()
	if !errors.Nil(err) {
		return err
	}

	// Peek ahead to see if it's the start of a function call...
	if c.t.IsNext("(") {
		return c.functionCall()
	}

	return nil
}
