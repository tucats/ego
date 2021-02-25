package compiler

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/errors"
)

// compilePrint compiles a print statement. The verb is already removed
// from the token stream.
func (c *Compiler) compilePrint() *errors.EgoError {
	newline := true

	for !c.isStatementEnd() {
		if c.t.IsNext(",") {
			return c.newError(errors.UnexpectedTokenError, c.t.Peek(1))
		}

		bc, err := c.Expression()
		if !errors.Nil(err) {
			return err
		}

		newline = true

		c.b.Append(bc)
		c.b.Emit(bytecode.Print)

		if !c.t.IsNext(",") {
			break
		}

		newline = false
	}

	if newline {
		c.b.Emit(bytecode.Newline)
	}

	return nil
}
