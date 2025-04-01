package compiler

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/tokenizer"
)

// Compile an initializer, given a type definition. This can be a literal
// initializer in braces or a simple value.
func (c *Compiler) compileInitializer(t *data.Type) error {
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
	case data.StructKind:
		return c.parseStructInitializer(base, t)

	case data.MapKind:
		return c.parseMapInitializer(base)

	case data.ArrayKind:
		return c.parseArrayInitializer(base)

	default:
		if err := c.unary(); err != nil {
			return err
		}

		// If we are doing dynamic typing, let's allow a coercion as well.
		c.b.Emit(bytecode.Coerce, base)

		return nil
	}
}

func (c *Compiler) parseArrayInitializer(base *data.Type) error {
	count := 0

	for !c.t.IsNext(tokenizer.DataEndToken) {
		// Values separated by commas.
		if err := c.compileInitializer(base.BaseType()); err != nil {
			return err
		}

		count++

		if c.t.IsNext(tokenizer.DataEndToken) {
			break
		}

		if !c.t.IsNext(tokenizer.CommaToken) {
			return c.compileError(errors.ErrInvalidList)
		}
	}

	// Emit the type as the final datum, and then emit the instruction
	// that will construct the array from the type and stack items.
	c.b.Emit(bytecode.Push, base.BaseType())
	c.b.Emit(bytecode.MakeArray, count)

	return nil
}

func (c *Compiler) parseMapInitializer(base *data.Type) error {
	count := 0

	for !c.t.IsNext(tokenizer.DataEndToken) {
		// Pairs of values with a colon between.
		if err := c.unary(); err != nil {
			return err
		}

		c.b.Emit(bytecode.Coerce, base.KeyType())

		if !c.t.IsNext(tokenizer.ColonToken) {
			return c.compileError(errors.ErrMissingColon)
		}

		// Note we compile the value using ourselves, to allow for nested
		// type specifications.
		if err := c.compileInitializer(base.BaseType()); err != nil {
			return err
		}

		count++

		if c.t.IsNext(tokenizer.DataEndToken) {
			break
		}

		if !c.t.IsNext(tokenizer.CommaToken) {
			return c.compileError(errors.ErrInvalidList)
		}
	}

	c.b.Emit(bytecode.Push, base.BaseType())
	c.b.Emit(bytecode.Push, base.KeyType())
	c.b.Emit(bytecode.MakeMap, count)

	return nil
}

func (c *Compiler) parseStructInitializer(base *data.Type, t *data.Type) error {
	count := 0

	c.b.Emit(bytecode.Push, bytecode.NewStackMarker("struct-init"))

	// It's possible this is an initializer of an ordered list of values. If so, they must
	// match the number of fields, and are stored in the designated order. We determine this
	// is the case by checking for an expression followed by a comma or the end of the data
	tokenMark := c.t.Mark()

	if _, err := c.Expression(false); err == nil && c.t.Peek(1).Is(tokenizer.CommaToken) || c.t.Peek(1).Is(tokenizer.DataEndToken) {
		// First, back up the tokenizer position
		fieldCount, err := c.structInitializeByOrderedList(tokenMark, base)
		if err != nil {
			return err
		} else {
			count += fieldCount
		}
	} else {
		// Initialization is done by named fields. Parse each field name and value
		fieldCount, err := c.structInitializeByName(tokenMark, base)
		if err != nil {
			return err
		} else {
			count += fieldCount
		}
	}

	c.b.Emit(bytecode.Push, t)
	c.b.Emit(bytecode.Push, data.TypeMDKey)
	c.b.Emit(bytecode.Struct, count+1)

	return nil
}

