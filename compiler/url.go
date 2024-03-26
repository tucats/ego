package compiler

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/runtime/strings"
)

// urlDirective compiles the @url directive. This can only
// be used in a service definition.
func (c *Compiler) urlDirective() error {
	_ = c.modeCheck("server")

	c.b.Emit(bytecode.Push, strings.URLPattern)
	c.b.Emit(bytecode.Load, "_path_suffix")

	if err := c.relations(); err != nil {
		return err
	}

	c.b.Emit(bytecode.Call, 2)
	c.b.Emit(bytecode.Explode)

	// This leaves a boolean on the stack indicating if the result
	// was empty. If not empty, branch around the error report.
	branch := c.b.Mark()
	c.b.Emit(bytecode.BranchFalse, 0)

	c.b.Emit(bytecode.Load, "BadURL")
	c.b.Emit(bytecode.Load, "_path_suffix")
	c.b.Emit(bytecode.Call, 1)
	c.b.Emit(bytecode.Return)

	return c.b.SetAddressHere(branch)
}
