package bytecode

import (
	"unicode"

	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/util"
)

/******************************************\
*                                         *
*   S Y M B O L S   A N D  T A B L E S    *
*                                         *
\******************************************/

// PushScopeImpl instruction processor.
func PushScopeImpl(c *Context, i interface{}) *errors.EgoError {
	s := symbols.NewChildSymbolTable("block", c.symbols)
	c.symbols = s

	return nil
}

// PopScopeImpl instruction processor. This drops the current
// symbol table and reverts to its parent table. It also flushes
// any pending "this" stack objects. A chain of receivers
// cannot span a block, so this is a good time to clean up
// any asymetric pushes.
//
// Note special logic; if this was a package symbol table, take
// time to update the reaadonly copies of the values in the package
// object itself.
func PopScopeImpl(c *Context, i interface{}) *errors.EgoError {
	// See if we're popping off a package table; if so there is work to do to
	// copy the values back to the named package object.
	c.syncPackageSymbols()

	// Pop off the symbol table and clear up the "this" stack
	c.popSymbolTable()
	c.thisStack = nil

	return nil
}

// SymbolCreateImpl instruction processor.
func SymbolCreateImpl(c *Context, i interface{}) *errors.EgoError {
	n := util.GetString(i)
	if c.symbolIsConstant(n) {
		return c.newError(errors.ReadOnlyError)
	}

	err := c.symbolCreate(n)
	if !errors.Nil(err) {
		err = c.newError(err)
	}

	return err
}

// SymbolOptCreateImpl instruction processor.
func SymbolOptCreateImpl(c *Context, i interface{}) *errors.EgoError {
	n := util.GetString(i)
	if c.symbolIsConstant(n) {
		return c.newError(errors.ReadOnlyError)
	}

	sp := c.symbols
	for sp.Parent != nil {
		if _, found := sp.Get(n); found {
			return nil
		}

		sp = sp.Parent
	}

	err := c.symbolCreate(n)
	if !errors.Nil(err) {
		err = c.newError(err)
	}

	return err
}

// SymbolDeleteImpl instruction processor.
func SymbolDeleteImpl(c *Context, i interface{}) *errors.EgoError {
	n := util.GetString(i)

	err := c.symbolDelete(n)
	if !errors.Nil(err) {
		err = c.newError(err)
	}

	return err
}

// ConstantImpl instruction processor.
func ConstantImpl(c *Context, i interface{}) *errors.EgoError {
	v, err := c.Pop()
	if !errors.Nil(err) {
		return err
	}

	varname := util.GetString(i)

	err = c.constantSet(varname, v)
	if !errors.Nil(err) {
		return c.newError(err)
	}

	return err
}

func (c *Context) syncPackageSymbols() {
	// Before we toss away this, check to see if there are package symbols
	// that need updating in the package object.
	if c.symbols.Parent != nil && c.symbols.Parent.Package != "" {
		pkgsyms := c.symbols.Parent
		pkgname := c.symbols.Parent.Package
		c.popSymbolTable()

		if pkg, ok := symbols.RootSymbolTable.Get(pkgname); ok {
			if m, ok := pkg.(map[string]interface{}); ok {
				for k, v := range pkgsyms.Symbols {
					var firstRune rune
					for _, ch := range k {
						firstRune = ch

						break
					}

					if unicode.IsUpper(firstRune) {
						m[k] = pkgsyms.Values[v]
					}
				}
			}
		}
	}
}
