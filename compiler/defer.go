package compiler

import "github.com/tucats/ego/bytecode"

func (c *Compiler) Defer() error {

	start := c.b.Mark()
	c.b.Emit(bytecode.Branch, 0)

	code := c.b.Mark()
	err := c.Statement()
	if err == nil {
		c.b.Emit(bytecode.Return)
		c.deferQueue = append(c.deferQueue, code)
		err = c.b.SetAddressHere(start)
	}
	return err
}
