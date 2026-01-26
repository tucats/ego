package bytecode

import (
	"fmt"
	"strconv"
	"sync"

	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/egostrings"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

/******************************************\
*                                         *
*   S Y M B O L S   A N D  T A B L E S    *
*                                         *
\******************************************/

var dumpMutex sync.Mutex

const (
	BoundaryScope = 1
	ForScope      = 2
)

// dumpSymbolsByteCode instruction processor. This prints all the symbols in the
// current symbol table. This is serialized so output for a given thread is
// printed without interleaving output from other threads.
func dumpSymbolsByteCode(c *Context, i any) error {
	label := c.name

	b, err := data.Bool(i)
	if err != nil {
		return c.runtimeError(err)
	}

	if b {
		if text, err := c.Pop(); err != nil {
			return err
		} else {
			label = data.String(text)
		}
	}

	dumpMutex.Lock()
	defer dumpMutex.Unlock()

	if label == "" {
		label = c.name
	}

	fmt.Printf("Symbols for %s, thread id %d:\n\n%s\n",
		label,
		c.threadID,
		c.symbols.Format(true))

	return nil
}

// pushScopeByteCode instruction processor. This creates a new symbol table.
// By default its parent is the current symbol table, so this creates a new
// symbol scope that has visibility to the parent symbol table(s). If the
// optional argument is a boolean true value, the scope is a function scope
// and is parented to the root/global table only.
func pushScopeByteCode(c *Context, i any) error {
	var (
		args       any
		found      bool
		isBoundary bool
	)

	oldName := c.symbols.Name
	newName := "block " + strconv.Itoa(c.blockDepth)

	c.blockDepth++

	// Normally, this is a block scope, and is a child of the current table.
	// However, if the opcode argument is "true", it is a function scope and
	// has the root table as it's parent. Such a scope cannot see the tables
	// of it's caller, only the common root table. This is used for function
	// blocks that are not closures (closures can see the parent scope)
	parent := c.symbols

	// If we are making a function scope, it does not have a parent table other
	// than the root table. Also, any argument list that was created by the
	// caller's scope must be copied to this scope so it can be unpacked by the
	// function body.
	//
	// Note that this behavior can be disabled by setting the "ego.runtime.deep.scope"
	// config value. This is set by default during "ego test" operations.
	scope, err := data.Int(i)
	if err != nil {
		return c.runtimeError(err)
	}

	if scope == BoundaryScope && !settings.GetBool(defs.RuntimeDeepScopeSetting) {
		isBoundary = true

		if c.name != "" {
			newName = "function " + c.bc.name
		}

		parent = parent.FindNextScope()
		if parent == nil {
			parent = &symbols.RootSymbolTable
		}

		// Fetch the argument symbol value if there is one in the parent scope
		args, found = c.symbols.GetLocal(defs.ArgumentListVariable)
	}

	c.symbols = symbols.NewChildSymbolTable(newName, parent).Shared(false).Boundary(isBoundary)

	ui.Log(ui.SymbolLogger, "symbols.push.table.boundary", ui.A{
		"thread": c.threadID,
		"name":   c.symbols.Name,
		"parent": oldName,
		"flag":   isBoundary})

	// If there was an argument list in our former parent, copy in into the new
	// current table. This moves argument values across the function call boundary.
	if found {
		// If there is an argument symbol value, store it in the current table
		c.symbols.SetAlways(defs.ArgumentListVariable, args)
	}

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
func popScopeByteCode(c *Context, i any) error {
	var err error

	count := 1
	if i != nil {
		count, err = data.Int(i)
		if err != nil {
			return c.runtimeError(err)
		}
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

		c.receiverStack = nil
		c.blockDepth--

		count--
	}

	return nil
}

// symbolCreateByteCode instruction processor.
func createAndStoreByteCode(c *Context, i any) error {
	var (
		value any
		err   error
		name  string
	)

	// It could be the wrappered list type, or an array of arguments,
	// or we might need to use the operand as the name and get the value
	// from the stack.
	if operands, ok := i.(data.List); ok && operands.Len() == 2 {
		name = data.String(operands.Get(0))
		value = c.unwrapConstant(operands.Get(1))
	} else if operands, ok := i.([]any); ok && len(operands) == 2 {
		name = data.String(operands[0])
		value = c.unwrapConstant(operands[1])
	} else {
		name = data.String(i)

		if value, err = c.Pop(); err != nil {
			return err
		}

		// If the value on the stack is a marker, then we had a case
		// of a function that did not return a value properly.
		if isStackMarker(value) {
			return c.runtimeError(errors.ErrFunctionReturnedVoid)
		}
	}

	// Do we allow a type to be stored? This is a language extension feature.
	if _, ok := value.(*data.Type); ok && !c.extensions {
		return c.runtimeError(errors.ErrInvalidType)
	}

	// Are we trying to overwrite an existing constant?
	if c.isConstant(name) {
		return c.runtimeError(errors.ErrReadOnly)
	}

	if err = c.create(name); err != nil {
		return c.runtimeError(err)
	}

	// If the name starts with "_" it is implicitly a readonly
	// variable. In this case, make a copy of the value to
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
func symbolCreateByteCode(c *Context, i any) error {
	n := data.String(i)
	if c.isConstant(n) {
		return c.runtimeError(errors.ErrReadOnly)
	}

	// Do we allow a type to be stored? This is a language extension feature.
	if _, ok := i.(*data.Type); ok && !c.extensions {
		return c.runtimeError(errors.ErrInvalidType)
	}

	err := c.create(n)
	if err != nil {
		err = c.runtimeError(err)
	}

	return err
}

// symbolCreateIfByteCode instruction processor.
func symbolCreateIfByteCode(c *Context, i any) error {
	n := data.String(i)
	if c.isConstant(n) {
		return c.runtimeError(errors.ErrReadOnly)
	}

	sp := c.symbols
	if _, found := sp.GetLocal(n); found {
		return nil
	}

	// Do we allow a type to be stored? This is a language extension feature.
	if _, ok := i.(*data.Type); ok && !c.extensions {
		return c.runtimeError(errors.ErrInvalidType)
	}

	err := c.symbols.Create(n)
	if err != nil {
		err = c.runtimeError(err)
	}

	return err
}

// symbolDeleteByteCode instruction processor.
func symbolDeleteByteCode(c *Context, i any) error {
	n := data.String(i)

	if err := c.delete(n); err != nil {
		return c.runtimeError(err)
	}

	return nil
}

// constantByteCode instruction processor.
func constantByteCode(c *Context, i any) error {
	v, err := c.Pop()
	if err != nil {
		return err
	}

	// Do we allow a type to be stored? This is a language extension feature.
	if _, ok := i.(*data.Type); ok && !c.extensions {
		return c.runtimeError(errors.ErrInvalidType)
	}

	if isStackMarker(v) {
		return c.runtimeError(errors.ErrFunctionReturnedVoid)
	}

	variableName := data.String(i)

	err = c.setConstant(variableName, v)
	if err != nil {
		return c.runtimeError(err)
	}

	return err
}

func (c *Context) syncPackageSymbols() error {
	// Before we toss away this, check to see if there are package symbols
	// that need updating in the package object.
	if c.symbols.Parent() != nil && c.symbols.Parent().Package() != "" {
		packageSymbols := c.symbols.Parent()
		packageName := c.symbols.Parent().Package()

		if err := c.popSymbolTable(); err != nil {
			return errors.New(err)
		}

		if pkg, ok := c.symbols.Root().Get(packageName); ok {
			if m, ok := pkg.(*data.Package); ok {
				for _, k := range packageSymbols.Names() {
					if egostrings.HasCapitalizedName(k) {
						v, attr, _ := packageSymbols.GetWithAttributes(k)
						if attr.Ephemeral {
							continue
						}

						m.Set(k, v)
					}
				}
			}
		}
	}

	return nil
}
