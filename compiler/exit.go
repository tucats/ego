package compiler

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/errors"
)

// compileExit handles the exit statement compilation.
func (c *Compiler) compileExit() *errors.EgoError {
	c.b.Emit(bytecode.Load, "util")
	c.b.Emit(bytecode.Member, "Exit")

	argCount := 0

	if !c.isStatementEnd() {
		bc, err := c.Expression()
		if !errors.Nil(err) {
			return err
		}

		c.b.Append(bc)

		argCount = 1
	}

	c.b.Emit(bytecode.Call, argCount)

	return nil
}
