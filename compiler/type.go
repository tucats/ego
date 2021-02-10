package compiler

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/tokenizer"
	"github.com/tucats/ego/util"
)

type modelIsType struct{}

// Type compiles a type statement.
func (c *Compiler) Type() error {
	name := c.t.Next()
	if !tokenizer.IsSymbol(name) {
		return c.NewError(errors.InvalidSymbolError)
	}

	name = c.Normalize(name)
	parent := name

	if c.PackageName != "" {
		parent = c.PackageName
	}

	// Make sure this is a legit type definition
	if c.t.Peek(1) == "struct" && c.t.Peek(2) == "{" {
		c.t.Advance(1)
	}

	if c.t.Peek(1) != "{" {
		return c.NewError(errors.MissingBlockError)
	}

	// If there is no parent, seal the chain by making the link point to a string of our own name.
	// If there is a parent, load it so it can be linked after type creation.
	if parent == name {
		c.b.Emit(bytecode.Push, parent)
	} else {
		c.b.Emit(bytecode.Load, parent)
	}

	// Compile a struct definition
	err := c.compileType()
	if err != nil {
		return err
	}

	// Add in the type linkage, and store as the type name. The __parent for a type is
	// a string that is the name of the type. When a member dereference on a struct
	// happens that includes a __parent, the __parent object is also checked for the
	// member if it is NOT a string.
	c.b.Emit(bytecode.Swap)
	c.b.Emit(bytecode.StoreMetadata, datatypes.ParentMDKey)
	c.b.Emit(bytecode.Dup)
	c.b.Emit(bytecode.Dup)

	// Use the name as the type. Note that StoreIndex will intercept the __
	// prefix of the index and redirect it into the metadata.
	c.b.Emit(bytecode.Push, name)
	c.b.Emit(bytecode.StoreMetadata, datatypes.TypeMDKey)

	// Finally, make it a static value now.
	c.b.Emit(bytecode.Dup) // One more needed for type statement
	c.b.Emit(bytecode.Push, true)
	c.b.Emit(bytecode.StoreMetadata, datatypes.StaticMDKey)

	if c.PackageName != "" {
		c.b.Emit(bytecode.Load, c.PackageName)
		c.b.Emit(bytecode.Push, name)
		c.b.Emit(bytecode.StoreIndex, true)
	} else {
		c.b.Emit(bytecode.SymbolCreate, name)
		c.b.Emit(bytecode.Store, name)
	}

	return nil
}

func (c *Compiler) compileType() error {
	// Skip over the optional struct type keyword
	if c.t.Peek(1) == "struct" && c.t.Peek(2) == "{" {
		c.t.Advance(1)
	}

	// Must start with {
	if !c.t.IsNext("{") {
		return c.NewError(errors.MissingBlockError)
	}

	count := 0

	for {
		name := c.t.Next()
		if !tokenizer.IsSymbol(name) {
			return c.NewError(errors.InvalidSymbolError, name)
		}

		name = c.Normalize(name)
		count = count + 1

		// Skip over the optional struct type keyword
		if c.t.Peek(1) == "struct" && c.t.Peek(2) == "{" {
			c.t.Advance(1)
		}

		if c.t.Peek(1) == "{" {
			err := c.compileType()
			if err != nil {
				return err
			}
		} else {
			model, err := c.typeDeclaration()
			if err != nil {
				return err
			}

			if _, ok := model.(modelIsType); !ok {
				c.b.Emit(bytecode.Push, model)
			}
		}

		c.b.Emit(bytecode.Push, name)

		// Eat any trailing commas, and the see if we're at the end
		_ = c.t.IsNext(",")

		if c.t.IsNext("}") {
			c.b.Emit(bytecode.Struct, count)

			return nil
		}

		if c.t.AtEnd() {
			return c.NewError(errors.MissingEndOfBlockError)
		}
	}
}

func (c *Compiler) typeDeclaration() (interface{}, error) {
	if c.t.Peek(1) == "struct" && c.t.Peek(2) == "{" {
		return nil, c.compileType()
	}

	for _, typeDef := range datatypes.TypeDeclarationMap {
		found := true

		for offset, token := range typeDef.Tokens {
			if c.t.Peek(1+offset) != token {
				found = false

				break
			}
		}

		if found {
			c.t.Advance(len(typeDef.Tokens))

			return typeDef.Model, nil
		}
	}

	// Not a known type, let's see if it's a user type initialzer.
	t := c.Normalize(c.t.Next())
	if !tokenizer.IsSymbol(t) {
		return nil, c.NewError(errors.InvalidTypeNameError, t)
	}

	// Is it a generator for a type?
	if c.t.Peek(1) == "{" && tokenizer.IsSymbol(c.t.Peek(2)) && c.t.Peek(3) == ":" {
		c.b.Emit(bytecode.Load, t)
		c.b.Emit(bytecode.Push, "__type")
		c.b.Emit(bytecode.LoadIndex)
		c.b.Emit(bytecode.Push, "__type")

		err := c.expressionAtom()
		if err != nil {
			return nil, err
		}

		i := c.b.Opcodes()
		ix := i[len(i)-1]
		ix.Operand = util.GetInt(ix.Operand) + 1
		i[len(i)-1] = ix

		return modelIsType{}, nil
	}

	if c.t.IsNext("{}") {
		c.b.Emit(bytecode.Load, "new")
		c.b.Emit(bytecode.Load, t)
		c.b.Emit(bytecode.Call, 1)
	}

	// Let's hope its a type name and see how it goes at runtime.
	c.b.Emit(bytecode.Load, "new")
	c.b.Emit(bytecode.Load, t)
	c.b.Emit(bytecode.Call, 1)

	return modelIsType{}, nil
}
