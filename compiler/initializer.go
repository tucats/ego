package compiler

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/tokenizer"
)

// Compile an initializer, given a type definition.
func (c *Compiler) compileInitializer(t datatypes.Type) *errors.EgoError {
	base := t
	if t.IsTypeDefinition() {
		base = *t.BaseType()
	}

	switch base.Kind() {
	case datatypes.StructKind:
		if !c.t.IsNext("{") {
			return c.newError(errors.MissingBracketError)
		}

		count := 0

		for !c.t.IsNext("}") {
			// Pairs of name:value
			name := c.t.Next()
			if !tokenizer.IsSymbol(name) {
				return c.newError(errors.InvalidSymbolError)
			}

			name = c.normalize(name)

			ft, err := base.Field(name)
			if !errors.Nil(err) {
				return err
			}

			if !c.t.IsNext(":") {
				return c.newError(errors.MissingColonError)
			}

			err = c.compileInitializer(ft)
			if !errors.Nil(err) {
				return err
			}

			// Now emit the name (names always come first on the stack)
			c.b.Emit(bytecode.Push, name)

			count++

			if c.t.IsNext("}") {
				break
			}

			if !c.t.IsNext(",") {
				return c.newError(errors.InvalidListError)
			}
		}

		c.b.Emit(bytecode.Push, t)
		c.b.Emit(bytecode.Push, "__type")
		c.b.Emit(bytecode.Struct, count+1)

		return nil

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

		// If we are doing dynamic typing, let's allow a coercion as well.
		c.b.Emit(bytecode.Coerce, base)

		return nil
	}
}
