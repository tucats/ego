package compiler

import (
	"github.com/tucats/ego/bytecode"
	bc "github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/errors"
)

func (c *Compiler) unary() *errors.EgoError {
	// Check for unary negation or not before passing into top-level diadic operators.
	t := c.t.Peek(1)

	switch t {
	case "&":
		c.t.Advance(1)

		err := c.functionOrReference()
		if !errors.Nil(err) {
			return err
		}

		c.b.Emit(bc.Address, false)

	case "*":
		c.t.Advance(1)

		err := c.functionOrReference()
		if !errors.Nil(err) {
			return err
		}

		c.b.Emit(bytecode.Address, true)

	case "-":
		c.t.Advance(1)

		err := c.functionOrReference()
		if !errors.Nil(err) {
			return err
		}

		c.b.Emit(bc.Negate, 0)

	case "!":
		c.t.Advance(1)

		err := c.functionOrReference()
		if !errors.Nil(err) {
			return err
		}

		c.b.Emit(bc.Negate, 0)

	default:
		return c.functionOrReference()
	}

	return nil
}
