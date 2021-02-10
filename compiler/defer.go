package compiler

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/errors"
)

func (c *Compiler) Defer() *errors.EgoError {
	start := c.b.Mark()
	c.b.Emit(bytecode.Branch, 0)

	code := c.b.Mark()

	err := c.Statement()
	if errors.Nil(err) {
		c.b.Emit(bytecode.Return)

		c.deferQueue = append(c.deferQueue, code)
		err = c.b.SetAddressHere(start)
	}

	return err
}
