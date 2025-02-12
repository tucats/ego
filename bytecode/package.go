package bytecode

import (
	"reflect"
	"sort"
	"strings"

	"github.com/tucats/ego/app-cli/tables"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
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

	if pkg := packages.Get(c.pkg); pkg != nil {
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

	// Finally, store the entire package definition by name as well.
	c.setAlways(name, pkg)

	return nil
}

// Opcode used to indicate start of package definitions.
func pushPackageByteCode(c *Context, i interface{}) error {
	return nil
}

// Instruction to indicate we are done with any definitions for a
// package.
func popPackageByteCode(c *Context, i interface{}) error {
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

	sort.Strings(items)

	return items
}
