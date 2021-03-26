package compiler

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/tokenizer"
	"github.com/tucats/ego/util"
)

type modelIsType struct{}

var modernTypes = true

// compileTypeDefinition compiles a type statement which creates
// a user-defined type specification.
func (c *Compiler) compileTypeDefinition() *errors.EgoError {
	name := c.t.Next()
	if !tokenizer.IsSymbol(name) {
		return c.newError(errors.InvalidSymbolError)
	}

	name = c.normalize(name)
	parent := name

	if c.PackageName != "" {
		parent = c.PackageName
	}

	if modernTypes {
		return c.typeEmitter(name)
	}
	// Make sure this is a legit type definition
	if c.t.Peek(1) == "struct" && c.t.Peek(2) == "{" {
		c.t.Advance(1)
	}

	if c.t.Peek(1) != "{" {
		return c.newError(errors.MissingBlockError)
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
	if !errors.Nil(err) {
		return err
	}

	// Indicate the type name and that this is a type object
	// (as opposed to an instance object)
	c.b.Emit(bytecode.Push, datatypes.UserType(name, datatypes.StructType))
	c.b.Emit(bytecode.StoreMetadata, datatypes.TypeMDKey)

	c.b.Emit(bytecode.Push, true)
	c.b.Emit(bytecode.StoreMetadata, datatypes.IsTypeMDKey)

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

// Compile a specific type definition.
func (c *Compiler) compileType() *errors.EgoError {
	// Skip over the optional struct type keyword
	if c.t.Peek(1) == "struct" && c.t.Peek(2) == "{" {
		c.t.Advance(1)
	}

	// Must start with {
	if !c.t.IsNext("{") {
		return c.newError(errors.MissingBlockError)
	}

	count := 0

	for {
		name := c.t.Next()
		if !tokenizer.IsSymbol(name) {
			return c.newError(errors.InvalidSymbolError, name)
		}

		name = c.normalize(name)
		count = count + 1

		// Skip over the optional struct type keyword
		if c.t.Peek(1) == "struct" && c.t.Peek(2) == "{" {
			c.t.Advance(1)
		}

		if c.t.Peek(1) == "{" {
			err := c.compileType()
			if !errors.Nil(err) {
				return err
			}
		} else {
			model, err := c.typeDeclaration()
			if !errors.Nil(err) {
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
			return c.newError(errors.MissingEndOfBlockError)
		}
	}
}

// Parses a token stream for a generic type declaration.
func (c *Compiler) typeDeclaration() (interface{}, *errors.EgoError) {
	if c.t.Peek(1) == "struct" && c.t.Peek(2) == "{" {
		return nil, c.compileType()
	}

	for _, typeDef := range datatypes.TypeDeclarations {
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

	// Not a known type, let's see if it's a user type initializer.
	t := c.normalize(c.t.Next())
	if !tokenizer.IsSymbol(t) {
		return nil, c.newError(errors.InvalidTypeNameError, t)
	}

	// Is it a generator for a type?
	if c.t.Peek(1) == "{" && tokenizer.IsSymbol(c.t.Peek(2)) && c.t.Peek(3) == ":" {
		c.b.Emit(bytecode.Load, t)
		c.b.Emit(bytecode.LoadIndex, "__type")
		c.b.Emit(bytecode.Push, "__type")

		err := c.expressionAtom()
		if !errors.Nil(err) {
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

func (c *Compiler) parseTypeSpec() (datatypes.Type, *errors.EgoError) {
	if c.t.Peek(1) == "*" {
		c.t.Advance(1)
		t, err := c.parseTypeSpec()

		return datatypes.PointerToType(t), err
	}

	if c.t.Peek(1) == "[" && c.t.Peek(2) == "]" {
		c.t.Advance(2)
		t, err := c.parseTypeSpec()

		return datatypes.ArrayOfType(t), err
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

		return datatypes.MapOfType(keyType, valueType), nil
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
