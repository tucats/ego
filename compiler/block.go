package compiler

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/errors"
)

// Block compiles a statement block. The leading { has already
// been parsed.
func (c *Compiler) Block() *EgoError {
	parsing := true
	c.blockDepth = c.blockDepth + 1

	c.b.Emit(bytecode.PushScope)

	for parsing {
		if c.t.IsNext("}") {
			break
		}

		err := c.Statement()
		if err != nil {
			return err
		}

		// Skip over a semicolon if found
		_ = c.t.IsNext(";")

		if c.t.AtEnd() {
			return c.NewError(errors.MissingEndOfBlockError)
		}
	}

	c.b.Emit(bytecode.PopScope)
	c.blockDepth = c.blockDepth - 1

	return nil
}
