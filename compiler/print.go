package compiler

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/errors"
)

// Print compiles a print statement. The verb is already removed
// from the token stream.
func (c *Compiler) Print() *errors.EgoError {
	newline := true

	for !c.StatementEnd() {
		if c.t.IsNext(",") {
			return c.NewError(errors.UnexpectedTokenError, c.t.Peek(1))
		}

		bc, err := c.Expression()
		if err != nil {
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
