package symbols

import (
	"github.com/google/uuid"
	"github.com/tucats/ego/datatypes"
)

// Make a copy of the symbol table, retaining the same values
// as before (in fact, the values are shared between the tables).
// This is mostly used to create unique symbol and constant maps,
// which are needed for shallow clones of a compiler.
func (s *SymbolTable) Clone(withLock bool) *SymbolTable {
	if withLock {
		s.mutex.Lock()
		defer s.mutex.Unlock()
	}

	t := SymbolTable{
		Name:          s.Name,
		Package:       s.Package,
		Parent:        s.Parent,
		Symbols:       map[string]*SymbolAttribute{},
		Values:        s.Values,
		ID:            uuid.New(),
		ValueSize:     s.ValueSize,
		ScopeBoundary: true,
		isRoot:        true,
	}

	for k, v := range s.Symbols {
		t.Symbols[k] = v
	}

	return &t
}

// For a given source table, find all the packages in the table and put them
// in the current table.
func (s *SymbolTable) GetPackages(source *SymbolTable) (count int) {
	if source == nil {
		return
	}

	for k, attributes := range source.Symbols {
		v := source.GetValue(attributes.Slot)
		if p, ok := v.(*datatypes.EgoPackage); ok {
			_ = s.SetAlways(k, p)
			count++
		}
	}

	return count
}
