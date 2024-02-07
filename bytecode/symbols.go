package bytecode

import (
	"strconv"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
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
	c.symbols = symbols.NewChildSymbolTable("block "+strconv.Itoa(c.blockDepth), c.symbols).Shared(false)

	ui.Log(ui.SymbolLogger, "(%d) push symbol table \"%s\" <= \"%s\"",
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
		count = data.Int(i)
	}

	for count > 0 {
		// See if we're popping off a package table; if so there is work to do to
		// copy the values back to the named package object.
		if err := c.syncPackageSymbols(); err != nil {
			return errors.New(err)
		}

		// Pop off the symbol table and clear up the "this" stack
		if err := c.popSymbolTable(); err != nil {
			return errors.New(err)
		}

		c.thisStack = nil
		c.blockDepth--

		count--
	}

	return nil
}

// symbolCreateByteCode instruction processor.
func createAndStoreByteCode(c *Context, i interface{}) error {
	var (
		value interface{}
		err   error
		name  string
	)

	// It could be the wrappered list type, or an array of arguments,
	// or we might need to use the operand as the name and get the value
	// from the stack.
	if operands, ok := i.(data.List); ok && operands.Len() == 2 {
		name = data.String(operands.Get(0))
		value = operands.Get(1)
	} else if operands, ok := i.([]interface{}); ok && len(operands) == 2 {
		name = data.String(operands[0])
		value = operands[1]
	} else {
		name = data.String(i)
		if value, err = c.Pop(); err != nil {
			return err
		}

		// If the value on the stack is a marker, then we had a case
		// of a function that did not return a value properly.
		if isStackMarker(value) {
			return c.error(errors.ErrFunctionReturnedVoid)
		}
	}

	if c.isConstant(name) {
		return c.error(errors.ErrReadOnly)
	}

	if err = c.create(name); err != nil {
		return c.error(err)
	}

	// If the name starts with "_" it is implicitly a readonly
	// variable.  In this case, make a copy of the value to
	// be stored, and mark it as a readonly value if it is
	// a complex type. Then, store the copy as a constant with
	// the given name.
	if len(name) > 1 && name[0:1] == defs.ReadonlyVariablePrefix {
		constantValue := data.DeepCopy(value)

		switch a := constantValue.(type) {
		case *data.Map:
			a.SetReadonly(true)

		case *data.Array:
			a.SetReadonly(true)

		case *data.Struct:
			a.SetReadonly(true)
		}

		err = c.setConstant(name, constantValue)
	} else {
		err = c.set(name, value)
	}

	return err
}

// symbolCreateByteCode instruction processor.
func symbolCreateByteCode(c *Context, i interface{}) error {
	n := data.String(i)
	if c.isConstant(n) {
		return c.error(errors.ErrReadOnly)
	}

	err := c.create(n)
	if err != nil {
		err = c.error(err)
	}

	return err
}

// symbolCreateIfByteCode instruction processor.
func symbolCreateIfByteCode(c *Context, i interface{}) error {
	n := data.String(i)
	if c.isConstant(n) {
		return c.error(errors.ErrReadOnly)
	}

	sp := c.symbols
	if _, found := sp.GetLocal(n); found {
		return nil
	}

	err := c.symbols.Create(n)
	if err != nil {
		err = c.error(err)
	}

	return err
}

// symbolDeleteByteCode instruction processor.
func symbolDeleteByteCode(c *Context, i interface{}) error {
	n := data.String(i)

	if err := c.delete(n); err != nil {
		return c.error(err)
	}

	return nil
}

// constantByteCode instruction processor.
func constantByteCode(c *Context, i interface{}) error {
	v, err := c.Pop()
	if err != nil {
		return err
	}

	if isStackMarker(v) {
		return c.error(errors.ErrFunctionReturnedVoid)
	}

	varname := data.String(i)

	err = c.setConstant(varname, v)
	if err != nil {
		return c.error(err)
	}

	return err
}

func (c *Context) syncPackageSymbols() error {
	// Before we toss away this, check to see if there are package symbols
	// that need updating in the package object.
	if c.symbols.Parent() != nil && c.symbols.Parent().Package() != "" {
		packageSymbols := c.symbols.Parent()
		pkgname := c.symbols.Parent().Package()

		if err := c.popSymbolTable(); err != nil {
			return errors.New(err)
		}

		if pkg, ok := c.symbols.Root().Get(pkgname); ok {
			if m, ok := pkg.(*data.Package); ok {
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
