package compiler

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/tokenizer"
)

func (c *Compiler) compileUnwrap() error {
	position := c.t.Mark()

	if c.t.IsNext(tokenizer.StartOfListToken) {
		typeName := c.t.Next()
		if typeName.IsIdentifier() {
			if c.t.IsNext(tokenizer.EndOfListToken) {
				if c.flags.inAssignment && c.flags.multipleTargets {
					c.b.Emit(bytecode.Push, bytecode.NewStackMarker("let"))
					c.b.Emit(bytecode.Swap)
				}

				c.flags.hasUnwrap = true
				
				c.b.Emit(bytecode.UnWrap, typeName)

				return nil
			}
		}
	}

	c.t.Set(position)

	return errors.ErrInvalidUnwrap
}
