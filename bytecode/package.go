package bytecode

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/tucats/ego/app-cli/tables"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/egostrings"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/i18n"
	"github.com/tucats/ego/packages"
	"github.com/tucats/ego/symbols"
)

// This file implements bytecode instructions and helper functions that
// manage Ego packages — their lookup, import, and scope injection.
//
// Packages in Ego are stored in two places:
//
//  1. The global package cache (managed by the packages/ sub-package), which
//     maps import paths and names to *data.Package values.  This is populated
//     when packages are loaded at startup or on first import.
//
//  2. The runtime symbol table, where a package appears as a named variable of
//     type *data.Package so that Ego code can reference its members.
//
// The bytecode instructions here bridge these two representations: Import
// copies a package from the cache into the symbol table, and InPackage pushes
// a proxy scope that makes the package's symbols visible to the executing
// function body.

// IsPackage reports whether a package with the given name (or import path)
// is registered in the global package cache.
//
// This is a thin convenience wrapper around packages.Get used by the compiler
// and the REPL to decide whether an identifier refers to a package.
func IsPackage(name string) bool {
	return packages.Get(name) != nil
}

// GetPackage retrieves a *data.Package from the global cache.
//
// The lookup is performed in two steps:
//  1. Search by the full import path (e.g. "fmt", "github.com/foo/bar").
//  2. If no match, search by the package's declared name (the identifier
//     after the `package` keyword), which may differ from the path.
//
// Returns the package and true when found, nil and false otherwise.
func GetPackage(name string) (*data.Package, bool) {
	p := packages.Get(name)

	if p != nil {
		return p, true
	}

	p = packages.GetByName(name)

	return p, p != nil
}

// inFileByteCode is the instruction handler for the InFile opcode.
//
// It records the source file name in the execution context so that runtime
// errors can report the file where the faulting instruction was compiled.
// The operand is the file name as a string.
//
// This opcode is emitted by the compiler at the start of each source file
// that is compiled into a bytecode unit.
func inFileByteCode(c *Context, i any) error {
	c.name = "file " + data.String(i)

	return nil
}

// inPackageByteCode is the instruction handler for the InPackage opcode.
//
// It records the current package name in the execution context (c.pkg) and
// then pushes a proxy scope for that package's symbol table onto the scope
// chain.  This makes the package's exported symbols directly visible inside
// the function body without needing to qualify every name with the package
// prefix.
//
// The lookup sequence is:
//  1. If the symbol table already holds a *data.Package under that name
//     (i.e., it was imported before this function was called), use it.
//  2. Otherwise look the package up in the global package cache by path.
//  3. If neither search succeeds, return ErrInvalidPackageName.
//
// A "proxy" scope is a lightweight wrapper that shares the package's symbol
// dictionary but inserts itself into the scope chain above the caller's local
// scope — changes made through the proxy are reflected in the shared package
// table without disturbing the caller's own variables.
func inPackageByteCode(c *Context, i any) error {
	c.pkg = data.String(i)

	// First, see if this package is already in the symbol table.
	// GetAnyScope walks up all scope levels looking for the name.
	if found, ok := c.symbols.GetAnyScope(c.pkg); ok {
		// Guard: the symbol must actually be a *data.Package.  If another
		// variable shadows the package name (PACKAGES-1 fix), we fall
		// through rather than panicking when GetPackageSymbolTable returns nil.
		if pkg, isPkg := found.(*data.Package); isPkg {
			c.symbols = symbols.GetPackageSymbolTable(pkg).NewChildProxy(c.symbols)

			return nil
		}
	}

	// See if this package is in the global package cache. If so, we'll have to add
	// it in now.
	if pkg := packages.Get(c.pkg); pkg != nil {
		c.symbols = symbols.GetPackageSymbolTable(pkg).NewChildProxy(c.symbols)

		return nil
	}

	return c.runtimeError(errors.ErrInvalidPackageName).Context(c.pkg)
}

// importByteCode is the instruction handler for the Import opcode.
//
// It loads a package from the global package cache into the current symbol
// table under the package's local alias.  The compiler emits this opcode for
// each `import` statement in an Ego source file.
//
// The operand can take two forms:
//
//   - A plain string: both the import path and the local alias are the same
//     value (e.g. `import "fmt"` → name="fmt", path="fmt").
//   - A data.List{name, path}: the first element is the local alias and the
//     second is the import path (e.g. `import f "fmt"` → name="f", path="fmt").
//
// Returns ErrImportNotCached when the package is not in the cache, which
// typically means the package was not loaded at startup.
func importByteCode(c *Context, i any) error {
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
		return c.runtimeError(errors.ErrImportNotCached).Context(name)
	}

	// Store the package in the current symbol table under its local alias.
	c.setAlways(name, pkg)

	return nil
}

