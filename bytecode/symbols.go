package bytecode

import (
	"strconv"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/util"
)

/******************************************\
*                                         *
*   S Y M B O L S   A N D  T A B L E S    *
*                                         *
\******************************************/

// pushScopeByteCode instruction processor.
func pushScopeByteCode(c *Context, i interface{}) error {
	oldName := c.symbols.Name

	c.mux.Lock()
	defer c.mux.Unlock()

	c.blockDepth++
	c.symbols = symbols.NewChildSymbolTable("block "+strconv.Itoa(c.blockDepth), c.symbols)

	ui.Debug(ui.SymbolLogger, "(%d) push symbol table \"%s\" <= \"%s\"",
		c.threadID, c.symbols.Name, oldName)

	return nil
}

// popScopeByteCode instruction processor. This drops the current
// symbol table and reverts to its parent table. It also flushes
// any pending "this" stack objects. A chain of receivers
// cannot span a block, so this is a good time to clean up
// any asymmetric pushes.
//
// Note special logic; if this was a package symbol table, take
// time to update the readonly copies of the values in the package
// object itself.
func popScopeByteCode(c *Context, i interface{}) error {
	count := 1
	if i != nil {
		count = datatypes.GetInt(i)
	}

	for count > 0 {
		// See if we're popping off a package table; if so there is work to do to
		// copy the values back to the named package object.
		if err := c.syncPackageSymbols(); err != nil {
			return errors.EgoError(err)
		}

		// Pop off the symbol table and clear up the "this" stack
		if err := c.popSymbolTable(); err != nil {
			return errors.EgoError(err)
		}

		c.thisStack = nil
		c.blockDepth--

		count--
	}

	return nil
}

// symbolCreateByteCode instruction processor.
func createAndStoreByteCode(c *Context, i interface{}) error {
	var value interface{}

	var name string

	wasList := false

	if operands, ok := i.([]interface{}); ok && len(operands) == 2 {
		name = datatypes.GetString(operands[0])
		value = operands[1]
		wasList = true
	} else {
		name = datatypes.GetString(i)
	}

	if c.symbolIsConstant(name) {
		return c.newError(errors.ErrReadOnly)
	}

	err := c.symbolCreate(name)
	if err != nil {
		return c.newError(err)
	}

	if wasList {
		i = []interface{}{name, value}
	}

	return storeByteCode(c, i)
}

// symbolCreateByteCode instruction processor.
func symbolCreateByteCode(c *Context, i interface{}) error {
	n := datatypes.GetString(i)
	if c.symbolIsConstant(n) {
		return c.newError(errors.ErrReadOnly)
	}

	err := c.symbolCreate(n)
	if err != nil {
		err = c.newError(err)
	}

	return err
}

// symbolCreateIfByteCode instruction processor.
func symbolCreateIfByteCode(c *Context, i interface{}) error {
	n := datatypes.GetString(i)
	if c.symbolIsConstant(n) {
		return c.newError(errors.ErrReadOnly)
	}

	sp := c.symbols
	for sp.Parent != nil {
		if _, found := sp.Get(n); found {
			return nil
		}

		sp = sp.Parent
	}

	err := c.symbols.Create(n)
	if err != nil {
		err = c.newError(err)
	}

	return err
}

// symbolDeleteByteCode instruction processor.
func symbolDeleteByteCode(c *Context, i interface{}) error {
	n := datatypes.GetString(i)

	err := c.symbolDelete(n)
	if err != nil {
		return c.newError(err)
	}

	return nil
}

// constantByteCode instruction processor.
func constantByteCode(c *Context, i interface{}) error {
	v, err := c.Pop()
	if err != nil {
		return err
	}

	if IsStackMarker(v) {
		return c.newError(errors.ErrFunctionReturnedVoid)
	}

	varname := datatypes.GetString(i)

	err = c.constantSet(varname, v)
	if err != nil {
		return c.newError(err)
	}

	return err
}

func (c *Context) syncPackageSymbols() error {
	// Before we toss away this, check to see if there are package symbols
	// that need updating in the package object.
	if c.symbols.Parent != nil && c.symbols.Parent.Package != "" {
		packageSymbols := c.symbols.Parent
		pkgname := c.symbols.Parent.Package

		if err := c.popSymbolTable(); err != nil {
			return errors.EgoError(err)
		}

		if pkg, ok := c.symbols.Root().Get(pkgname); ok {
			if m, ok := pkg.(*datatypes.EgoPackage); ok {
				for _, k := range packageSymbols.Names() {
					if util.HasCapitalizedName(k) {
						v, _ := packageSymbols.Get(k)
						m.Set(k, v)
					}
				}
			}
		}
	}

	return nil
}
