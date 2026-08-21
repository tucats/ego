package symbols

import (
	"strings"

	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/data"
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
//
// Concurrency note: a proxy is created fresh every time a package is referenced
// at runtime (see inPackageByteCode in internal/language/bytecode/package.go),
// which for a busy server can mean many goroutines each building their own
// proxy for the very same package at the same time. Every one of those proxies
// aliases the SAME underlying symbols map and values storage as the receiver
// table s, so they all need to be protected by the SAME lock -- otherwise two
// goroutines could each believe they safely hold "the" lock while actually
// holding two different, uncontended locks, and both mutate the shared map at
// once (this used to happen here: the old code gave the proxy its own brand
// new mutex instead of reusing s's, so the locking was decorative). Copying
// the *pointer* to s's mutex (rather than creating a new mutex) is what fixes
// that: proxy.mutex and s.mutex are now literally the same sync.RWMutex.
func (s *SymbolTable) NewChildProxy(parent *SymbolTable) *SymbolTable {
	s.shared.Store(true)

	proxy := &SymbolTable{
		Name:     "Proxy for " + s.Name,
		symbols:  s.symbols,
		values:   s.values,
		id:       newTableID(),
		parent:   parent,
		depth:    s.depth,
		boundary: false,
		isRoot:   false,
		isClone:  false,
		proxy:    true,
		// Share the original table's mutex (a pointer copy, not a new mutex) so
		// that locking the proxy and locking s serialize against each other.
		mutex: s.mutex,
	}
	proxy.shared.Store(true)

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
		return 0
	}

	// Take a read lock on source before looking at its symbols map. Without this,
	// Merge used to range over source.symbols completely unprotected, even when
	// source.shared was true -- so a table meant to be safe for concurrent access
	// (for example, a cached, compiled service's symbol table that is reused by
	// every subsequent request to the same REST endpoint, see
	// internal/server/services/cache.go's getCachedService) could have its
	// underlying Go map read here at the exact moment another goroutine was
	// writing to it elsewhere. Concurrent unsynchronized access to a Go map is
	// undefined behavior and can crash or hang the whole process. RLock/RUnlock
	// are no-ops when source.shared is false, so this costs nothing for tables
	// that really are private to one goroutine.
	source.RLock()
	defer source.RUnlock()

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
	newTable.shared.Store(false)
	newTable.boundary = s.boundary
	newTable.forPackage = s.forPackage
	// Fix bug (found while implementing PERFORMANCE.md Finding 1): this used
	// to unconditionally overwrite newTable.id with a second, freshly
	// generated ID here, discarding the one NewChildSymbolTable (above) had
	// just assigned. That was always a wasted ID generation - harmless
	// beyond the waste itself when IDs were cheap uuid.New() values used
	// only for logging, but there is no reason to keep it now that it's
	// been noticed, so the id NewChildSymbolTable already assigned is left
	// alone.
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