// dumpPackagesByteCode is the instruction handler for the DumpPackages opcode.
//
// It prints a formatted table of all known packages (or a specific subset)
// to the context's output writer.  Each row describes one item exported by a
// package, annotated with its kind (type, constant, variable, or function).
//
// The operand controls which packages are listed:
//
//   - nil  — list every package in the global cache.
//   - string — list the single named package.
//   - []any, []string, or data.List — list each named package in the slice.
//
// Invalid package names cause ErrInvalidPackageName.  All names are validated
// before any output is produced, so the table is never partially written.
//
// This opcode is emitted by the `ego dump` REPL command; it is not part of
// normal program execution.
func dumpPackagesByteCode(c *Context, i any) error {
	var (
		err         error
		packageList []string
	)

	// The argument could be empty, a string name, or a list of names.
	packageList, err = getStringListFromOperand(c, i)
	if err != nil {
		return err
	}

	// Pre-qualify the package list to ensure they are all valid names. We want
	// to generate the error(s) before producing output.
	for _, path := range packageList {
		pkg := packages.Get(path)
		if pkg == nil {
			pkg = packages.GetByName(path)
		}

		if pkg == nil {
			nextErr := c.runtimeError(errors.ErrInvalidPackageName).Context(path)
			if err == nil {
				err = nextErr
			} else {
				err = errors.New(err).Chain(nextErr)
			}
		}
	}

	if !errors.Nil(err) {
		return err
	}

	// Use a Table object to format the output neatly.
	t, err := tables.New([]string{i18n.L("Package"), i18n.L("Attributes"), i18n.L("Kind"), i18n.L("Item")})
	if err != nil {
		return c.runtimeError(err)
	}

	t.SetPagination(0, 0)

	// Re-scan the list and generate the output.
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

	text, _ := t.String(ui.OutputFormat)
	_, err = fmt.Fprintf(c.output, "%s", text)

	return err
}

// getStringListFromOperand converts the instruction operand into a []string.
//
// This helper is used by dumpPackagesByteCode to accept flexible operand
// forms without duplicating the dispatch logic.  Supported operand types:
//
//   - nil        — returns packages.List() (all registered package paths).
//   - string     — returns a one-element slice.
//   - []any      — each element is converted to a string via data.String.
//   - []string   — returned as-is.
//   - data.List  — each element is converted to a string via data.String.
//   - any other  — returns ErrInvalidOperand.
func getStringListFromOperand(c *Context, i any) ([]string, error) {
	var packageList []string

	if i == nil {
		packageList = packages.List()
	} else {
		switch actual := i.(type) {
		case string:
			packageList = []string{actual}

		case []any:
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
			return nil, c.runtimeError(errors.ErrInvalidOperand)
		}
	}

	return packageList, nil
}

// makePackageItemList returns a sorted list of formatted strings describing
// the exported items in pkg.  Each string begins with a single digit that
// controls sort order (1=type, 2=const, 3=var, 4=func) followed by a
// human-readable description.
//
// Items are collected from two sources:
//  1. The package's own key-value dictionary (populated at package-load time).
//  2. The package's associated symbol table, which holds items declared by
//     compiled Ego source (functions, type aliases, constants).
//
// Items from source (2) that are already present in source (1) are skipped
// to avoid duplicates.  Items whose names start with the invisible prefix
// ("__") or that are not exported (first letter not uppercase) are excluded.
//
// Note: Items with nil values in the package dictionary or symbol table are
// safely skipped (PACKAGES-2 fix) rather than panicking on reflect.TypeOf(nil).
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
		// functions).
		item := data.Format(v)

		switch v.(type) {
		case data.Function:
			item = "4func " + item

		case *data.Type:
			item = "1type " + key + " " + item

		default:
			// Guard against nil values — reflect.TypeOf(nil) returns nil and
			// calling .String() on it panics (PACKAGES-2 fix).
			if v == nil {
				item = "3var " + key + " = nil"
			} else {
				r := reflect.TypeOf(v).String()
				if strings.Contains(r, "bytecode.ByteCode") {
					item = "4func " + item
				} else if strings.HasPrefix(item, "^") {
					item = "2const " + key + " = " + item[1:]
				} else {
					item = "3var " + key + " = " + item
				}
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
		// definition dictionary, skip it.
		if _, found := pkg.Get(name); found {
			continue
		}

		value, _ := s.Get(name)
		text := data.Format(value)

		// Guard against nil values in the symbol table (PACKAGES-2 fix).
		if value == nil {
			item = "3var " + name + " = nil"
		} else {
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
		}

		items = append(items, item)
	}

	// Sort them by type and name
	sort.Strings(items)

	return items
}
