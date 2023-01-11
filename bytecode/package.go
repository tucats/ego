package bytecode

import (
	"fmt"
	"strings"
	"sync"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/util"
)

type packageDef struct {
	name string
}

// Note there are reflection dependencies on the name of the
// field; it must be named "Value".
type ConstantWrapper struct {
	Value interface{}
}

var packageCache = map[string]*data.Package{}
var packageCacheLock sync.RWMutex

func CopyPackagesToSymbols(s *symbols.SymbolTable) {
	packageCacheLock.Lock()
	defer packageCacheLock.Unlock()

	for k, v := range packageCache {
		s.SetAlways(k, v)
	}
}

// String generates a human-readable string describing the value
// in the constant wrapper.
func (w ConstantWrapper) String() string {
	return fmt.Sprintf("%s <read only>", data.Format(w.Value))
}

func IsPackage(name string) bool {
	packageCacheLock.Lock()
	defer packageCacheLock.Unlock()

	_, found := packageCache[name]

	return found
}
func GetPackage(name string) (*data.Package, bool) {
	packageCacheLock.Lock()
	defer packageCacheLock.Unlock()

	// Is this one we've already processed? IF so, return the
	// cached value.
	if p, ok := packageCache[name]; ok {
		return p, true
	}

	// No such package already defined, so let's create one and store a new
	// empty symbol table for it's use.
	pkg := data.NewPackage(name)
	pkg.Set(data.SymbolsMDKey, symbols.NewSymbolTable("package "+name))

	packageCache[name] = pkg

	return pkg, false
}

func importByteCode(c *Context, i interface{}) error {
	name := data.String(i)

	pkg, ok := GetPackage(name)
	if !ok {
		return c.error(errors.ErrImportNotCached).Context(name)
	}

	// Do we already have the local symbol table in the tree?
	alreadyFound := false

	for s := c.symbols; s != nil; s = s.Parent() {
		if s.Package() == name {
			alreadyFound = true

			break
		}
	}

	// If the package table isn't already in the tree, inject if it
	// there is one.
	if !alreadyFound {
		if symV, found := pkg.Get(data.SymbolsMDKey); found {
			sym := symV.(*symbols.SymbolTable)
			sym.SetPackage(name)

			sym.SetParent(c.symbols)
			c.symbols = sym
		}
	}

	// Finally, store the entire package definition by name as well.
	c.symbolSetAlways(name, pkg)

	return nil
}

func pushPackageByteCode(c *Context, i interface{}) error {
	name := data.String(i)

	// Are we already in this package? Happens when a directory of package
	// files are concatenated together...
	if len(c.packageStack) > 0 && c.packageStack[len(c.packageStack)-1].name == name {
		return nil
	}

	// Add the package to the stack, and create a nested symbol table scope.
	c.packageStack = append(c.packageStack, packageDef{
		name,
	})

	// Create an initialize the package variable. If it already exists
	// as a package (from a previous import or autoimport) re-use it
	pkg, _ := GetPackage(name)

	// Define a symbol table to be used with the package. If there
	// already is one for this package, use it. Else create a new one.
	var syms *symbols.SymbolTable

	if symV, ok := pkg.Get(data.SymbolsMDKey); ok {
		syms = symV.(*symbols.SymbolTable)
	} else {
		syms = symbols.NewSymbolTable("package " + name)
	}

	syms.SetParent(c.symbols)
	syms.SetPackage(name)

	c.symbols = syms

	return nil
}

// Instruction to indicate we are done with any definitions for a
// package. The current (package-specific) symbol table is drained
// and any visible names are copied into the package structure, which
// is then saved in the package cache.
func popPackageByteCode(c *Context, i interface{}) error {
	size := len(c.packageStack)
	if size == 0 {
		return c.error(errors.ErrMissingPackageStatement)
	}

	// Pop the item off the package stack.
	pkgdef := c.packageStack[size-1]
	c.packageStack = c.packageStack[:size-1]

	// Verify that we're on the right package.
	if pkgdef.name != data.String(i) {
		return c.error(errors.ErrPanic).Context("package name mismatch: " + pkgdef.name)
	}

	// Retrieve the package variable
	pkg, found := GetPackage(pkgdef.name)
	if !found {
		return c.error(errors.ErrMissingPackageStatement)
	}

	first := true
	// Copy all the upper-case ("external") symbols names to the package level.
	for _, k := range c.symbols.Names() {
		if !strings.HasPrefix(k, "__") && util.HasCapitalizedName(k) {
			v, attr, _ := c.symbols.GetWithAttributes(k)

			if first {
				ui.Debug(ui.TraceLogger, "(%d) Updating package %s", c.threadID, pkgdef.name)

				first = false
			}

			ui.Debug(ui.TraceLogger, "(%d)   symbol   %s", c.threadID, k)

			// If it was readonly, and not already in a constant wrapper,
			// wrap it as a constant now.
			if attr.Readonly {
				if _, ok := v.(ConstantWrapper); !ok {
					pkg.Set(k, ConstantWrapper{v})
				} else {
					pkg.Set(k, v)
				}
			} else {
				pkg.Set(k, v)
			}
		}
	}

	// Save a copy of symbol table as well in the package, containing the non-exported
	// symbols that aren't hidden values used by Ego itself.
	s := symbols.NewSymbolTable("package " + pkgdef.name + " local values")

	for _, k := range c.symbols.Names() {
		if !strings.HasPrefix(k, "__") {
			v, _ := c.symbols.Get(k)

			s.SetAlways(k, v)
		}
	}

	pkg.Set(data.SymbolsMDKey, s)

	// Reset the active symbol table to the state before we processed
	// the package.
	return c.popSymbolTable()
}
