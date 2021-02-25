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

// PopScopeImpl instruction processor.
func PopScopeImpl(c *Context, i interface{}) *errors.EgoError {
	c.symbols = c.symbols.Parent

	return nil
}

// SymbolCreateImpl instruction processor.
func SymbolCreateImpl(c *Context, i interface{}) *errors.EgoError {
	n := util.GetString(i)
	if c.symbolIsConstant(n) {
		return c.NewError(errors.ReadOnlyError)
	}

	err := c.symbolCreate(n)
	if !errors.Nil(err) {
		err = c.NewError(err)
	}

	return err
}

// SymbolOptCreateImpl instruction processor.
func SymbolOptCreateImpl(c *Context, i interface{}) *errors.EgoError {
	n := util.GetString(i)
	if c.symbolIsConstant(n) {
		return c.NewError(errors.ReadOnlyError)
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
		err = c.NewError(err)
	}

	return err
}

// SymbolDeleteImpl instruction processor.
func SymbolDeleteImpl(c *Context, i interface{}) *errors.EgoError {
	n := util.GetString(i)

	err := c.symbolDelete(n)
	if !errors.Nil(err) {
		err = c.NewError(err)
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
		return c.NewError(err)
	}

	return err
}
