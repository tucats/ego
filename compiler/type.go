package compiler

import (
	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/tokenizer"
)

// compileTypeDefinition compiles a type statement which creates
// a user-defined type specification.
func (c *Compiler) compileTypeDefinition() *errors.EgoError {
	if c.t.AnyNext(";", tokenizer.EndOfTokens) {
		return c.newError(errors.ErrMissingType)
	}

	name := c.t.Next()
	if !tokenizer.IsSymbol(name) {
		return c.newError(errors.ErrInvalidSymbolName)
	}

	name = c.normalize(name)

	if c.t.AnyNext(";", tokenizer.EndOfTokens) {
		return c.newError(errors.ErrMissingType)
	}

	return c.typeEmitter(name)
}

// Parses a token stream for a generic type declaration.
func (c *Compiler) typeDeclaration() (interface{}, *errors.EgoError) {
	theType, err := c.parseType(false)
	if !errors.Nil(err) {
		return nil, err
	}

	return datatypes.InstanceOfType(theType), nil
}

func (c *Compiler) parseTypeSpec() (datatypes.Type, *errors.EgoError) {
	if c.t.Peek(1) == "*" {
		c.t.Advance(1)
		t, err := c.parseTypeSpec()

		return datatypes.Pointer(t), err
	}

	if c.t.Peek(1) == "[" && c.t.Peek(2) == "]" {
		c.t.Advance(2)
		t, err := c.parseTypeSpec()

		return datatypes.Array(t), err
	}

	if c.t.Peek(1) == "map" && c.t.Peek(2) == "[" {
		c.t.Advance(2)

		keyType, err := c.parseTypeSpec()
		if err != nil {
			return datatypes.UndefinedType, err
		}

		c.t.IsNext("]")

		valueType, err := c.parseTypeSpec()
		if err != nil {
			return datatypes.UndefinedType, err
		}

		return datatypes.Map(keyType, valueType), nil
	}

	for _, typeDef := range datatypes.TypeDeclarations {
		found := true

		for pos, token := range typeDef.Tokens {
			eval := c.t.Peek(1 + pos)
			if eval != token {
				found = false
			}
		}

		if found {
			c.t.Advance(len(typeDef.Tokens))

			return typeDef.Kind, nil
		}
	}

	// Is it a type we already know about?
	typeName := c.t.Peek(1)
	if typeDef, ok := c.Types[typeName]; ok {
		c.t.Advance(1)

		return typeDef, nil
	}

	return datatypes.UndefinedType, nil
}

// Given a string expression of a type specification, compile it asn return the
// type it represents, and an optional error if it was incorrectly formed. This
// cannot reference user types as they are not visible to this function.
//
// If the string starts with the keyword `type` followed by a type name, then
// the resulting value is a type definition of the given name.
func CompileTypeSpec(source string) (datatypes.Type, *errors.EgoError) {
	typeCompiler := New("type compiler")
	typeCompiler.t = tokenizer.New(source)
	name := ""

	// Does it have a type <name> prefix? And is that a package.name style name?
	if typeCompiler.t.IsNext("type") {
		name = typeCompiler.t.Next()
		if !tokenizer.IsSymbol(name) {
			return datatypes.Type{}, errors.New(errors.ErrInvalidSymbolName).Context(name)
		}

		if typeCompiler.t.IsNext(".") {
			name2 := typeCompiler.t.Next()
			if !tokenizer.IsSymbol(name2) {
				return datatypes.Type{}, errors.New(errors.ErrInvalidSymbolName).Context(name2)
			}

			name = name + "." + name2
		}
	}

	t, err := typeCompiler.parseType(true)
	if errors.Nil(err) && name != "" {
		t = datatypes.TypeDefinition(name, t)
	}

	return t, err
}

// For a given package and type name, get the underlying type.
func (c *Compiler) GetPackageType(packageName, typeName string) (*datatypes.Type, bool) {
	if p, found := c.packages.Package[packageName]; found {
		if t, found := p.Get(typeName); found {
			if theType, ok := t.(*datatypes.Type); ok {
				return theType, true
			}
		}

		// It was a package, but without a package body. Already moved to global storage?
		if pkg, found := c.s.Root().Get(packageName); found {
			if m, ok := pkg.(datatypes.EgoPackage); ok {
				if t, found := m.Get(typeName); found {
					if theType, ok := t.(datatypes.Type); ok {
						return &theType, true
					}
				}

				if t, found := m.Get(datatypes.TypeMDKey); found {
					if theType, ok := t.(datatypes.Type); ok {
						return theType.BaseType(), true
					}
				}
			}
		}
	}

	return nil, false
}
