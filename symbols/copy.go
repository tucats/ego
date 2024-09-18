package symbols

import (
	"github.com/google/uuid"
	"github.com/tucats/ego/data"
)

// For a given source table, find all the packages in the table and put them
// in the current table. Note that the underlying package data is shared by
// both tables, but cannot be modified by either.
func (s *SymbolTable) CopyPackagesFromTable(source *SymbolTable) (count int) {
	if source == nil {
		return
	}

	for k, attributes := range source.symbols {
		v := source.GetValue(attributes.slot)
		if p, ok := v.(*data.Package); ok {
			s.SetAlways(k, p)

			// Because we've made a copy of the package, we need to
			// ensure that the copy is not modifiable.
			s.symbols[k].Readonly = true
			source.symbols[k].Readonly = true

			count++
		}
	}

	return count
}

// For a given table, make a copy of the table and return the new
// copy.
func (s *SymbolTable) Clone(parent *SymbolTable) *SymbolTable {
	if s == nil {
		return nil
	}

	newTable := NewChildSymbolTable("clone of "+s.Name, parent)

	newTable.isRoot = s.isRoot
	newTable.shared = false
	newTable.boundary = s.boundary
	newTable.forPackage = s.forPackage
	newTable.id = uuid.New()
	newTable.depth = s.depth
	newTable.isClone = true

	// Copy the values from the source table to the new table.
	for k := range s.symbols {
		v, _ := s.Get(k)
		newTable.SetAlways(k, v)
	}

	if newTable.forPackage != "" {
		if pkg, found := s.Get(newTable.forPackage); found {
			if p, ok := pkg.(*data.Package); ok {
				keys := p.Keys()
				for _, key := range keys {
					if v, found := p.Get(key); found {
						newTable.SetAlways(key, v)
					}
				}
			}
		}
	}

	return newTable
}
