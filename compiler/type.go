package compiler

import (
	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/tokenizer"
)

// compileTypeDefinition compiles a type statement which creates
// a user-defined type specification.
func (c *Compiler) compileTypeDefinition() *errors.EgoError {
	name := c.t.Next()
	if !tokenizer.IsSymbol(name) {
		return c.newError(errors.InvalidSymbolError)
	}

	name = c.normalize(name)

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

	return datatypes.UndefinedType, nil
}
