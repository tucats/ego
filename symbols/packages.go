package symbols

import (
	"github.com/tucats/ego/data"
)

// GetPackageSymbol retrieves a symbol from a package. The pkg can be an any
// to a package, or the package object itself. The name is the name of the symbol to retrieve.
// The value of the symbol is returned, along with a boolean indicating whether the symbol was found.
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
