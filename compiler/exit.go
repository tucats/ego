package compiler

import (
	"github.com/tucats/ego/bytecode"
)

// compileExit compiles the "exit" extension statement, which terminates the
// program with an optional integer exit code. The "exit" keyword has already
// been consumed by the caller.
//
// The statement is implemented by calling os.Exit at runtime: the compiler
// loads the "os" package, dereferences its "Exit" member, evaluates the
// optional argument expression (defaulting to 0), and emits a Call instruction.
// This means "exit" is really syntactic sugar for "os.Exit(n)".
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
