package compiler

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/packages"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/tokenizer"
)

func (c *Compiler) typeEmitter(name string) error {
	typeInfo, err := c.typeCompiler(name)
	if err == nil {
		if typeInfo.Kind() == data.TypeKind {
			typeInfo.SetPackage(c.activePackageName)
		}

		c.b.Emit(bytecode.Push, typeInfo)
		c.b.Emit(bytecode.StoreAlways, name)
	}

	return err
}

func (c *Compiler) typeCompiler(name string) (*data.Type, error) {
	// Is there a dot in the name? If so, it's a qualified type reference from a package.
	if c.t.IsNext(tokenizer.DotToken) {
		packageName := name
		name = c.t.Next().Spelling()

		// Is there a package of this name? If so, see if this is a predefined type
		if pkg := packages.Get(packageName); pkg != nil {
			if typeV, found := pkg.Get(name); found {
				if t, ok := typeV.(*data.Type); ok {
					return t, nil
				}
			}

			// Not a predefined type, so check to see if the type was created during
			// source import for the package and placed in the package symbol table.
			s := symbols.GetPackageSymbolTable(packageName)
			if typeV, found := s.Get(name); found {
				if t, ok := typeV.(*data.Type); ok {
					return t, nil
				}
			}
		}

		// Nope, not a valid package type reference.
		return nil, c.compileError(errors.ErrUnknownType).Context(packageName + "." + name)
	}

	if _, found := c.types[name]; found {
		return data.UndefinedType, c.compileError(errors.ErrDuplicateTypeName).Context(name)
	}

	baseType, err := c.parseType(name, false)
	if err != nil {
		return data.UndefinedType, err
	}

	typeInfo := data.TypeDefinition(name, baseType).SetPackage(c.activePackageName)
	c.types[name] = typeInfo
	c.DefineSymbol(name)

	return typeInfo, nil
}

func (c *Compiler) parseType(name string, anonymous bool) (*data.Type, error) {
	isPointer := c.t.IsNext(tokenizer.PointerToken)

	result := c.previouslyDefinedType(anonymous, isPointer)
	if result != nil {
		return result, nil
	}

	// Is it a known complex type?

	// Base error type
	if c.t.Peek(1).Is(tokenizer.ErrorToken) {
		c.t.Advance(1)

		if isPointer {
			return data.PointerType(data.ErrorType), nil
		}

		return data.ErrorType, nil
	}

	// Empty interface
	if c.t.Peek(1).Is(tokenizer.EmptyInterfaceToken) {
		c.t.Advance(1)

		return data.InterfaceType, nil
	}

	if c.t.Peek(1).Is(tokenizer.InterfaceToken) && c.t.Peek(2).Is(tokenizer.EmptyInitializerToken) {
		c.t.Advance(2)

		return data.InterfaceType, nil
	}

	// Function type
	if c.t.Peek(1).Is(tokenizer.FuncToken) {
		c.t.Advance(1)

		f, err := c.ParseFunctionDeclaration(true)
		if err != nil {
			return data.UndefinedType, err
		}

		return data.FunctionType(&data.Function{Declaration: f}), nil
	}

	// Interfaces
	if c.t.Peek(1).Is(tokenizer.InterfaceToken) && c.t.Peek(2).Is(tokenizer.DataBeginToken) {
		// Parse function declarations, add to the type object.
		return c.parseInterface(name)
	}

	// Maps
	if c.t.Peek(1).Is(tokenizer.MapToken) && c.t.Peek(2).Is(tokenizer.StartOfArrayToken) {
		return c.parseMapType(isPointer)
	}

	// Structures
	if c.t.Peek(1).Is(tokenizer.StructToken) && c.t.Peek(2).Is(tokenizer.DataBeginToken) {
		return c.parseStructType(isPointer)
	}

	// Arrays
	if c.t.Peek(1).Is(tokenizer.StartOfArrayToken) && c.t.Peek(2).Is(tokenizer.EndOfArrayToken) {
		return c.parseArrayType(isPointer)
	}

	// Known base types?
	result, err := c.compileKnownBaseType(isPointer)
	if err != nil || result != nil {
		return result, err
	}

	// User type known to this compilation?
	typeName := c.t.Peek(1)

	if t, found := c.types[typeName.Spelling()]; found {
		c.t.Advance(1)

		if isPointer {
			t = data.PointerType(t)
		}

		return t, nil
	}

	// Is it a compound type name referenced in a package
	typeNameSpelling := typeName.Spelling()

	if typeName.IsIdentifier() && c.t.Peek(2).Is(tokenizer.DotToken) && c.t.Peek(3).IsIdentifier() {
		packageName := typeName
		typeName = c.t.Peek(3)

		if packageType := c.isPackageType(packageName, typeName, isPointer); packageType != nil {
			return packageType, nil
		}

		typeNameSpelling = packageName.Spelling() + "." + typeNameSpelling
	}

	// No idea what this is, complain
	return data.UndefinedType, c.compileError(errors.ErrUnknownType, typeNameSpelling)
}

