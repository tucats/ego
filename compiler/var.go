package compiler

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/tokenizer"
)

// compileVar compiles the var statement.
func (c *Compiler) compileVar() *errors.EgoError {
	names := []string{}

	for {
		name := c.t.Next()
		if name == tokenizer.EndOfTokens {
			if len(names) > 0 {
				break
			}

			return c.newError(errors.ErrMissingSymbol)
		}

		if !tokenizer.IsSymbol(name) {
			c.t.Advance(-1)

			return c.newError(errors.ErrInvalidSymbolName, name)
		}

		// See if it's a reserved word.
		if tokenizer.IsReserved(name, c.extensionsEnabled) {
			c.t.Advance(-1)
			// If we mid-list, then just done with list
			if len(names) > 0 {
				break
			}

			return c.newError(errors.ErrInvalidSymbolName, name)
		}

		name = c.normalize(name)
		names = append(names, name)

		if !c.t.IsNext(",") {
			break
		}
	}

	// We'll need to use this token string over and over for each name
	// in the list, so remember where to start.
	kind, err := c.parseTypeSpec()
	if err != nil {
		return err
	}

	if kind.IsUndefined() {
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
		return c.newError(errors.ErrInvalidTypeSpec)
	}

	// We got a defined type, so emit the model and store it
	// in each symbol
	model := kind.InstanceOf(&kind) // datatypes.InstanceOfType(kind)

	for _, name := range names {
		c.b.Emit(bytecode.Push, model)
		c.b.Emit(bytecode.SymbolCreate, name)
		c.b.Emit(bytecode.Store, name)
	}

	return nil
}
