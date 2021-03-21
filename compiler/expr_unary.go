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
	case "-":
		c.t.Advance(1)

		err := c.unary()
		if !errors.Nil(err) {
			return err
		}

		// Optimization for numeric constant values; if it is an integer
		// or a float, then just update the instruction with the negative
		// of it's value. Otherwise, we'll emit an explicit Negate.
		addr := c.b.Mark() - 1
		i := c.b.GetInstruction(c.b.Mark() - 1)

		if i.Operation == bytecode.Push {
			switch v := i.Operand.(type) {
			case int:
				i.Operand = -v
				c.b.Opcodes()[addr] = *i

			case float64:
				i.Operand = -v
				c.b.Opcodes()[addr] = *i

			default:
				c.b.Emit(bc.Negate, false)
			}
		} else {
			c.b.Emit(bc.Negate, false)
		}

	case "!":
		c.t.Advance(1)

		err := c.unary()
		if !errors.Nil(err) {
			return err
		}

		c.b.Emit(bc.Negate, true)

	default:
		return c.functionOrReference()
	}

	return nil
}
