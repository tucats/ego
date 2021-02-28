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

func pushPackage(c *Context, i interface{}) *errors.EgoError {
	name := util.GetString(i)
	c.packageStack = append(c.packageStack, packageDef{
		name,
	})
	c.symbols = symbols.NewChildSymbolTable("package "+name, c.symbols)

	// Create an initialize the package variable. If it alreaady exists
	// as a package (from a previous import or autoimport) re-use it
	pkg := map[string]interface{}{}

	if v, ok := symbols.RootSymbolTable.Get(name); ok {
		switch actual := v.(type) {
		case map[string]interface{}:
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
	datatypes.SetMetadata(pkg, datatypes.TypeMDKey, "package")

	return c.symbols.SetAlways(name, pkg)
}

// Instruction to indicate we are done with any definitions for a
// package. The current (package-specific) symbol table is drained
// and any visible names are copied into the package structure, which
// is then saved in the current symbol table.
func popPackage(c *Context, i interface{}) *errors.EgoError {
	size := len(c.packageStack)
	if size == 0 {
		return c.newError(errors.MissingPackageStatement)
	}

	// Pop the item off the package stack.
	pkgdef := c.packageStack[size-1]
	c.packageStack = c.packageStack[:size-1]

	// Verify that we're on the right package.
	if pkgdef.name != util.GetString(i) {
		return c.newError(errors.Panic).Context("package name mismatch: " + pkgdef.name)
	}
	// Retrieve the package variable
	pkgx, found := c.symbols.Get(pkgdef.name)
	if !found {
		return c.newError(errors.MissingPackageStatement)
	}

	pkg, _ := pkgx.(map[string]interface{})

	// Copy all the upper-case ("external") symbols names to the package level.
	for k := range c.symbols.Symbols {
		if util.HasCapitalizedName(k) {
			v, _ := c.symbols.Get(k)
			pkg[k] = v

			ui.Debug(ui.ByteCodeLogger, "Copy symbol %s to package", k)
		}
	}

	// Copy all the exported constants
	for k, v := range c.symbols.Constants {
		if util.HasCapitalizedName(k) {
			pkg[k] = v

			ui.Debug(ui.ByteCodeLogger, "Copy constant %s to package", k)
		}
	}

	// Mark the active symbol table we just used as belonging to a package.
	c.symbols.Package = pkgdef.name

	// Define the attribute of the struct as a package.
	datatypes.SetMetadata(pkg, datatypes.ReadonlyMDKey, true)
	datatypes.SetMetadata(pkg, datatypes.StaticMDKey, true)
	datatypes.SetMetadata(pkg, datatypes.SymbolsMDKey, c.symbols)

	// Reset the active symbol table to the state before we processed
	// the package.
	c.popSymbolTable()

	// Store the package definition in the root symbol table.
	return symbols.RootSymbolTable.SetAlways(pkgdef.name, pkg)
}
