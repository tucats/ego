package util

import (
	"reflect"
	"sort"
	"strings"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/egostrings"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/packages"
	"github.com/tucats/ego/symbols"
)

// Implement util.Package("name") function, which returns an array of string
// definitions of each type, variable, constant, and function in the package.
func getPackage(s *symbols.SymbolTable, args data.List) (any, error) {
	path := data.String(args.Get(0))

	pkg := packages.Get(path)
	if pkg == nil {
		pkg = packages.GetByName(path)
		if pkg == nil {
			return nil, errors.New(errors.ErrInvalidPackageName).Context(path)
		}
	}

	items := makePackageItemList(pkg)

	typeMap := data.NewMap(data.StringType, data.StringType)
	constMap := data.NewMap(data.StringType, data.StringType)
	varMap := data.NewMap(data.StringType, data.StringType)
	funcMap := data.NewMap(data.StringType, data.StringType)

	for _, item := range items {
		parts := strings.SplitN(item, " ", 2)
		kind := parts[0][1:]
		text := parts[1]

		switch kind {
		case "var":
			nameParts := strings.SplitN(text, " ", 2)
			if _, err := varMap.Set(nameParts[0], nameParts[1]); err != nil {
				return nil, err
			}

		case "const":
			nameParts := strings.SplitN(text, " ", 2)
			valueParts := strings.SplitN(nameParts[1], "=", 2)

			if _, err := constMap.Set(nameParts[0], strings.TrimSpace(valueParts[1])); err != nil {
				return nil, err
			}

		case "type":
			nameParts := strings.SplitN(text, " ", 2)
			if _, err := typeMap.Set(nameParts[0], nameParts[1]); err != nil {
				return nil, err
			}

		case "func":
			nameParts := strings.SplitN(text, "(", 2)

			if _, err := funcMap.Set(nameParts[0], text); err != nil {
				return nil, err
			}
		}
	}

	resultMap := data.NewMap(data.StringType, data.MapType(data.StringType, data.StringType))

	if typeMap.Len() > 0 {
		if _, err := resultMap.Set("types", typeMap); err != nil {
			return nil, err
		}
	}

	if constMap.Len() > 0 {
		if _, err := resultMap.Set("constants", constMap); err != nil {
			return nil, err
		}
	}

	if varMap.Len() > 0 {
		if _, err := resultMap.Set("variables", varMap); err != nil {
			return nil, err
		}
	}

	if funcMap.Len() > 0 {
		if _, err := resultMap.Set("functions", funcMap); err != nil {
			return nil, err
		}
	}

	return resultMap, nil
}

// makePackageItemList builds a sorted list of human-readable declarations for every
// exported item in pkg. Each element is prefixed with a digit that controls the
// final sort order: "1" for types, "2" for constants, "3" for variables, "4" for
// functions. The getPackage function strips that leading digit when organizing items
// into the returned map.
//
// Constants use a "^" prefix convention in data.Format output to distinguish them
// from plain variables; makePackageItemList strips the "^" and prepends "2const".
func makePackageItemList(pkg *data.Package) []string {
	items := make([]string, 0, len(pkg.Keys()))

	keys := pkg.Keys()
	for _, key := range keys {
		// Skip internal bookkeeping keys that are not visible to Ego programs.
		if strings.HasPrefix(key, defs.ReadonlyVariablePrefix) {
			continue
		}

		v, _ := pkg.Get(key)

		// Format the value and prepend the sort-order digit plus kind keyword.
		item := data.Format(v)

		switch v.(type) {
		case data.Function:
			// data.Function covers built-in functions declared in the package map.
			item = "4func " + item

		case *data.Type:
			// Type definitions include the key (name) explicitly because
			// data.Format does not embed the name for type values.
			item = "1type " + key + " " + item

		default:
			r := reflect.TypeOf(v).String()
			if strings.Contains(r, "bytecode.ByteCode") {
				// Compiled Ego functions are stored as bytecode, not data.Function.
				item = "4func " + item
			} else if strings.HasPrefix(item, "^") {
				// data.Format marks read-only (constant) values with a leading "^".
				// Strip it and categorize as a constant.
				item = "2const " + key + " = " + item[1:]
			} else {
				item = "3var " + key + " = " + item
			}
		}

		items = append(items, item)
	}

	// Also grab any exported values stored in the package's symbol table rather
	// than the package definition map (e.g., symbols defined by Ego source files
	// that were imported into the package at runtime).
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

	// Sort alphabetically; because each element starts with the digit kind prefix,
	// items end up grouped by kind (types first, then constants, variables, functions).
	sort.Strings(items)

	return items
}

func getPackages(s *symbols.SymbolTable, args data.List) (any, error) {
	var uniqueNames = map[string]bool{}

	// Make the unordered list of all package names defined in all
	// scopes from here. This may include duplicates.
	allNames := packages.List()

	// Scan the list and set values in the map accordingly. This will
	// effectively remove the duplicates.
	for _, name := range allNames {
		uniqueNames[name] = true
	}

	// Now scan over the list of now-unique names and make an Ego array
	// out of the values.
	packages := data.NewArray(data.StringType, 0)
	for name := range uniqueNames {
		packages.Append(name)
	}

	// Ask the array to sort itself, and return the array as the
	// function value.
	_ = packages.Sort()

	return packages, nil
}
