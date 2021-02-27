package bytecode

import (
	"strings"

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

	// Create an initialize the package variable
	pkg := map[string]interface{}{}

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

	// Retrieve the package variable
	pkgx, found := c.symbols.Get(pkgdef.name)
	if !found {
		return c.newError(errors.MissingPackageStatement)
	}

	pkg, _ := pkgx.(map[string]interface{})

	// Copy all the symbols, except any embedded version of the package.
	for k := range c.symbols.Symbols {
		if !strings.HasPrefix(k, "__") && !strings.HasPrefix(k, "$") && k != pkgdef.name {
			v, _ := c.symbols.Get(k)
			pkg[k] = v
		}
	}

	// Define the attribute of the struct as a package.
	datatypes.SetMetadata(pkg, datatypes.ReadonlyMDKey, true)
	datatypes.SetMetadata(pkg, datatypes.StaticMDKey, true)

	// Reset the active symbol table to the state before we processed
	// the package.
	c.symbols = c.symbols.Parent

	// Store the package definition in the now-current symbol table.
	return c.symbols.SetAlways(pkgdef.name, pkg)
}
