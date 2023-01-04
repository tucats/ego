package compiler

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/tokenizer"
)

func (c *Compiler) compileDefer() error {
	if c.t.AnyNext(tokenizer.SemicolonToken, tokenizer.EndOfTokens) {
		return c.newError(errors.ErrMissingStatement)
	}

	start := c.b.Mark()
	c.b.Emit(bytecode.Branch, 0)

	code := c.b.Mark()

	err := c.compileStatement()
	if err == nil {
		c.b.Emit(bytecode.Return)

		c.deferQueue = append(c.deferQueue, code)
		err = c.b.SetAddressHere(start)
	}

	return err
}
