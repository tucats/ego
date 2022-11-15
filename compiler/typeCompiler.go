package compiler

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/tokenizer"
)

func (c *Compiler) typeEmitter(name string) *errors.EgoError {
	typeInfo, err := c.typeCompiler(name)
	if err == nil {
		c.b.Emit(bytecode.Push, typeInfo)
		c.b.Emit(bytecode.StoreAlways, name)
	}

	return err
}

func (c *Compiler) typeCompiler(name string) (datatypes.Type, *errors.EgoError) {
	if _, found := c.Types[name]; found {
		return datatypes.UndefinedType, c.newError(errors.ErrDuplicateTypeName).Context(name)
	}

	baseType, err := c.parseType(name, false)
	if err != nil {
		return datatypes.UndefinedType, err
	}

	typeInfo := datatypes.TypeDefinition(name, baseType)
	c.Types[name] = typeInfo

	return typeInfo, nil
}

func (c *Compiler) parseType(name string, anonymous bool) (datatypes.Type, *errors.EgoError) {
	found := false

	if !anonymous {
		// Is it a previously defined type?
		typeName := c.t.Peek(1)
		if tokenizer.IsSymbol(typeName) {
			if t, ok := c.Types[typeName]; ok {
				c.t.Advance(1)

				return t, nil
			}
		}
	}

	// Is it a known complex type?

	// Empty interface
	if c.t.Peek(1) == "interface" && c.t.Peek(2) == "{}" {
		c.t.Advance(2)

		t := datatypes.InterfaceType

		return t, nil
	}

	// Interfaces
	if c.t.Peek(1) == "interface" && c.t.Peek(2) == "{" {
		c.t.Advance(2)

		t := datatypes.NewInterfaceType(name)

		// Parse function declarations, add to the type object.
		for !c.t.IsNext("}") {
			f, err := c.parseFunctionDeclaration()
			if !errors.Nil(err) {
				return datatypes.UndefinedType, err
			}

			t.DefineFunction(f.Name, f)
		}

		return *t, nil
	}

	// Maps
	if c.t.Peek(1) == "map" && c.t.Peek(2) == "[" {
		c.t.Advance(2)

		keyType, err := c.parseType("", false)
		if err != nil {
			return datatypes.UndefinedType, err
		}

		if !c.t.IsNext("]") {
			return datatypes.UndefinedType, c.newError(errors.ErrMissingBracket)
		}

		valueType, err := c.parseType("", false)
		if err != nil {
			return datatypes.UndefinedType, err
		}

		return datatypes.Map(keyType, valueType), nil
	}

	// Structures
	if c.t.Peek(1) == "struct" && c.t.Peek(2) == "{" {
		t := datatypes.Structure()
		c.t.Advance(2)

		for !c.t.IsNext("}") {
			name := c.t.Next()
			if !tokenizer.IsSymbol(name) {
				return datatypes.UndefinedType, c.newError(errors.ErrInvalidSymbolName)
			}

			// Is it a compound name? Could be a package reference to an embedded type.
			if c.t.Peek(1) == "." && tokenizer.IsSymbol(c.t.Peek(2)) {
				packageName := name
				name := c.t.Peek(2)

				// Is it a package name? If so, convert it to an actual package type and
				// look to see if this is a known type. If so, copy the embedded fields to
				// the newly created type we're working on.
				if pkgData, found := c.Symbols().Get(packageName); found {
					if pkg, ok := pkgData.(datatypes.EgoPackage); ok {
						if typeInterface, ok := pkg.Get(name); ok {
							if typeData, ok := typeInterface.(datatypes.Type); ok {
								embedType(&t, typeData)

								// Skip past the tokens and any optional trailing comma
								c.t.Advance(2)
								c.t.IsNext(",")

								continue
							}
						}
					}
				}
			}

			// Is the name actually a type that we embed? If so, get the base type and iterate
			// over its fields, copying them into our current structure definition.
			if typeData, found := c.Types[name]; found {
				embedType(&t, typeData)
				c.t.IsNext(",")

				continue
			}

			// Nope, parse a type.
			fieldType, err := c.parseType("", false)
			if err != nil {
				return datatypes.UndefinedType, err
			}

			t.DefineField(name, fieldType)

			c.t.IsNext(",")
		}

		return t, nil
	}

	// Arrays
	if c.t.Peek(1) == "[" && c.t.Peek(2) == "]" {
		c.t.Advance(2)

		valueType, err := c.parseType("", false)
		if err != nil {
			return datatypes.UndefinedType, err
		}

		return datatypes.Array(valueType), nil
	}

	// Known base types?
	for _, typeDeclaration := range datatypes.TypeDeclarations {
		found = true

		for idx, token := range typeDeclaration.Tokens {
			if c.t.Peek(1+idx) != token {
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
	if t, found := c.Types[typeName]; found {
		c.t.Advance(1)

		return t, nil
	}

	if tokenizer.IsSymbol(typeName) && c.t.Peek(2) == "." && tokenizer.IsSymbol(c.t.Peek(3)) {
		packageName := typeName
		typeName = c.t.Peek(3)

		if t, found := c.GetPackageType(packageName, typeName); found {
			c.t.Advance(3)

			return *t, nil
		}
	}

	return datatypes.UndefinedType, c.newError(errors.ErrInvalidType)
}

// Embed a given user-defined type's fields in the current type we are compiling.
func embedType(newType *datatypes.Type, embeddedType datatypes.Type) {
	baseType := embeddedType.BaseType()
	if baseType.Kind() == datatypes.StructKind {
		fieldNames := baseType.FieldNames()
		for _, fieldName := range fieldNames {
			fieldType, _ := baseType.Field(fieldName)
			newType.DefineField(fieldName, fieldType)
		}
	}
}
