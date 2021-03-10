package compiler

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/functions"
)

// urlDirective compiles the @url directive. This can only
// be used in a service definition.
func (c *Compiler) urlDirective() *errors.EgoError {
	_ = c.modeCheck("server", true)

	c.b.Emit(bytecode.Push, functions.URLPattern)
	c.b.Emit(bytecode.Load, "_path_suffix")

	err := c.relations()
	if !errors.Nil(err) {
		return err
	}

	c.b.Emit(bytecode.Call, 2)
	c.b.Emit(bytecode.Explode)

	return nil
}
