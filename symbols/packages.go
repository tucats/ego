package symbols

import (
	"github.com/tucats/ego/data"
)

// GetPackageSymbolTable returns the symbol table embedded in the given package.
// pkg must be a *data.Package; if it is not, nil is returned. If the package
// does not yet have an associated symbol table, a new one is created, stored
// inside the package, and returned. This is used to access or initialize the
// set of symbols that belong to an Ego package.
func GetPackageSymbolTable(pkg any) *SymbolTable {
	if pkt, ok := pkg.(*data.Package); ok {
		if syms, found := pkt.Get(data.SymbolsMDKey); found {
			if table, ok := syms.(*SymbolTable); ok {
				return table
			}
		}

		table := NewSymbolTable("package " + pkt.Name)

		pkt.Set(data.SymbolsMDKey, table)

		return table
	}

	return nil
}
