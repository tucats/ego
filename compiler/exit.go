package compiler

import (
	"github.com/tucats/ego/bytecode"
)

// compileExit handles the exit statement compilation.
func (c *Compiler) compileExit() error {
	c.b.Emit(bytecode.Load, "os")
	c.b.Emit(bytecode.Member, "Exit")

	if !c.isStatementEnd() {
		if err := c.emitExpression(); err != nil {
			return err
		}
	} else {
		c.b.Emit(bytecode.Push, 0)
	}

	c.b.Emit(bytecode.Call, 1)

	return nil
}
