package compiler

import (
	"fmt"

	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/tokenizer"
)

// compileTypeDefinition compiles a type statement which creates
// a user-defined type specification.
func (c *Compiler) compileTypeDefinition() error {
	if c.t.AnyNext(tokenizer.SemicolonToken, tokenizer.EndOfTokens) {
		return c.error(errors.ErrMissingType)
	}

	name := c.t.Next()
	if !name.IsIdentifier() {
		return c.error(errors.ErrInvalidSymbolName)
	}

	name = c.normalizeToken(name)

	if c.t.AnyNext(tokenizer.SemicolonToken, tokenizer.EndOfTokens) {
		return c.error(errors.ErrMissingType)
	}

	return c.typeEmitter(name.Spelling())
}

// Parses a token stream for a generic type declaration.
func (c *Compiler) typeDeclaration() (interface{}, error) {
	theType, err := c.parseType("", false)
	if err != nil {
		return nil, err
	}

	return data.InstanceOfType(theType), nil
}

func (c *Compiler) parseTypeSpec() (*data.Type, error) {
	if c.t.Peek(1) == tokenizer.PointerToken {
		c.t.Advance(1)
		t, err := c.parseTypeSpec()

		return data.PointerType(t), err
	}

	if c.t.Peek(1) == tokenizer.StartOfArrayToken && c.t.Peek(2) == tokenizer.EndOfArrayToken {
		c.t.Advance(2)
		t, err := c.parseTypeSpec()

		return data.ArrayType(t), err
	}

	if c.t.Peek(1) == tokenizer.MapToken && c.t.Peek(2) == tokenizer.StartOfArrayToken {
		c.t.Advance(2)

		keyType, err := c.parseTypeSpec()
		if err != nil {
			return &data.UndefinedType, err
		}

		c.t.IsNext(tokenizer.EndOfArrayToken)

		valueType, err := c.parseTypeSpec()
		if err != nil {
			return &data.UndefinedType, err
		}

		return data.MapType(keyType, valueType), nil
	}

	for _, typeDef := range data.TypeDeclarations {
		found := true

		if c.t.PeekText(1) == "interface" {
			fmt.Println("DEBUG")
		}

		for pos, token := range typeDef.Tokens {
			eval := c.t.Peek(1 + pos)
			if eval.Spelling() != token {
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
	if typeDef, ok := c.types[typeName.Spelling()]; ok {
		c.t.Advance(1)

		return typeDef, nil
	}

	return &data.UndefinedType, nil
}

// Given a string expression of a type specification, compile it asn return the
// type it represents, and an optional error if it was incorrectly formed. This
// cannot reference user types as they are not visible to this function.
//
// If the string starts with the keyword `type` followed by a type name, then
// the resulting value is a type definition of the given name.
func CompileTypeSpec(source string) (*data.Type, error) {
	typeCompiler := New("type compiler")
	typeCompiler.t = tokenizer.New(source)
	nameSpelling := ""

	// Does it have a type <name> prefix? And is that a package.name style name?
	if typeCompiler.t.IsNext(tokenizer.TypeToken) {
		name := typeCompiler.t.Next()
		if !name.IsIdentifier() {
			return &data.UndefinedType, errors.ErrInvalidSymbolName.Context(name)
		}

		nameSpelling = name.Spelling()

		if typeCompiler.t.IsNext(tokenizer.DotToken) {
			name2 := typeCompiler.t.Next()
			if !name2.IsIdentifier() {
				return &data.UndefinedType, errors.ErrInvalidSymbolName.Context(name2)
			}

			nameSpelling = nameSpelling + "." + name2.Spelling()
		}
	}

	t, err := typeCompiler.parseType("", true)
	if err == nil && nameSpelling != "" {
		t = data.TypeDefinition(nameSpelling, t)
	}

	return t, err
}

// For a given package and type name, get the underlying type.
func (c *Compiler) GetPackageType(packageName, typeName string) (*data.Type, bool) {
	if p, found := c.packages[packageName]; found {
		if t, found := p.Get(typeName); found {
			if theType, ok := t.(*data.Type); ok {
				return theType, true
			}
		}

		// It was a package, but without a package body. Already moved to global storage?
		if pkg, found := c.s.Root().Get(packageName); found {
			if m, ok := pkg.(*data.Package); ok {
				if t, found := m.Get(typeName); found {
					if theType, ok := t.(*data.Type); ok {
						return theType, true
					}
				}

				if t, found := m.Get(data.TypeMDKey); found {
					if theType, ok := t.(*data.Type); ok {
						return theType.BaseType(), true
					}
				}
			}
		}
	}

	// Is it a previously imported package type?
	if bytecode.IsPackage(packageName) {
		p, _ := bytecode.GetPackage(packageName)
		if tV, ok := p.Get(typeName); ok {
			if t, ok := tV.(*data.Type); ok {
				return t, true
			}
		}
	}

	return nil, false
}
