package util

import (
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/packages"
	"github.com/tucats/ego/symbols"
)

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
