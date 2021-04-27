package compiler

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/errors"
)

// compileBlock compiles a statement block. The leading { has already
// been parsed.
func (c *Compiler) compileBlock() *errors.EgoError {
	parsing := true
	c.blockDepth = c.blockDepth + 1

	c.b.Emit(bytecode.PushScope)

	for parsing {
		if c.t.IsNext("}") {
			break
		}

		err := c.compileStatement()
		if !errors.Nil(err) {
			return err
		}

		// Skip over a semicolon if found
		_ = c.t.IsNext(";")

		if c.t.AtEnd() {
			return c.newError(errors.ErrMissingEndOfBlock)
		}
	}

	c.b.Emit(bytecode.PopScope)

	c.blockDepth = c.blockDepth - 1

	return nil
}

// Require that the next item be a block, enclosed in {} characters.
func (c *Compiler) compileRequiredBlock() *errors.EgoError {
	// If an empty block, no work to do
	if c.t.IsNext("{}") {
		return nil
	}

	// Otherwise, needs to start with the open block
	if !c.t.IsNext("{") {
		return c.newError(errors.ErrMissingBlock)
	}

	// now compile and close the block.
	return c.compileBlock()
}
