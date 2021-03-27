package compiler

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
)

// Compile an initializer, given a type definition.
func (c *Compiler) compileInitializer(t datatypes.Type) *errors.EgoError {
	base := t
	if t.IsTypeDefinition() {
		base = *t.BaseType()
	}

	switch base.Kind() {
	case datatypes.MapKind:
		if !c.t.IsNext("{") {
			return c.newError(errors.MissingBracketError)
		}

		count := 0

		for !c.t.IsNext("}") {
			// Pairs of values with a colon between.
			err := c.unary()
			if !errors.Nil(err) {
				return err
			}

			c.b.Emit(bytecode.Coerce, base.KeyType())

			if !c.t.IsNext(":") {
				return c.newError(errors.MissingColonError)
			}

			// Note we compile the value using ourselves, to allow for nested
			// type specifications.
			err = c.compileInitializer(*base.BaseType())
			if !errors.Nil(err) {
				return err
			}

			count++

			if c.t.IsNext("}") {
				break
			}

			if !c.t.IsNext(",") {
				return c.newError(errors.InvalidListError)
			}
		}

		c.b.Emit(bytecode.Push, base.BaseType())
		c.b.Emit(bytecode.Push, base.KeyType())
		c.b.Emit(bytecode.MakeMap, count)

		return nil

	case datatypes.ArrayKind:
		if !c.t.IsNext("{") {
			return c.newError(errors.MissingBracketError)
		}

		count := 0

		for !c.t.IsNext("}") {
			// Values separated by commas.
			err := c.compileInitializer(*base.BaseType())
			if !errors.Nil(err) {
				return err
			}

			count++

			if c.t.IsNext("}") {
				break
			}

			if !c.t.IsNext(",") {
				return c.newError(errors.InvalidListError)
			}
		}

		c.b.Emit(bytecode.Push, base.BaseType())
		c.b.Emit(bytecode.MakeArray, count)

		return nil

	default:
		err := c.unary()
		if !errors.Nil(err) {
			return err
		}

		// If we are doing dynamic typing, let's allow a coercsion as well.
		c.b.Emit(bytecode.Coerce, base)

		return nil
	}
}
