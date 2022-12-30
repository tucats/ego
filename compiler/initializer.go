package compiler

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/tokenizer"
)

// Compile an initializer, given a type definition. This can be a literal
// initializer in braces or a simple value.
func (c *Compiler) compileInitializer(t *datatypes.Type) *errors.EgoError {
	if !c.t.IsNext(tokenizer.DataBeginToken) {
		// It's not an initializer constant, but it could still be an expression. Try the
		// top-level expression compiler.
		return c.conditional()
	}

	base := t
	if t.IsTypeDefinition() {
		base = t.BaseType()
	}

	switch base.Kind() {
	case datatypes.StructKind:
		count := 0

		for !c.t.IsNext(tokenizer.DataEndToken) {
			// Pairs of name:value
			name := c.t.Next()
			if !name.IsIdentifier() {
				return c.newError(errors.ErrInvalidSymbolName)
			}

			name = tokenizer.NewIdentifierToken(c.normalize(name.Spelling()))

			ft, err := base.Field(name.Spelling())
			if !errors.Nil(err) {
				return err
			}

			if !c.t.IsNext(tokenizer.ColonToken) {
				return c.newError(errors.ErrMissingColon)
			}

			err = c.compileInitializer(ft)
			if !errors.Nil(err) {
				return err
			}

			// Now emit the name (names always come first on the stack)
			c.b.Emit(bytecode.Push, name)

			count++

			if c.t.IsNext(tokenizer.DataEndToken) {
				break
			}

			if !c.t.IsNext(tokenizer.CommaToken) {
				return c.newError(errors.ErrInvalidList)
			}
		}

		c.b.Emit(bytecode.Push, t)
		c.b.Emit(bytecode.Push, datatypes.TypeMDKey)
		c.b.Emit(bytecode.Struct, count+1)

		return nil

	case datatypes.MapKind:
		count := 0

		for !c.t.IsNext(tokenizer.DataEndToken) {
			// Pairs of values with a colon between.
			err := c.unary()
			if !errors.Nil(err) {
				return err
			}

			c.b.Emit(bytecode.Coerce, base.KeyType())

			if !c.t.IsNext(tokenizer.ColonToken) {
				return c.newError(errors.ErrMissingColon)
			}

			// Note we compile the value using ourselves, to allow for nested
			// type specifications.
			err = c.compileInitializer(base.BaseType())
			if !errors.Nil(err) {
				return err
			}

			count++

			if c.t.IsNext(tokenizer.DataEndToken) {
				break
			}

			if !c.t.IsNext(tokenizer.CommaToken) {
				return c.newError(errors.ErrInvalidList)
			}
		}

		c.b.Emit(bytecode.Push, base.BaseType())
		c.b.Emit(bytecode.Push, base.KeyType())
		c.b.Emit(bytecode.MakeMap, count)

		return nil

	case datatypes.ArrayKind:
		count := 0

		for !c.t.IsNext(tokenizer.DataEndToken) {
			// Values separated by commas.
			err := c.compileInitializer(base.BaseType())
			if !errors.Nil(err) {
				return err
			}

			count++

			if c.t.IsNext(tokenizer.DataEndToken) {
				break
			}

			if !c.t.IsNext(tokenizer.CommaToken) {
				return c.newError(errors.ErrInvalidList)
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
