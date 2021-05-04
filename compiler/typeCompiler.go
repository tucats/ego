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

	baseType, err := c.parseType(false)
	if err != nil {
		return datatypes.UndefinedType, err
	}

	typeInfo := datatypes.TypeDefinition(name, baseType)
	c.Types[name] = typeInfo

	return typeInfo, nil
}

func (c *Compiler) parseType(anonymous bool) (datatypes.Type, *errors.EgoError) {
	found := false
	name := ""

	if !anonymous {
		// Is it a previously defined type?
		name = c.t.Peek(1)
		if tokenizer.IsSymbol(name) {
			if t, ok := c.Types[name]; ok {
				c.t.Advance(1)

				return t, nil
			}
		}
	}

	// Is it a known complex type?

	// Maps
	if c.t.Peek(1) == "map" && c.t.Peek(2) == "[" {
		c.t.Advance(2)

		keyType, err := c.parseType(false)
		if err != nil {
			return datatypes.UndefinedType, err
		}

		if !c.t.IsNext("]") {
			return datatypes.UndefinedType, c.newError(errors.ErrMissingBracket)
		}

		valueType, err := c.parseType(false)
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

			fieldType, err := c.parseType(false)
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

		valueType, err := c.parseType(false)
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
