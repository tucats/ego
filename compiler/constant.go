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
	terminator := tokenizer.EmptyToken

	if c.t.IsNext(tokenizer.StartOfListToken) {
		terminator = tokenizer.EndOfListToken
	}

	for terminator == tokenizer.EmptyToken || !c.t.IsNext(terminator) {
		name := c.t.Next()
		if !name.IsIdentifier() {
			return c.newError(errors.ErrInvalidSymbolName)
		}

		nameSpelling := c.normalize(name.Spelling())

		if !c.t.IsNext(tokenizer.AssignToken) {
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

		c.constants = append(c.constants, nameSpelling)

		c.b.Append(vx)
		c.b.Emit(bytecode.Constant, nameSpelling)

		if terminator == tokenizer.EmptyToken {
			break
		}
	}

	return nil
}
