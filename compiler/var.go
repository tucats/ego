package compiler

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/tokenizer"
)

// Var compiles the var statement.
func (c *Compiler) Var() *errors.EgoError {
	names := []string{}

	for {
		name := c.t.Next()
		if !tokenizer.IsSymbol(name) {
			c.t.Advance(-1)

			return c.NewError(errors.InvalidSymbolError, name)
		}

		// See if it's a reserved word.
		if tokenizer.IsReserved(name, c.extensionsEnabled) {
			c.t.Advance(-1)
			// If we mid-list, then just done with list
			if len(names) > 0 {
				break
			}

			return c.NewError(errors.InvalidSymbolError, name)
		}

		name = c.Normalize(name)
		names = append(names, name)

		if !c.t.IsNext(",") {
			break
		}
	}

	// We'll need to use this token string over and over for each name
	// in the list, so remember where to start.
	kind := datatypes.UndefinedType

	var model interface{}

	for _, typeInfo := range datatypes.TypeDeclarationMap {
		match := true

		for idx, token := range typeInfo.Tokens {
			if c.t.Peek(idx+1) != token {
				match = false

				break
			}
		}

		if match {
			kind = typeInfo.Kind
			model = typeInfo.Model
			c.t.Advance(len(typeInfo.Tokens))
		}
	}

	if kind == datatypes.UndefinedType {
		// Is the next item a symbol? If so, assume it's a user
		// defined type
		typeName := c.t.Next()
		if tokenizer.IsSymbol(typeName) {
			for _, name := range names {
				c.b.Emit(bytecode.Load, "new")
				c.b.Emit(bytecode.Load, typeName)
				c.b.Emit(bytecode.Call, 1)
				c.b.Emit(bytecode.SymbolCreate, name)
				c.b.Emit(bytecode.Store, name)
			}

			return nil
		}

		// Not a symbol name, so fail
		return c.NewError(errors.InvalidTypeSpecError)
	}

	// We got a built-in type, so emit the model and store it
	// in each symbo
	for _, name := range names {
		c.b.Emit(bytecode.Push, model)
		c.b.Emit(bytecode.SymbolCreate, name)
		c.b.Emit(bytecode.Store, name)
	}

	return nil
}
