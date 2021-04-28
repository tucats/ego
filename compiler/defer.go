package compiler

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/tokenizer"
)

func (c *Compiler) compileDefer() *errors.EgoError {
	if c.t.AnyNext(";", tokenizer.EndOfTokens) {
		return c.newError(errors.ErrMissingStatement)
	}

	start := c.b.Mark()
	c.b.Emit(bytecode.Branch, 0)

	code := c.b.Mark()

	err := c.compileStatement()
	if errors.Nil(err) {
		c.b.Emit(bytecode.Return)

		c.deferQueue = append(c.deferQueue, code)
		err = c.b.SetAddressHere(start)
	}

	return err
}
