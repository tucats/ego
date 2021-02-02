package compiler

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/datatypes"
)

// Map compiles a map type declaration

func (c *Compiler) Map() error {
	if !c.t.IsNext("map") {
		return c.NewError(UnexpectedTokenError, c.t.Peek(1))
	}
	if !c.t.IsNext("[") {
		return c.NewError(MissingBracketError)
	}

	// Parse the key type
	keyType := c.ParseType()

	// Closing bracket on the key type
	if !c.t.IsNext("]") {
		return c.NewError(MissingBracketError)
	}

	// Parse the object type
	valueType := c.ParseType()

	// Make a suitable map object
	m := datatypes.NewMap(keyType, valueType)

	// Eat any {} initializer stuff; not really supported yet
	_ = c.t.IsNext("{}")

	// Push it on the stack.
	c.b.Emit(bytecode.Push, m)

	return nil
}

func (c *Compiler) ParseType() int {
	for _, typeDef := range datatypes.TypeDeclarationMap {
		found := true
		for pos, token := range typeDef.Tokens {
			if c.t.Peek(1+pos) != token {
				found = false
			}
		}
		if found {
			c.t.Advance(len(typeDef.Tokens))
			return typeDef.Kind
		}
	}

	return datatypes.UndefinedType
}
