package compiler

import bc "github.com/tucats/ego/bytecode"

func (c *Compiler) unary() error {
	// Check for unary negation or not before passing into top-level diadic operators.
	t := c.t.Peek(1)
	switch t {
	case "-":
		c.t.Advance(1)
		err := c.functionOrReference()
		if err != nil {
			return err
		}
		c.b.Emit(bc.Negate, 0)

		return nil

	case "!":
		c.t.Advance(1)
		err := c.functionOrReference()
		if err != nil {
			return err
		}
		c.b.Emit(bc.Negate, 0)

		return nil

	default:
		return c.functionOrReference()
	}
}
