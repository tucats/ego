package bytecode

import (
	"reflect"
	"sort"
	"strings"
	"sync"

	"github.com/tucats/ego/app-cli/tables"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/egostrings"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/packages"
	"github.com/tucats/ego/symbols"
)

const (
	packagePrefix = "package "
)

type packageDef struct {
	name string
}

// Note there are reflection dependencies on the name of the
// field; it must be named "Value".
type ConstantWrapperx struct {
	Value interface{}
}

var packageCache = map[string]*data.Package{}
var packageCacheLock sync.RWMutex

func IsPackage(name string) bool {
	packageCacheLock.Lock()
	defer packageCacheLock.Unlock()

	_, found := packageCache[name]

	return found
}

func GetPackage(name string) (*data.Package, bool) {
	p := packages.Get(name)

	return p, p != nil
}

func inFileByteCode(c *Context, i interface{}) error {
	c.name = "file " + data.String(i)

	return nil
}

func inPackageByteCode(c *Context, i interface{}) error {
	c.pkg = data.String(i)

	if pkg, found := packageCache[c.pkg]; found {
		if v, found := pkg.Get(data.SymbolsMDKey); found {
			if s, ok := v.(*symbols.SymbolTable); ok {
				c.symbols = s.NewChildProxy(c.symbols)

				return nil
			}

			return c.error(errors.ErrUnknownPackageMember).Context(data.SymbolsMDKey)
		}

		return c.error(errors.ErrUnknownPackageMember).Context(data.SymbolsMDKey)
	}

	return c.error(errors.ErrInvalidPackageName).Context(c.pkg)
}

func importByteCode(c *Context, i interface{}) error {
	var name, path string

	if v, ok := i.(data.List); ok {
		name = data.String(v.Get(0))
		path = data.String(v.Get(1))
	} else {
		name = data.String(i)
		path = name
	}

	pkg := packages.Get(path)
	if pkg == nil {
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
			// Must make a clone of the symbol table since packages are shared
			c.symbols = sym.Clone(c.symbols)
		}
	}

	// Finally, store the entire package definition by name as well.
	c.setAlways(name, pkg)

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

	// Create and initialize the package variable. If it already exists
	// as a package (from a previous import or autoimport) re-use it
	pkg, _ := GetPackage(name)

	// Define a symbol table to be used with the package. If there
	// already is one for this package, use it. Else create a new one.
	var syms *symbols.SymbolTable

	if symV, ok := pkg.Get(data.SymbolsMDKey); ok {
		syms = symV.(*symbols.SymbolTable)
	} else {
		syms = symbols.NewSymbolTable(packagePrefix + name)
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
		if !strings.HasPrefix(k, defs.InvisiblePrefix) && egostrings.HasCapitalizedName(k) {
			v, attr, _ := c.symbols.GetWithAttributes(k)

			if first {
				ui.Log(ui.TraceLogger, "trace.package.update", ui.A{
					"thread":  c.threadID,
					"package": pkgdef.name})

				first = false
			}

			ui.Log(ui.TraceLogger, "trace.symbol.readonly", ui.A{
				"thread": c.threadID,
				"name":   k,
				"flag":   attr.Readonly})

			// If it was readonly, store it as a constant value now.
			if attr.Readonly {
				pkg.Set(k, data.Constant(v))
			} else {
				pkg.Set(k, v)
			}

			// We've modified the package, so update the cache.
			packageCache[pkgdef.name] = pkg
		}
	}

	// Save a copy of symbol table as well in the package, containing the non-exported
	// symbols that aren't hidden values used by Ego itself. This will be grabbed
	// by a call to a package function that might need them.
	s := symbols.NewSymbolTable(pkgdef.name + " local")
	s.SetPackage(pkgdef.name)

	for _, k := range c.symbols.Names() {
		if !strings.HasPrefix(k, defs.InvisiblePrefix) {
			v, _ := c.symbols.Get(k)

			s.SetAlways(k, v)
		}
	}

	pkg.Set(data.SymbolsMDKey, s)

	// Reset the active symbol table to the state before we processed
	// the package.
	return c.popSymbolTable()
}

// Implement DumpPackages bytecode to print out all the packages.
func dumpPackagesByteCode(c *Context, i interface{}) error {
	var (
		err         error
		packageList []string
	)

	// The argument could be empty, a string name, or a list of names.
	if i == nil {
		packageList = packages.List()
	} else {
		switch actual := i.(type) {
		case string:
			packageList = []string{actual}

		case []interface{}:
			for i := 0; i < len(actual); i++ {
				packageList = append(packageList, data.String(actual[i]))
			}

		case []string:
			packageList = actual

		case data.List:
			for i := 0; i < actual.Len(); i++ {
				packageList = append(packageList, data.String(actual.Get(i)))
			}

		default:
			return c.error(errors.ErrInvalidOperand)
		}
	}

	// Prequalify the package list to ensure they are all valid names. We want
	// to geenrate the error(s) before producing output.
	for _, path := range packageList {
		pkg := packages.Get(path)
		if pkg == nil {
			nextErr := c.error(errors.ErrInvalidPackageName).Context(path)
			if err == nil {
				err = nextErr
			} else {
				err = errors.Chain(errors.New(err), nextErr)
			}
		}
	}

	if !errors.Nil(err) {
		return err
	}

	// Use a Table object to format the output neatly.
	t, err := tables.New([]string{"Package", "Attributes", "Kind", "Item"})
	if err != nil {
		return c.error(err)
	}

	t.SetPagination(0, 0)

	// Rescan the list and generate the output.
	for _, path := range packageList {
		pkg := packages.Get(path)
		attributeList := make([]string, 0, 3)

		if pkg.Builtins {
			attributeList = append(attributeList, "Builtins")
		}

		if pkg.Source {
			attributeList = append(attributeList, "Source")
		}

		attributes := strings.Join(attributeList, ", ")
		items := make([]string, 0, len(pkg.Keys()))

		keys := pkg.Keys()
		for _, key := range keys {
			if strings.HasPrefix(key, defs.ReadonlyVariablePrefix) {
				continue
			}

			v, _ := pkg.Get(key)

			// Format the item, with any helpful prefix. The first byte of the
			// prefix controls the sort order (types, constants, variables, and
			// functions). )
			item := data.Format(v)

			switch v.(type) {
			case data.Function:
				item = "4func " + item

			case *data.Type:
				item = "1type " + key + " " + item

			default:
				r := reflect.TypeOf(v).String()
				if strings.Contains(r, "bytecode.ByteCode") {
					item = "4func " + item
				} else if strings.HasPrefix(item, "^") {
					item = "2const " + key + " = " + item[1:]
				} else {
					item = "3var " + key + " = " + item
				}
			}

			items = append(items, item)
		}

		sort.Strings(items)

		// Now that the list is sorted by types, add the items to the table, stripping
		// off the sort key prefix.
		lastKind := ""
		kind := ""

		for _, item := range items {
			item = item[1:]

			fields := strings.SplitN(item, " ", 2)
			if fields[0] != lastKind {
				lastKind = fields[0]
				kind = lastKind
			}

			t.AddRow([]string{path, attributes, kind, fields[1]})

			if ui.OutputFormat == ui.TextFormat {
				path = ""
				attributes = ""
				kind = ""
			}
		}

		if ui.OutputFormat == ui.TextFormat {
			t.AddRow([]string{"", "", "", ""})
		}
	}

	return t.Print(ui.OutputFormat)
}