// structInitializeByOrderedList generates code for field initialization using an ordered list of values.
// These must match up to the types of the corresponding fields in the structure for the type.
func (c *Compiler) structInitializeByOrderedList(tokenMark int, base *data.Type) (int, error) {
	var count int

	c.t.Set(tokenMark)

	fieldNames := base.FieldNames()

	for !c.t.IsNext(tokenizer.DataEndToken) {
		if c.t.AtEnd() {
			break
		}

		if count >= len(fieldNames) {
			return 0, c.compileError(errors.ErrInitializerCount, count)
		}

		// Is this initializer a type name for a struct?
		typeName := c.t.Peek(1)
		if typeName.IsIdentifier() {
			if typeData, ok := c.types[typeName.Spelling()]; ok {
				if typeData.Kind() == data.TypeKind && typeData.BaseType().Kind() == data.StructKind {
					// Do the types of the embedded type align with the struct fields?
					isEmbedded := isEmbeddedTypeName(typeData, fieldNames, count, base)

					if isEmbedded {
						// We will have to run the list manually, so start by eating
						// the type name, and checking to see if there is a bracket
						// indicating an initializer list.
						c.t.Advance(1)

						if !c.t.IsNext(tokenizer.DataBeginToken) {
							continue
						}

						initializers, err := c.compileEmbeddedInitializer(fieldNames, base)
						if err != nil {
							return 0, err
						}

						count += initializers

						continue
					}
				}
			} else {
				// Parse the next value expression
				fieldName := fieldNames[count]
				fieldType, _ := base.Field(fieldName)

				// Generate the initializer value from the expression
				if err := c.compileInitializer(fieldType); err != nil {
					return 0, err
				}

				// Now emit the name of the field
				c.b.Emit(bytecode.Push, fieldName)

				count++

				if c.t.IsNext(tokenizer.DataEndToken) {
					break
				}

				if !c.t.IsNext(tokenizer.CommaToken) {
					return 0, c.compileError(errors.ErrInvalidList)
				}
			}
		} else {
			// Parse the next value expression
			fieldName := fieldNames[count]
			fieldType, _ := base.Field(fieldName)

			// Generate the initializer value from the expression
			if err := c.compileInitializer(fieldType); err != nil {
				return 0, err
			}

			// Now emit the name of the field
			c.b.Emit(bytecode.Push, fieldName)

			count++

			if c.t.IsNext(tokenizer.DataEndToken) {
				break
			}

			if !c.t.IsNext(tokenizer.CommaToken) {
				return 0, c.compileError(errors.ErrInvalidList)
			}
		}
	}

	// If we failed to initialize all fields, return an error
	if count < len(fieldNames) {
		return 0, c.compileError(errors.ErrInitializerCount, count)
	}

	return count, nil
}

func (c *Compiler) structInitializeByName(tokenMark int, base *data.Type) (int, error) {
	var count int

	c.t.Set(tokenMark)

	// Scan the list of field names and values
	for !c.t.IsNext(tokenizer.DataEndToken) {
		// Pairs of name:value
		name := c.t.Next()
		if !name.IsIdentifier() {
			return 0, c.compileError(errors.ErrInvalidSymbolName)
		}

		name = tokenizer.NewIdentifierToken(c.normalize(name.Spelling()))

		fieldType, err := base.Field(name.Spelling())
		if err != nil {
			return 0, err
		}

		if !c.t.IsNext(tokenizer.ColonToken) {
			return 0, c.compileError(errors.ErrMissingColon)
		}

		if err = c.compileInitializer(fieldType); err != nil {
			return 0, err
		}

		// Now emit the name (names always come first on the stack)
		c.b.Emit(bytecode.Push, name)

		count++

		if c.t.IsNext(tokenizer.DataEndToken) {
			break
		}

		if !c.t.IsNext(tokenizer.CommaToken) {
			return 0, c.compileError(errors.ErrInvalidList)
		}
	}

	return count, nil
}

func (c *Compiler) compileEmbeddedInitializer(fieldNames []string, base *data.Type) (int, error) {
	var count int

	for c.t.Peek(1).IsNot(tokenizer.DataEndToken) {
		fieldName := fieldNames[count]
		fieldType, _ := base.Field(fieldName)

		if err := c.compileInitializer(fieldType); err != nil {
			return 0, err
		}

		c.b.Emit(bytecode.Push, fieldName)

		count++
		_ = c.t.IsNext(tokenizer.CommaToken)
	}

	if !c.t.IsNext(tokenizer.DataEndToken) {
		return 0, c.compileError(errors.ErrInvalidList)
	}

	_ = c.t.IsNext(tokenizer.CommaToken)

	return count, nil
}

// isEmbeddedTypeName checks if the given type name is an reference to a type with matching field names.
// Returns true if the field names of the base type all match the field names in the provided list, and
// match the type of the field.
func isEmbeddedTypeName(typeData *data.Type, fieldNames []string, count int, base *data.Type) bool {
	embeddedNames := typeData.BaseType().FieldNames()
	isEmbedded := true

	for idx, name := range embeddedNames {
		if fieldNames[idx+count] != name {
			isEmbedded = false

			break
		}

		baseField, e1 := base.Field(name)
		embeddedField, e2 := typeData.BaseType().Field(name)

		if e1 != nil || e2 != nil {
			isEmbedded = false

			break
		}

		if baseField.Kind() != embeddedField.Kind() {
			isEmbedded = false

			break
		}
	}

	return isEmbedded
}
