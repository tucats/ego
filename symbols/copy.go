package symbols

import (
	"strings"

	"github.com/google/uuid"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
)

// NewChildProxy creates a new symbol table that points to the same dictionary
// and value data as the receiver table, and then binds it to the specified
// parent table. This allows the proxy to have a different parent table than
// the one it is a proxy for, without modifying the original table.
//
// This is primarily used to create a new symbol scope for a package symbol
// table, which might be shared between multiple invocations so the parent
// value cannot be written directly to the package table. But we want to be
// sure to use the same symbol dictionary and values storage.
func (s *SymbolTable) NewChildProxy(parent *SymbolTable) *SymbolTable {
	s.shared = true

	proxy := &SymbolTable{
		Name:     "Proxy for " + s.Name,
		symbols:  s.symbols,
		values:   s.values,
		id:       uuid.New(),
		shared:   true,
		parent:   parent,
		depth:    s.depth,
		boundary: false,
		isRoot:   false,
		isClone:  false,
		proxy:    true,
	}

	return NewChildSymbolTable("runtime for "+s.Name, proxy)
}

func (s *SymbolTable) IsProxy() bool {
	return s.proxy
}

// For a given source table, find all the packages in the table and put them
// in the current table. Note that the underlying package data is shared by
// both tables, but cannot be modified by either.
func (s *SymbolTable) CopyPackagesFromTable(source *SymbolTable) (count int) {
	if source == nil {
		return
	}

	for k, attributes := range source.symbols {
		v := source.getValue(attributes.slot)
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

// For a given source table, find all the symbols in the table and put them
// in the current table.
func (s *SymbolTable) Merge(source *SymbolTable) (count int) {
	if source == nil {
		return
	}

	for k, attributes := range source.symbols {
		if strings.HasPrefix(k, defs.ReadonlyVariablePrefix) {
			continue
		}

		v := source.getValue(attributes.slot)
		s.SetAlways(k, v)

		count++
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

// For a given symbol table, discard any variables that are marked as ephemeral.
// These are variables that are created for only a single use.
func (s *SymbolTable) DiscardEphemera() {
	for k, attributes := range s.symbols {
		if attributes.Ephemeral {
			s.Delete(k, true)
		}
	}
}

func (s *SymbolTable) MarkEphemeral(name string) error {
	var err error

	_, attr, found := s.GetWithAttributes(name)
	if found {
		attr.Ephemeral = true
	} else {
		err = errors.ErrUnknownSymbol.Clone().Context(name)
	}

	return err
}
