package compiler

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/tokenizer"
)

// compileVar compiles the var statement.
func (c *Compiler) compileVar() error {
	isList := c.t.IsNext(tokenizer.StartOfListToken)
	if isList {
		c.t.IsNext(tokenizer.SemicolonToken)
	}

	parsing := true

	for parsing {
		names := []string{}

		// If we are in a list and the next token is end-of-list, break
		// on out.
		nextToken := c.t.Peek(1)
		if isList && nextToken == tokenizer.EndOfListToken {
			break
		}

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
					parsing = false

					break
				}

				return c.error(errors.ErrInvalidSymbolName, name)
			}

			name = c.normalizeToken(name)
			names = append(names, name.Spelling())

			if isList && (c.t.Peek(1) == tokenizer.EndOfListToken || c.t.Peek(1) == tokenizer.AssignToken) {
				parsing = false

				break
			}

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
			// Not a symbol name, so fail
			return c.varUserType(names)
		}

		// We got a defined type, so emit the model and store it
		// in each symbol. However, if there's an "=" next, it
		// means the user has supplied the model (initial value).
		model := kind.InstanceOf(kind)

		// Generage as many copies of this value on the stack as
		// needed to satisfy the number of symbols being declared.
		// Cast the initiaiizer value the correct type and store
		// in each named symbol.
		err = varInitializer(c, kind, names, model)
		if err != nil {
			return err
		}

		// If this isn't a list of variables, we're done. If it is, there
		// will be a semicolon after this var clause we might need to eat.
		if !isList {
			break
		} else {
			c.t.IsNext(tokenizer.SemicolonToken)
		}
	}

	// If this was a list of variables, we need to parse the trailing list close token.
	if isList {
		if !c.t.IsNext(tokenizer.EndOfListToken) {
			return c.error(errors.ErrInvalidList)
		}
	}

	return nil
}

// varInitializer parses the initializer for the var list, if present. If there is no
// initializer, no work is done. If there is an initializer, it's parsed and the model
// is then stored in each symbol.
func varInitializer(c *Compiler, kind *data.Type, names []string, model interface{}) error {
	var err error

	if c.t.IsNext(tokenizer.AssignToken) {
		err = c.compileInitializer(kind)
		if err != nil {
			return err
		}

		count := len(names)
		for count > 1 {
			c.b.Emit(bytecode.Dup)

			count--
		}

		for _, name := range names {
			c.b.Emit(bytecode.SymbolCreate, name)
			c.b.Emit(bytecode.Push, kind)
			c.b.Emit(bytecode.Swap)
			c.b.Emit(bytecode.Call, 1)
			c.b.Emit(bytecode.Store, name)
		}
	} else {
		for _, name := range names {
			c.b.Emit(bytecode.Push, model)
			c.b.Emit(bytecode.SymbolCreate, name)
			c.b.Emit(bytecode.Store, name)
		}
	}

	return nil
}

func (c *Compiler) varUserType(names []string) error {
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

	return c.error(errors.ErrInvalidTypeSpec)
}
