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

// IsProxy reports whether this symbol table is a proxy. A proxy table shares
// its symbol dictionary and value storage with another table but has its own
// parent pointer, allowing it to sit at a different position in the scope chain.
func (s *SymbolTable) IsProxy() bool {
	return s.proxy
}

// CopyPackagesFromTable copies all package symbols from source into the receiver
// table. The underlying package data is shared between both tables (not deep-copied),
// and both copies are marked read-only to prevent either side from mutating shared state.
// Returns the number of package symbols copied.
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

// Merge copies all non-readonly symbols from source into the receiver table.
// Symbols whose names begin with the readonly prefix (typically "_") are skipped.
// Returns the number of symbols merged.
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

// Clone creates an independent copy of the receiver table attached to the given
// parent. Each symbol and its value is copied into a fresh table; the clone does
// not share storage with the original. The returned table is marked as a clone so
// callers can detect that further changes to it will not be reflected in the source.
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

// MarkEphemeral marks a named symbol as ephemeral. Ephemeral symbols are
// automatically removed when DiscardEphemera is called (typically at the end
// of a scope), making them useful for temporary values that should not outlive
// the current block. Returns an error if the symbol does not exist.
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
