package compiler

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/tokenizer"
)

func (c *Compiler) unary() error {
	// Check for unary negation or not before passing into top-level diadic operators.
	t := c.t.Peek(1)

	switch {
	case t.Is(tokenizer.NegateToken):
		c.t.Advance(1)

		if err := c.unary(); err != nil {
			return err
		}

		// Optimization for numeric constant values; if it is an integer
		// or a float64, then just update the instruction with the negative
		// of it's value. Otherwise, we'll emit an explicit Negate.
		addr := c.b.Mark() - 1
		i := c.b.Instruction(c.b.Mark() - 1)

		if i.Operation == bytecode.Push {
			switch v := i.Operand.(type) {
			case byte:
				i.Operand = -v
				c.b.Opcodes()[addr] = *i

			case int32:
				i.Operand = -v
				c.b.Opcodes()[addr] = *i

			case int:
				i.Operand = -v
				c.b.Opcodes()[addr] = *i

			case int64:
				i.Operand = -v
				c.b.Opcodes()[addr] = *i

			case float32:
				i.Operand = -v
				c.b.Opcodes()[addr] = *i

			case float64:
				i.Operand = -v
				c.b.Opcodes()[addr] = *i

			default:
				c.b.Emit(bytecode.Negate, false)
			}
		} else {
			c.b.Emit(bytecode.Negate, false)
		}

	case t.Is(tokenizer.NotToken):
		c.t.Advance(1)

		if err := c.unary(); err != nil {
			return err
		}

		c.b.Emit(bytecode.Negate, true)

	default:
		return c.functionOrReference()
	}

	return nil
}
