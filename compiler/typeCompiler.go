package compiler

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/tokenizer"
)

func (c *Compiler) typeEmitter(name string) error {
	typeInfo, err := c.typeCompiler(name)
	if err == nil {
		c.b.Emit(bytecode.Push, typeInfo)
		c.b.Emit(bytecode.StoreAlways, name)
	}

	return err
}

func (c *Compiler) typeCompiler(name string) (*datatypes.Type, error) {
	if _, found := c.types[name]; found {
		return &datatypes.UndefinedType, c.newError(errors.ErrDuplicateTypeName).Context(name)
	}

	baseType, err := c.parseType(name, false)
	if err != nil {
		return &datatypes.UndefinedType, err
	}

	typeInfo := datatypes.TypeDefinition(name, baseType)
	c.types[name] = typeInfo

	return typeInfo, nil
}

func (c *Compiler) parseType(name string, anonymous bool) (*datatypes.Type, error) {
	found := false

	if !anonymous {
		// Is it a previously defined type?
		typeName := c.t.Peek(1)
		if typeName.IsIdentifier() {
			if t, ok := c.types[typeName.Spelling()]; ok {
				c.t.Advance(1)

				return t, nil
			}
		}
	}

	// Is it a known complex type?

	// Empty interface
	if c.t.Peek(1) == tokenizer.EmptyInterfaceToken {
		c.t.Advance(1)

		return &datatypes.InterfaceType, nil
	}

	if c.t.Peek(1) == tokenizer.InterfaceToken && c.t.Peek(2) == tokenizer.EmptyInitializerToken {
		c.t.Advance(2)

		return &datatypes.InterfaceType, nil
	}

	// Interfaces
	if c.t.Peek(1) == tokenizer.InterfaceToken && c.t.Peek(2) == tokenizer.DataBeginToken {
		c.t.Advance(2)

		t := datatypes.NewInterfaceType(name)

		// Parse function declarations, add to the type object.
		for !c.t.IsNext(tokenizer.DataEndToken) {
			f, err := c.parseFunctionDeclaration()
			if err != nil {
				return &datatypes.UndefinedType, err
			}

			t.DefineFunction(f.Name, f)
		}

		return t, nil
	}

	// Maps
	if c.t.Peek(1) == tokenizer.MapToken && c.t.Peek(2) == tokenizer.StartOfArrayToken {
		c.t.Advance(2)

		keyType, err := c.parseType("", false)
		if err != nil {
			return &datatypes.UndefinedType, err
		}

		if !c.t.IsNext(tokenizer.EndOfArrayToken) {
			return &datatypes.UndefinedType, c.newError(errors.ErrMissingBracket)
		}

		valueType, err := c.parseType("", false)
		if err != nil {
			return &datatypes.UndefinedType, err
		}

		return datatypes.Map(keyType, valueType), nil
	}

	// Structures
	if c.t.Peek(1) == tokenizer.StructToken && c.t.Peek(2) == tokenizer.DataBeginToken {
		t := datatypes.Structure()
		c.t.Advance(2)

		for !c.t.IsNext(tokenizer.DataEndToken) {
			name := c.t.Next()
			if !name.IsIdentifier() {
				return &datatypes.UndefinedType, c.newError(errors.ErrInvalidSymbolName)
			}

			// Is it a compound name? Could be a package reference to an embedded type.
			if c.t.Peek(1) == tokenizer.DotToken && c.t.Peek(2).IsIdentifier() {
				packageName := name
				name := c.t.Peek(2)

				// Is it a package name? If so, convert it to an actual package type and
				// look to see if this is a known type. If so, copy the embedded fields to
				// the newly created type we're working on.
				if pkgData, found := c.Symbols().Get(packageName.Spelling()); found {
					if pkg, ok := pkgData.(*datatypes.EgoPackage); ok {
						if typeInterface, ok := pkg.Get(name.Spelling()); ok {
							if typeData, ok := typeInterface.(*datatypes.Type); ok {
								embedType(t, typeData)

								// Skip past the tokens and any optional trailing comma
								c.t.Advance(2)
								c.t.IsNext(tokenizer.CommaToken)

								continue
							}
						}
					}
				}
			}

			// Is the name actually a type that we embed? If so, get the base type and iterate
			// over its fields, copying them into our current structure definition.
			if typeData, found := c.types[name.Spelling()]; found {
				embedType(t, typeData)
				c.t.IsNext(tokenizer.CommaToken)

				continue
			}

			// Nope, parse a type. This can include multiple field names, all
			// separated by commas before the actual type definition. So scoop
			// up the names first, and then use each one on the list against
			// the type definition.
			fieldNames := make([]string, 1)
			fieldNames[0] = name.Spelling()

			for c.t.IsNext(tokenizer.CommaToken) {
				nextField := c.t.Next()
				if !nextField.IsIdentifier() {
					return &datatypes.UndefinedType, c.newError(errors.ErrInvalidSymbolName)
				}

				fieldNames = append(fieldNames, nextField.Spelling())
			}

			fieldType, err := c.parseType("", false)
			if err != nil {
				return &datatypes.UndefinedType, err
			}

			for _, fieldName := range fieldNames {
				t.DefineField(fieldName, fieldType)
			}

			c.t.IsNext(tokenizer.CommaToken)
		}

		return t, nil
	}

	// Arrays
	if c.t.Peek(1) == tokenizer.StartOfArrayToken && c.t.Peek(2) == tokenizer.EndOfArrayToken {
		c.t.Advance(2)

		valueType, err := c.parseType("", false)
		if err != nil {
			return &datatypes.UndefinedType, err
		}

		return datatypes.Array(valueType), nil
	}

	// Known base types?
	for _, typeDeclaration := range datatypes.TypeDeclarations {
		found = true

		for idx, token := range typeDeclaration.Tokens {
			if c.t.PeekText(1+idx) != token {
				found = false

				break
			}
		}

		if found {
			c.t.Advance(len(typeDeclaration.Tokens))

			return typeDeclaration.Kind, nil
		}
	}

	// User type known to this compilation?
	typeName := c.t.Peek(1)

	if t, found := c.types[typeName.Spelling()]; found {
		c.t.Advance(1)

		return t, nil
	}

	typeNameSpelling := typeName.Spelling()

	if typeName.IsIdentifier() && c.t.Peek(2) == tokenizer.DotToken && c.t.Peek(3).IsIdentifier() {
		packageName := typeName
		typeName = c.t.Peek(3)

		if t, found := c.GetPackageType(packageName.Spelling(), typeName.Spelling()); found {
			c.t.Advance(3)

			return t, nil
		}

		typeNameSpelling = packageName.Spelling() + "." + typeNameSpelling
	}

	return &datatypes.UndefinedType, c.newError(errors.ErrUnknownType, typeNameSpelling)
}

// Embed a given user-defined type's fields in the current type we are compiling.
func embedType(newType *datatypes.Type, embeddedType *datatypes.Type) {
	baseType := embeddedType.BaseType()
	if baseType.Kind() == datatypes.StructKind {
		fieldNames := baseType.FieldNames()
		for _, fieldName := range fieldNames {
			fieldType, _ := baseType.Field(fieldName)
			newType.DefineField(fieldName, fieldType)
		}
	}
}
