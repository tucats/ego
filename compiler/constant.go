package compiler

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/tokenizer"
	"github.com/tucats/ego/util"
)

// compileConst compiles a constant block.
func (c *Compiler) compileConst() *errors.EgoError {
	terminator := ""

	if c.t.IsNext("(") {
		terminator = ")"
	}

	for terminator == "" || !c.t.IsNext(terminator) {
		name := c.t.Next()
		if !tokenizer.IsSymbol(name) {
			return c.newError(errors.ErrInvalidSymbolName)
		}

		name = c.normalize(name)

		if !c.t.IsNext("=") {
			return c.newError(errors.ErrMissingEqual)
		}

		vx, err := c.Expression()
		if !errors.Nil(err) {
			return err
		}

		// Search to make sure it doesn't contain a load statement that isn't for another
		// constant

		for _, i := range vx.Opcodes() {
			if i.Operation == bytecode.Load && !util.InList(datatypes.GetString(i.Operand), c.constants...) {
				return c.newError(errors.ErrInvalidConstant)
			}
		}

		c.constants = append(c.constants, name)

		c.b.Append(vx)
		c.b.Emit(bytecode.Constant, name)

		if terminator == "" {
			break
		}
	}

	return nil
}
