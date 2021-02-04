package bytecode

import (
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/util"
)

/******************************************\
*                                         *
*   S Y M B O L S   A N D  T A B L E S    *
*                                         *
\******************************************/

// PushScopeImpl instruction processor.
func PushScopeImpl(c *Context, i interface{}) error {
	s := symbols.NewChildSymbolTable("block", c.symbols)
	c.symbols = s

	return nil
}

// PopScopeImpl instruction processor.
func PopScopeImpl(c *Context, i interface{}) error {
	c.symbols = c.symbols.Parent

	return nil
}

// SymbolCreateImpl instruction processor.
func SymbolCreateImpl(c *Context, i interface{}) error {
	n := util.GetString(i)
	if c.IsConstant(n) {
		return c.NewError(ReadOnlyError)
	}

	err := c.Create(n)
	if err != nil {
		err = c.NewError(err.Error())
	}

	return err
}

// SymbolOptCreateImpl instruction processor.
func SymbolOptCreateImpl(c *Context, i interface{}) error {
	n := util.GetString(i)
	if c.IsConstant(n) {
		return c.NewError(ReadOnlyError)
	}

	sp := c.symbols
	for sp.Parent != nil {
		if _, found := sp.Get(n); found {
			return nil
		}

		sp = sp.Parent
	}

	err := c.Create(n)
	if err != nil {
		err = c.NewError(err.Error())
	}

	return err
}

// SymbolDeleteImpl instruction processor.
func SymbolDeleteImpl(c *Context, i interface{}) error {
	n := util.GetString(i)

	err := c.Delete(n)
	if err != nil {
		err = c.NewError(err.Error())
	}

	return err
}

// ConstantImpl instruction processor.
func ConstantImpl(c *Context, i interface{}) error {
	v, err := c.Pop()
	if err != nil {
		return err
	}

	varname := util.GetString(i)

	err = c.SetConstant(varname, v)
	if err != nil {
		return c.NewError(err.Error())
	}

	return err
}