func (c *Compiler) previouslyDefinedType(anonymous bool, isPointer bool) *data.Type {
	if !anonymous {
		// Is it a previously defined type?
		typeName := c.t.Peek(1)
		if typeName.IsIdentifier() {
			if t, ok := c.types[typeName.Spelling()]; ok {
				c.t.Advance(1)

				if isPointer {
					t = data.PointerType(t)
				}

				c.ReferenceSymbol(typeName.Spelling())

				return t
			}
		}
	}

	return nil
}

func (c *Compiler) isPackageType(packageName tokenizer.Token, typeName tokenizer.Token, isPointer bool) *data.Type {
	if t := c.GetPackageType(packageName.Spelling(), typeName.Spelling()); t != nil {
		c.t.Advance(3)

		if isPointer {
			t = data.PointerType(t)
		}

		return t
	}

	return nil
}

func (c *Compiler) compileKnownBaseType(isPointer bool) (*data.Type, error) {
	found := false
	for _, typeDeclaration := range data.TypeDeclarations {
		found = true

		for idx, token := range typeDeclaration.Tokens {
			if c.t.PeekText(1+idx) != token {
				found = false

				break
			}
		}

		if found {
			c.t.Advance(len(typeDeclaration.Tokens))
			t := typeDeclaration.Kind

			if isPointer {
				t = data.PointerType(t)
			}

			return t, nil
		}
	}

	return nil, nil
}

func (c *Compiler) parseArrayType(isPointer bool) (*data.Type, error) {
	c.t.Advance(2)

	valueType, err := c.parseType("", false)
	if err != nil {
		return data.UndefinedType, err
	}

	t := data.ArrayType(valueType)
	if isPointer {
		t = data.PointerType(t)
	}

	return t, nil
}

func (c *Compiler) parseStructType(isPointer bool) (*data.Type, error) {
	t := data.StructureType()
	c.t.Advance(2)

	result, err := c.parseStructFieldTypes(t)
	if err != nil {
		return result, c.compileError(err)
	}

	if isPointer {
		t = data.PointerType(t)
	}

	return t, nil
}

