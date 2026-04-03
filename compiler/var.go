package compiler

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/tokenizer"
)

// compileVar compiles a var declaration statement. The "var" keyword has already
// been consumed by the caller. The supported forms are:
//
//	var name Type
//	var name Type = value
//	var name1, name2 Type
//	var (
//	    name1 Type
//	    name2 Type = value
//	)
//
// The parenthesised list form is recognised by an opening "(" immediately after
// "var". Each declaration inside the list is terminated by a ";".
//
// For every declared name, a SymbolCreate instruction is emitted to allocate the
// variable in the current scope, followed by a Store instruction to set its initial
// value. If no initializer is present the zero-value for the type is stored instead.
func (c *Compiler) compileVar() error {
	var err error

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
		if isList && nextToken.Is(tokenizer.EndOfListToken) {
			break
		}

		// Collect the list of name(s) of symbols to be created.
		// This includes being able to parse a () list of variable
		// names.
		names, parsing, err = c.collectVarListNames(names, isList)
		if err != nil {
			return err
		}

		for _, name := range names {
			c.DefineSymbol(name)
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

		// Generate as many copies of this value on the stack as
		// needed to satisfy the number of symbols being declared.
		// Cast the initializer value the correct type and store
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
			return c.compileError(errors.ErrInvalidList)
		}
	}

	return nil
}

// collectVarListNames reads the comma-separated list of variable names that
// precede the type in a var declaration. For example, in "var a, b, c int"
// it reads [a, b, c] and returns them as a string slice. The isList flag
// indicates whether the declaration is inside a "()" block; in that case
// parsing stops at ")" or "=". Returns the names, whether parsing should
// continue (i.e. more declarations follow), and any error encountered.
func (c *Compiler) collectVarListNames(names []string, isList bool) ([]string, bool, error) {
	parsing := true

	for {
		name := c.t.Next()
		if name.Is(tokenizer.EndOfTokens) {
			if len(names) > 0 {
				break
			}

			return nil, false, c.compileError(errors.ErrMissingSymbol)
		}

		if !name.IsIdentifier() {
			c.t.Advance(-1)

			return nil, false, c.compileError(errors.ErrInvalidSymbolName, name)
		}

		if name.IsReserved(c.flags.extensionsEnabled) {
			c.t.Advance(-1)

			if len(names) > 0 {
				parsing = false

				break
			}

			return nil, false, c.compileError(errors.ErrInvalidSymbolName, name)
		}

		name = c.normalizeToken(name)
		names = append(names, name.Spelling())

		if isList && (c.t.Peek(1).Is(tokenizer.EndOfListToken) || c.t.Peek(1).Is(tokenizer.AssignToken)) {
			parsing = false

			break
		}

		if !c.t.IsNext(tokenizer.CommaToken) {
			break
		}
	}

	return names, parsing, nil
}

// varInitializer parses the initializer for the var list, if present. If there is no
// initializer, no work is done. If there is an initializer, it's parsed and the model
// is then stored in each symbol.
func varInitializer(c *Compiler, kind *data.Type, names []string, model any) error {
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

// varUserType handles a var declaration whose type is a user-defined type name
// rather than one of the built-in primitive types. For example:
//
//	var p Point         // Point is a user-defined struct type
//	var c pkg.Color     // Color is exported from package pkg
//
// For each name in the list, a call to the special "$new" function is emitted,
// passing the type value as the argument. This allocates a properly initialised
// instance of the type and stores it in the newly created symbol.
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
				c.ReferenceSymbol(typeName.Spelling())
			}

			c.b.Emit(bytecode.Call, 1)
			c.b.Emit(bytecode.SymbolCreate, name)
			c.b.Emit(bytecode.Store, name)
		}

		return nil
	}

	return c.compileError(errors.ErrInvalidTypeSpec)
}
