package bytecode

import (
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
func PopScopeImpl(c *Context, i interface{}) *errors.EgoError {
	c.symbols = c.symbols.Parent
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
