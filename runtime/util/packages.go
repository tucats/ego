package util

import (
	"strings"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/symbols"
)

func getPackages(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	// Make the unordered list of all package names defined in all
	// scopes from here. This may include duplicates.
	allNames := makePackageList(s)

	// Scan the list and set values in the map accordingly. This will
	// effectively remove the duplicates.
	var uniqueNames = map[string]bool{}

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

// makePackageList is a helper function that recursively
// scans the symbol table scope tree from the current
// location, and makes a list of all the package names
// defined within the current scope. The result is an
// array of strings, which may contain duplicates as the
// same package may be defined at multiple scope levels.
func makePackageList(s *symbols.SymbolTable) []string {
	var result []string

	// Scan over the symbol table. Skip hidden symbols.
	for _, k := range s.Names() {
		if strings.HasPrefix(k, defs.InvisiblePrefix) {
			continue
		}

		// Get the symbol. IF it is a package, add it's name
		// to our list.
		v, _ := s.Get(k)
		if p, ok := v.(*data.Package); ok {
			result = append(result, p.Name)
		}
	}

	// If there is a parent table, repeat the operation
	// with the parent table, appending those results to
	// our own.
	if s.Parent() != nil {
		px := makePackageList(s.Parent())
		if len(px) > 0 {
			result = append(result, px...)
		}
	}

	return result
}
