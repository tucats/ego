package compiler

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/tokenizer"
)

// compileDefer compiles the "defer" statement. This compiles a statement,
// and attaches the resulting bytecode to the compilation unit's defer queue.
// Later, when a return is processed, this queue will be used to generate the
// appropriate deferred operations. The order of the "defer" statements determines
// the order in the queue, and therefore the order in which they are run when a
// return is executed.
func (c *Compiler) compileDefer() error {
	if c.t.AnyNext(tokenizer.SemicolonToken, tokenizer.EndOfTokens) {
		return c.error(errors.ErrMissingStatement)
	}

	// Start by branching around this block of code.
	start := c.b.Mark()
	c.b.Emit(bytecode.Branch, 0)

	// Compile the defer statement, and ensure it ends with
	// a Return operation. This address is then stored in the
	// defer queue, indicating the address(es) of defer statements
	// to be executed as local calls before a return from this
	// block.
	code := c.b.Mark()

	err := c.compileStatement()
	if err == nil {
		c.b.Emit(bytecode.Return)

		c.deferQueue = append(c.deferQueue, code)
		err = c.b.SetAddressHere(start)
	}

	return err
}
