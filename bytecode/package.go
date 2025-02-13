package bytecode

import (
	"reflect"
	"sort"
	"strings"

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

func IsPackage(name string) bool {
	return packages.Get(name) != nil
}

// GetPackage retrieves a package by name. It assumes the name is the full path
// of the package, and returns the package and a true value indicating the package
// was found. If it was not found, a second search is done to see if this is a
// package name as opposed to a path. If found, it returns the package and a true
// value. If neither search results in a package, it returns nil and a false value.
func GetPackage(name string) (*data.Package, bool) {
	p := packages.Get(name)

	if p != nil {
		return p, true
	}

	p = packages.GetByName(name)

	return p, p != nil
}

func inFileByteCode(c *Context, i interface{}) error {
	c.name = "file " + data.String(i)

	return nil
}

func inPackageByteCode(c *Context, i interface{}) error {
	c.pkg = data.String(i)

	// First, see if this package is known in the symbole table
	// by this name. If so, we'll use it.
	if pkg, found := c.symbols.GetAnyScope(c.pkg); found {
		c.symbols = symbols.GetPackageSymbolTable(pkg).NewChildProxy(c.symbols)

		return nil
	}

	// See if this package is in the package cache. IF so, we'll have to add
	// it in now.
	if pkg := packages.Get(c.pkg); pkg != nil {
		c.symbols = symbols.GetPackageSymbolTable(pkg).NewChildProxy(c.symbols)

		return nil
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

	// Finally, store the entire package definition by name as well.
	c.setAlways(name, pkg)

	return nil
}

// Implement DumpPackages bytecode to print out all the packages.
func dumpPackagesByteCode(c *Context, i interface{}) error {
	var (
		err         error
		packageList []string
	)

	// The argument could be empty, a string name, or a list of names.
	packageList, err = getStringListFromOperand(c, i)
	if err != nil {
		return err
	}

	// Prequalify the package list to ensure they are all valid names. We want
	// to geenrate the error(s) before producing output.
	for _, path := range packageList {
		pkg := packages.Get(path)
		if pkg == nil {
			pkg = packages.GetByName(path)
		}

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
		if pkg == nil {
			pkg = packages.GetByName(path)
		}

		attributeList := make([]string, 0, 3)

		if pkg.Builtins {
			attributeList = append(attributeList, "Builtins")
		}

		if pkg.Source {
			attributeList = append(attributeList, "Source")
		}

		attributes := strings.Join(attributeList, ", ")
		items := makePackageItemList(pkg)

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

func getStringListFromOperand(c *Context, i interface{}) ([]string, error) {
	var packageList []string

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
			return nil, c.error(errors.ErrInvalidOperand)
		}
	}

	return packageList, nil
}

func makePackageItemList(pkg *data.Package) []string {
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

	// Also grab any external values in the internal symbol table, if there is one.
	s := symbols.GetPackageSymbolTable(pkg)
	for _, name := range s.Names() {
		var item string
		// If it is an invisible prefix, it's not exported.
		if strings.HasPrefix(name, defs.InvisiblePrefix) {
			continue
		}

		// If it doesn't start with a capitalized letter, it's not exported.
		if !egostrings.HasCapitalizedName(name) {
			continue
		}

		// If the name is already in the items list because it's in the package
		// definition dictionary, skiop it.
		if _, found := pkg.Get(name); found {
			continue
		}

		value, _ := s.Get(name)
		text := data.Format(value)

		r := reflect.TypeOf(value).String()
		if strings.Contains(r, "bytecode.ByteCode") {
			item = "4func " + text
		} else if strings.HasPrefix(text, "^") {
			item = "2const " + name + " = " + text[1:]
		} else if r == "*data.Type" {
			item = "1type " + name + " " + text
		} else {
			item = "3var " + name + " = " + text
		}

		items = append(items, item)
	}

	// Sort them by type and name
	sort.Strings(items)

	return items
}
