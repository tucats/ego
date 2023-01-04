package compiler

import (
	"github.com/tucats/ego/bytecode"
)

// compileExit handles the exit statement compilation.
func (c *Compiler) compileExit() error {
	c.b.Emit(bytecode.Load, "os")
	c.b.Emit(bytecode.Member, "Exit")

	argCount := 0

	if !c.isStatementEnd() {
		bc, err := c.Expression()
		if err != nil {
			return err
		}

		c.b.Append(bc)

		argCount = 1
	}

	c.b.Emit(bytecode.Call, argCount)

	return nil
}
