package bytecode

import (
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/util"
)

type packageDef struct {
	name string
}

func pushPackageByteCode(c *Context, i interface{}) *errors.EgoError {
	name := util.GetString(i)

	// Are we already in this package? Happens when a directory of package
	// files are concatenated together...

	if len(c.packageStack) > 0 && c.packageStack[len(c.packageStack)-1].name == name {
		//ui.Debug(ui.CompilerLogger, "+++ Already processing package %s, not pushed", name)
		return nil
	}

	// Add the package to the stack, and create a nested symbol table scope.
	c.packageStack = append(c.packageStack, packageDef{
		name,
	})
	c.symbols = symbols.NewChildSymbolTable("package "+name, c.symbols)

	// Create an initialize the package variable. If it already exists
	// as a package (from a previous import or autoimport) re-use it
	pkg := datatypes.EgoPackage{}

	if v, ok := c.symbols.Root().Get(name); ok {
		switch actual := v.(type) {
		case *datatypes.EgoStruct:
			panic("DEBUG: map/struct confusion: pushPackageByteCode()")

		case datatypes.EgoPackage:
			pkg = actual

			if v, ok := datatypes.GetMetadata(actual, datatypes.SymbolsMDKey); ok {
				if ps, ok := v.(*symbols.SymbolTable); ok {
					ps.Parent = c.symbols
					c.symbols = ps
				}
			}
		}
	}

	// Define the attribute of the struct as a package.
	datatypes.SetType(pkg, datatypes.Package(name))

	return c.symbols.SetAlways(name, pkg)
}

// Instruction to indicate we are done with any definitions for a
// package. The current (package-specific) symbol table is drained
// and any visible names are copied into the package structure, which
// is then saved in the current symbol table.
func popPackageByteCode(c *Context, i interface{}) *errors.EgoError {
	size := len(c.packageStack)
	if size == 0 {
		return c.newError(errors.ErrMissingPackageStatement)
	}

	// Pop the item off the package stack.
	pkgdef := c.packageStack[size-1]
	c.packageStack = c.packageStack[:size-1]

	// Verify that we're on the right package.
	if pkgdef.name != util.GetString(i) {
		return c.newError(errors.ErrPanic).Context("package name mismatch: " + pkgdef.name)
	}
	// Retrieve the package variable
	pkgValue, found := c.symbols.Get(pkgdef.name)
	if !found {
		return c.newError(errors.ErrMissingPackageStatement)
	}

	if _, ok := pkgValue.(*datatypes.EgoStruct); ok {
		panic("DEBUG: map/struct confusion: popPackageByteCode()")
	}

	pkg, _ := pkgValue.(datatypes.EgoPackage)
	if pkg == nil {
		pkg = datatypes.EgoPackage{}
	}

	first := true
	// Copy all the upper-case ("external") symbols names to the package level.
	for k := range c.symbols.Symbols {
		if util.HasCapitalizedName(k) {
			v, _ := c.symbols.Get(k)
			pkg[k] = v

			if first {
				ui.Debug(ui.TraceLogger, "(%d) Updating package %s", c.threadID, pkgdef.name)

				first = false
			}

			ui.Debug(ui.TraceLogger, "(%d)   symbol   %s", c.threadID, k)
		}
	}

	// Copy all the exported constants
	for k, v := range c.symbols.Constants {
		if util.HasCapitalizedName(k) {
			pkg[k] = v

			if first {
				ui.Debug(ui.TraceLogger, "(%d) Updating package %s", c.threadID, pkgdef.name)

				first = false
			}

			ui.Debug(ui.ByteCodeLogger, "(%d)   constant %s", c.threadID, k)
		}
	}

	// Mark the active symbol table we just used as belonging to a package.
	c.symbols.Package = pkgdef.name

	// Define the attribute of the package.
	datatypes.SetMetadata(pkg, datatypes.ReadonlyMDKey, true)
	datatypes.SetMetadata(pkg, datatypes.StaticMDKey, true)
	datatypes.SetMetadata(pkg, datatypes.SymbolsMDKey, c.symbols)

	// Reset the active symbol table to the state before we processed
	// the package.
	c.popSymbolTable()

	// Store the package definition in the root symbol table.
	rootTable := c.symbols.Root()

	ui.Debug(ui.TraceLogger, "(%d) Store package %s in root table %s", c.threadID, pkgdef.name, rootTable.Name)

	return c.symbols.Root().SetAlways(pkgdef.name, pkg)
}
