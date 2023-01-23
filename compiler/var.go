package compiler

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/tokenizer"
)

// compileVar compiles the var statement.
func (c *Compiler) compileVar() error {
	names := []string{}

	for {
		name := c.t.Next()
		if name == tokenizer.EndOfTokens {
			if len(names) > 0 {
				break
			}

			return c.error(errors.ErrMissingSymbol)
		}

		if !name.IsIdentifier() {
			c.t.Advance(-1)

			return c.error(errors.ErrInvalidSymbolName, name)
		}

		// See if it's a reserved word.
		if name.IsReserved(c.flags.extensionsEnabled) {
			c.t.Advance(-1)
			// If we mid-list, then just done with list
			if len(names) > 0 {
				break
			}

			return c.error(errors.ErrInvalidSymbolName, name)
		}

		name = c.normalizeToken(name)
		names = append(names, name.Spelling())

		if !c.t.IsNext(tokenizer.CommaToken) {
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
		var pkgName tokenizer.Token

		typeName := c.t.Next()
		isPackageType := false

		if typeName.IsIdentifier() {
			if c.t.IsNext(tokenizer.DotToken) {
				pkgName = typeName
				typeName = c.t.Next()
				isPackageType = true
			}

			for _, name := range names {
				c.b.Emit(bytecode.Load, "$new")

				if isPackageType {
					c.b.Emit(bytecode.Load, pkgName)
					c.b.Emit(bytecode.Member, typeName)
				} else {
					c.b.Emit(bytecode.Load, typeName)
				}

				c.b.Emit(bytecode.Call, 1)
				c.b.Emit(bytecode.SymbolCreate, name)
				c.b.Emit(bytecode.Store, name)
			}

			return nil
		}

		// Not a symbol name, so fail
		return c.error(errors.ErrInvalidTypeSpec)
	}

	// We got a defined type, so emit the model and store it
	// in each symbol
	model := kind.InstanceOf(kind) // data.InstanceOfType(kind)

	for _, name := range names {
		c.b.Emit(bytecode.Push, model)
		c.b.Emit(bytecode.SymbolCreate, name)
		c.b.Emit(bytecode.Store, name)
	}

	return nil
}
