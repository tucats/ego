package compiler

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/tokenizer"
)

// compileInitializer compiles a value that initializes a typed variable. The
// type t describes what kind of value is expected. Two forms are handled:
//
//   - Brace-enclosed literal:  { … }
//     Dispatches to the appropriate typed initializer (struct, map, or array)
//     based on the base kind of t.
//
//   - Plain expression:
//     Falls through to the conditional expression compiler.
//
// This function is called recursively for nested types (e.g. an array of
// structs whose field values are themselves structs).
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

// parseArrayInitializer compiles a brace-enclosed list of values into an array.
// The opening "{" has already been consumed by the caller. Each comma-separated
// element is compiled by recursively calling compileInitializer with the array's
// element type. After all elements are on the stack, a MakeArray instruction is
// emitted with the element count so the runtime can build the array value.
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

// parseMapInitializer compiles a brace-enclosed set of key:value pairs into a
// map. The opening "{" has already been consumed. Keys and values are compiled
// as expressions and coerced to the map's declared key and value types. After
// all pairs are on the stack, a MakeMap instruction is emitted with the pair
// count.
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

// parseStructInitializer compiles a brace-enclosed struct literal. The opening
// "{" has already been consumed. A stack marker is pushed first so the Struct
// bytecode instruction can find the boundary between the struct fields and
// whatever was on the stack before.
//
// Two initialization styles are probed:
//   - Ordered list:  Point{1, 2}   — values assigned to fields in declaration order.
//   - Named fields:  Point{x:1, y:2} — values assigned by field name.
//
// The probe is done by trying Expression() on the first token; if it succeeds and
// is followed by "," or "}" then it is an ordered list.
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

// structInitializeByName compiles a named-field struct initializer such as
// Point{x: 1, y: 2}. The token position is rewound to tokenMark, then each
// "fieldName: value" pair is compiled. The value is type-checked against the
// declared field type, and the field name is pushed as a string constant so
// the Struct instruction can match them up at runtime.
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

// compileEmbeddedInitializer compiles the body of an embedded struct type that
// appears inside an ordered-list initializer. The caller has already consumed
// the type name and the opening "{"; this function reads the field values in
// order, emitting each value and its corresponding field name, and returns the
// number of fields initialized.
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