func (c *Compiler) parseStructFieldTypes(t *data.Type) (*data.Type, error) {
	for !c.t.IsNext(tokenizer.DataEndToken) {
		_ = c.t.IsNext(tokenizer.SemicolonToken)

		if c.t.IsNext(tokenizer.DataEndToken) {
			break
		}

		name := c.t.Next()

		if !name.IsIdentifier() {
			return data.UndefinedType, c.compileError(errors.ErrInvalidSymbolName).Context(name)
		}

		// Is it a compound name? Could be a package reference to an embedded type.
		if c.t.Peek(1).Is(tokenizer.DotToken) && c.t.Peek(2).IsIdentifier() {
			packageName := name
			name := c.t.Peek(2)

			// Is it a package name? If so, convert it to an actual package type and
			// look to see if this is a known type. If so, copy the embedded fields to
			// the newly created type we're working on.
			if c.parseEmbeddedPackageTypeReference(packageName, name, t) {
				continue
			}
		}

		// Skip past the tokens and any optional trailing comma
		if typeData, found := c.types[name.Spelling()]; found {
			embedType(t, typeData)
			c.t.IsNext(tokenizer.CommaToken)

			if err := c.ReferenceSymbol(name.Spelling()); err != nil {
				return data.UndefinedType, err
			}

			continue
		}

		fieldNames := make([]string, 1)
		fieldNames[0] = name.Spelling()

		for c.t.IsNext(tokenizer.CommaToken) {
			nextField := c.t.Next()
			// Is the name actually a type that we embed? If so, get the base type and iterate
			// over its fields, copying them into our current structure definition.
			if !nextField.IsIdentifier() {
				return data.UndefinedType, c.compileError(errors.ErrInvalidSymbolName)
			}

			fieldNames = append(fieldNames, nextField.Spelling())
		}

		// Nope, parse a type. This can include multiple field names, all
		// separated by commas before the actual type definition. So scoop
		// up the names first, and then use each one on the list against
		// the type definition.
		fieldType, err := c.parseType("", false)
		if err != nil {
			return data.UndefinedType, err
		}

		for _, fieldName := range fieldNames {
			t.DefineField(fieldName, fieldType)
		}

		c.t.IsNext(tokenizer.CommaToken)
	}

	return nil, nil
}

// parseEmbeddedPackageTypeReference looks to see if a type reference to a package type has been given
// that should be embedded into the current structure. If an embedded type reference was found,
// returns true.
func (c *Compiler) parseEmbeddedPackageTypeReference(packageName tokenizer.Token, name tokenizer.Token, t *data.Type) bool {
	if pkgData, found := c.Symbols().Get(packageName.Spelling()); found {
		if pkg, ok := pkgData.(*data.Package); ok {
			if typeInterface, ok := pkg.Get(name.Spelling()); ok {
				if typeData, ok := typeInterface.(*data.Type); ok {
					embedType(t, typeData)

					c.t.Advance(2)
					c.t.IsNext(tokenizer.CommaToken)

					return true
				}
			}
		}
	}

	return false
}

func (c *Compiler) parseMapType(isPointer bool) (*data.Type, error) {
	c.t.Advance(2)

	keyType, err := c.parseType("", false)
	if err != nil {
		return data.UndefinedType, err
	}

	if !c.t.IsNext(tokenizer.EndOfArrayToken) {
		return data.UndefinedType, c.compileError(errors.ErrMissingBracket)
	}

	valueType, err := c.parseType("", false)
	if err != nil {
		return data.UndefinedType, err
	}

	t := data.MapType(keyType, valueType)
	if isPointer {
		t = data.PointerType(t)
	}

	return t, nil
}

func (c *Compiler) parseInterface(name string) (*data.Type, error) {
	c.t.Advance(2)

	t := data.NewInterfaceType(name)

	for !c.t.IsNext(tokenizer.DataEndToken) {
		for c.t.IsNext(tokenizer.SemicolonToken) {
		}

		if c.t.IsNext(tokenizer.DataEndToken) {
			break
		}

		f, err := c.ParseFunctionDeclaration(false)
		if err != nil {
			return data.UndefinedType, err
		}

		t.DefineFunction(f.Name, f, nil)
	}

	return t, nil
}

// Embed a given user-defined type's fields in the current type we are compiling.
func embedType(newType *data.Type, embeddedType *data.Type) {
	baseType := embeddedType.BaseType()
	if baseType.Kind() == data.StructKind {
		fieldNames := baseType.FieldNames()
		for _, fieldName := range fieldNames {
			fieldType, _ := baseType.Field(fieldName)
			newType.DefineField(fieldName, fieldType)
		}
	}
}
