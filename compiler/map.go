package compiler

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
)

// Map compiles a map type declaration

func (c *Compiler) mapDeclaration() *errors.EgoError {
	if !c.t.IsNext("map") {
		return c.newError(errors.UnexpectedTokenError, c.t.Peek(1))
	}

	if !c.t.IsNext("[") {
		return c.newError(errors.MissingBracketError)
	}

	// Parse the key type
	keyType := c.parseTypeSpec()
	if keyType == datatypes.UndefinedType {
		return c.newError(errors.InvalidTypeSpecError)
	}

	// Closing bracket on the key type
	if !c.t.IsNext("]") {
		return c.newError(errors.MissingBracketError)
	}

	// Parse the object type
	valueType := c.parseTypeSpec()
	if valueType == datatypes.UndefinedType {
		return c.newError(errors.InvalidTypeSpecError)
	}

	// Make a suitable map object and push it on the stack.
	c.b.Emit(bytecode.Push, datatypes.NewMap(keyType, valueType))

	// Eat {}, if not present parse an initializer}
	if !c.t.IsNext("{}") {
		if !c.t.IsNext("{") {
			return c.newError(errors.MissingBlockError)
		}

		for {
			if c.t.IsNext("}") {
				break
			}

			keyBC, err := c.Expression()
			if !errors.Nil(err) {
				return err
			}

			if !c.t.IsNext(":") {
				return c.newError(errors.MissingColonError)
			}

			valueBC, err := c.Expression()
			if !errors.Nil(err) {
				return err
			}

			c.b.Append(valueBC)
			c.b.Append(keyBC)
			c.b.Emit(bytecode.StoreInto)

			if c.t.Peek(1) == "," {
				c.t.Advance(1)

				continue
			}

			if c.t.Peek(1) != "}" {
				return c.newError(errors.MissingEndOfBlockError)
			}
		}
	}

	return nil
}
