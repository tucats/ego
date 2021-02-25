package compiler

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
)

func (c *Compiler) makeInvocation() *errors.EgoError {
	if !c.t.IsNext("make") {
		return c.newError(errors.UnexpectedTokenError, c.t.Peek(1))
	}

	if !c.t.IsNext("(") {
		return c.newError(errors.MissingParenthesisError)
	}

	c.b.Emit(bytecode.Load, "make")

	// is this a channel?
	if c.t.IsNext("chan") {
		c.b.Emit(bytecode.Push, &datatypes.Channel{})
	} else {
		found := false

		for _, typeDef := range datatypes.TypeDeclarationMap {
			found = true

			for pos, token := range typeDef.Tokens {
				if c.t.Peek(1+pos) != token {
					found = false
				}
			}
			if found {
				c.t.Advance(len(typeDef.Tokens))
				c.b.Emit(bytecode.Push, typeDef.Model)

				break
			}
		}

		if !found {
			return c.newError(errors.InvalidTypeSpecError)
		}
	}

	if !c.t.IsNext(",") {
		return c.newError(errors.InvalidListError)
	}

	bc, err := c.Expression()

	c.b.Append(bc)
	c.b.Emit(bytecode.Call, 2)

	if errors.Nil(err) && !c.t.IsNext(")") {
		err = c.newError(errors.MissingParenthesisError)
	}

	return err
}
