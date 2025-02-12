package symbols

import (
	"fmt"
	"sort"
	"sync"

	"github.com/google/uuid"
	"github.com/tucats/ego/app-cli/ui"
)

// SymbolAllocationSize is the number of symbols that are allocated
// in each bin of a symbol table. Allocation within a bin is faster
// than creating a new bin, so this value should reflect the most
// common maximum size of a symbol table. Note that symbol tables are
// created for each basic block, so the idea value may be smaller than
// the number of symbols in a program. Exported because it can be set
// by a caller prior to constructing a symbol table. For example,both
// the RUN and SERVER RUN commands have command line options to set
// this value.
var SymbolAllocationSize = 32

// alwaysShared determines if new symbol tables are automatically marked
// as sharable, which incurs extra locking. The default if false, where
// tables are only shared if the individual sharing attribute is explicitly
// enabled.
var alwaysShared = true

// No symbol table allocation extent will be smaller than this size.
// Exported because it is referenced by CLI handlers.
const MinSymbolAllocationSize = 16

// SymbolAttribute is the object that defines information about a symbol. This
// includes private data that indicates where the value is stored, as well as
// any metadata about the symbol.
type SymbolAttribute struct {
	// The slot (location) in the values array of arrays that contains the value.
	// The slot number is divided by the length of each allocation to determine
	// the array eleemtn in the values array that contains the data, and the
	// remainder determines which element in the array slice contains the value.
	slot int

	// Flag that indicates this value is to be considered immutable regardless
	// of it's type or name.
	Readonly bool
}

// SymbolTable contains a symbol table. The symbol table maps names to storage of the
// corresponding values. It also manages metadata regarding whether the values are
// readonly, addressed by reference, etc.
type SymbolTable struct {
	// Name is the name of the symbol table. This is used for debugging and logging.
	Name string

	// The name of the package this symbol table belongs to. If this is not part of a
	// package definition, the package name is set to an empty string.
	forPackage string

	// The parent symbol table. If this is the root symbol table, the parent is nil.
	parent *SymbolTable

	// The map of all symbols defined in this table. The keys are the symbol names, which
	// must bre unique. The values are the SymbolAttribute objects, which defines both the
	// location of the value and metadata about the symbol.
	symbols map[string]*SymbolAttribute

	// The storage for symbol values. This is an array of array slices. Each slice contains the
	// values for each symbol. The number of slices grows as the symbol table grows; the intent
	// is that for most symbol tables a single slice should be sufficient. For very large tables,
	// the slices are increased. The value attribute contains information about which array in
	// the storage slice the value is stored.
	values []*[]interface{}

	// A unique identifier for the symbol table.
	id uuid.UUID

	// The current size of the symbol table.
	size int

	// The depth of the symbol table in terms of symbol scope. The root table is always at depth 0.
	// All it's children are at depth 1, and so forth.
	depth int

	// Flag indicat this table should be considered a root table, irrespective of whether it's parent
	// is nill. This is used to create artificial symbol scope isolation with packages.
	isRoot bool

	// Is this symbol table potentially shared by multiple go routines? If so, the symbol table manager
	// will use mutexes to serialize access to the symbol table. For tables that are not shared, the
	// flag is set to false. In these cases, the symbol table is NOT managed in a thread-safe manner.
	shared bool

	// Flag indicating if the symbol table has been marked as a boundary. A boundary is a symbol
	// table at which a scope search for a symbol stops, and skips to the top level scope above the
	// highest boundary frame. This allows scope searches to be limited to within a single
	// function's symbol table tree, for example.
	boundary bool

	// Is this symbol table potentially cloned by multiple go routines? If so, the symbol table manager
	// won't allow certain updates to be done to the table, since that would put it out of sync with the
	// original table it was copied from.
	isClone bool

	// Flag indicating whether the symbol table has been modified.
	modified bool

	// Flag indicating if this is a proxy symbol table. A proxy symbol table is a symbol table that
	// is used to "proxy" another symbol table. For example, a package symbol table can be shared
	// by many functions and threads, so the parent chain cannot be used with that table. So a proxy
	// table is created when a package table must be added to the chain, that points to the same
	// symbol dictionary and values storage as the table it proxies.
	proxy bool

	// The synchronization mutex used to serialize access to this table from multiple go routines. Only
	// used if the shared flag is true.
	mutex sync.RWMutex
}

// NewRootSymbolTable generates a new root symbol table. A root symbol table is any table that does not
// have a parent table.
func NewRootSymbolTable(name string) *SymbolTable {
	return NewChildSymbolTable(name, nil)
}

// NewSymbolTable generates a new symbol table. The table always has the global symbol table as it's parent.
// This initializes all the required storage for the symbol name dictionary and the values storage.
func NewSymbolTable(name string) *SymbolTable {
	symbols := SymbolTable{
		Name:    name,
		parent:  &RootSymbolTable,
		symbols: map[string]*SymbolAttribute{},
		id:      uuid.New(),
		depth:   0,
		shared:  alwaysShared,
	}
	symbols.initializeValues()

	return &symbols
}

// NewChildSymbolTableWithSize generates a new symbol table with an assigned parent table. The
// table is created with a default capacity.
func NewChildSymbolTable(name string, parent *SymbolTable) *SymbolTable {
	symbols := SymbolTable{
		Name:    name,
		symbols: map[string]*SymbolAttribute{},
		id:      uuid.New(),
		shared:  alwaysShared,
	}

	symbols.SetParent(parent)

	if parent == nil {
		symbols.boundary = true
		symbols.isRoot = true
		symbols.depth = 0
	} else {
		symbols.depth = parent.depth + 1
	}

	symbols.initializeValues()

	return &symbols
}

// NewChildProxy creates a new symbol table that points to the same dictionary
// and value data as the receiver table, and then binds it to the specified
// pqarent table. This allows the proxy to have a different parent table than
// the one it is a proxy for, without modifying the original table.
//
// This is primarily used to create a new symbol scope for a package symbol
// table, which might be shared between multiple invocations so the parent
// value cannot be written directly to the package table. But we want to be
// sure to use the same symbol dictionary and values storage.
func (s *SymbolTable) NewChildProxy(parent *SymbolTable) *SymbolTable {
	return &SymbolTable{
		Name:     "Proxy for " + s.Name,
		symbols:  s.symbols,
		values:   s.values,
		id:       uuid.New(),
		shared:   s.shared,
		parent:   parent,
		depth:    s.depth,
		boundary: false,
		isRoot:   false,
		isClone:  false,
		proxy:    true,
	}
}

func (s *SymbolTable) IsProxy() bool {
	return s.proxy
}

// IsModified returns whether the symbol table has been modified. This is used in package management to determine
// if compiling an imported file means the symbol table needs to be re-merged with the package master symbol table.
func (s *SymbolTable) IsModified() bool {
	return s.modified
}

// IsClone returns whether the symbol table has been cloned. This is used in package management to determine if
// compiling an imported file means the symbol table needs to be re-merged with the package master.
func (s *SymbolTable) IsClone() bool {
	return s.isClone
}

// Boundary sets the scope boundary of the symbol table. A scope boundary means that a search for a symbol
// will stop at this location, and then skip to the unbounded tables at the top of the tree. This is used to
// limit searches for symbols within a function to the symbol table tree for just that function, plus any
// global symbols (in those tables at the top of the tree above any boundary tables).
func (s *SymbolTable) Boundary(flag bool) *SymbolTable {
	if s == nil {
		return s
	}

	s.boundary = flag

	return s
}

// IsBounary returns whether the symbol table is a scope boundary.
func (s *SymbolTable) IsBoundary() bool {
	if s == nil {
		return false
	}

	return s.boundary
}

// FindNextScope searches for the next parent scope that can be used within the current scope
// boundary. If the search hits a scope boundary, then return top of the symbol table tree
// that is unbounded.
func (s *SymbolTable) FindNextScope() *SymbolTable {
	if s == nil || s.parent == nil || s.isRoot {
		return nil
	}

	// If this isn't a scope boundary, then we just return the parent.
	if !s.boundary {
		return s.parent
	}

	// It's a scope boundary, so we need to find the last boundary
	// in the chain and return that table's parent.
	p := s.parent
	lastBoundaryParent := p

	for p != nil {
		// Package symbol tables are always boundaries
		if p.forPackage != "" {
			return p
		}

		if p.boundary && p.parent != nil {
			lastBoundaryParent = p.parent
		}

		p = p.parent
	}

	ui.Log(ui.SymbolLogger, "symbols.boundary.skip", ui.A{
		"name":      s.Name,
		"depth":     s.depth,
		"next":      lastBoundaryParent.Name,
		"nextdepth": lastBoundaryParent.depth})

	return lastBoundaryParent
}

// Shared marke this symbol table as being able to be shared by multiple threads or go
// routines. When set, it causes extra read/write locking to be done on the table to prevent
// collisions in the table maps. Set the flag to true if you want this table (and all it's
// parents) to support sharing.
func (s *SymbolTable) Shared(flag bool) *SymbolTable {
	if s == nil {
		return s
	}

	if alwaysShared && !flag {
		s.shared = false

		return s
	}

	// Set the shared flag based on the user input, but overrriden
	// by the defaul tif necessary.
	s.shared = flag || alwaysShared

	// If we ended up setting this table to be shared, crawl up the
	// parent chain to set all symbol tables as shared that are
	// above us, as a get will do a crawl of the entire chain.
	if s.shared {
		p := s.parent
		for p != nil {
			p.shared = true
			p = p.parent
		}
	}

	return s
}

// IsShared returns whether the symbol table is shared. This function is used by the
// utility functions that can print a symbol tables.
func (s *SymbolTable) IsShared() bool {
	if s == nil {
		return false
	}

	return s.shared
}

// SharedParent returns the symbol table in the tree where sharing starts. This can be
// used to reach up the tree to prune off the non-shared tables from the scope of a go
// routine, for example.
func (s *SymbolTable) SharedParent() *SymbolTable {
	if s == nil {
		return nil
	}

	for s != nil && !s.shared {
		s = s.parent
	}

	return s
}

// Lock locks the symbol table so it cannot be used concurrently.
func (s *SymbolTable) Lock() *SymbolTable {
	if s == nil {
		return s
	}

	if s.shared {
		s.mutex.Lock()
	}

	return s
}

// Unlock unlocks the symbol table for concurrent use.
func (s *SymbolTable) Unlock() *SymbolTable {
	if s.shared {
		s.mutex.Unlock()
	}

	return s
}

// Lock locks the symbol table for readaing so it cannot be used concurrently.
func (s *SymbolTable) RLock() *SymbolTable {
	if s.shared {
		s.mutex.RLock()
	}

	return s
}

// Unlock unlocks the symbol table previouly readlocked.
func (s *SymbolTable) RUnlock() *SymbolTable {
	if s.shared {
		s.mutex.RUnlock()
	}

	return s
}

// Parent retrieves the parent symbol table of this table. If there
// is no parent table, nil is returned.
func (s *SymbolTable) Parent() *SymbolTable {
	if s == nil {
		return nil
	}

	return s.parent
}

// SetParent sets the parent of the currnent table to the provided
// table.
func (s *SymbolTable) SetParent(p *SymbolTable) *SymbolTable {
	if s == nil {
		return s
	}

	pName := "<root table>"
	if p != nil {
		pName = p.Name
	}

	ui.Log(ui.SymbolLogger, "symbols.set.parent", ui.A{
		"name":   s.Name,
		"parent": pName})

	// Chase the parent chain from the new parent to make sure this symbol table
	// is not already in the loop.
	chain := p
	for chain != nil {
		if chain == s {
			panic(fmt.Sprintf("+++ Symbol table loop detected attaching %s to %s", p.Name, s.Name))
		}

		chain = chain.parent
	}

	s.Lock()
	defer s.Unlock()

	s.parent = p
	s.isRoot = (p == nil)

	return s
}

// Package returns the package for which this symbol table provides
// symbol information. If there is no package, it returns an empty string.
func (s *SymbolTable) Package() string {
	return s.forPackage
}

// SetPackage sets the package name for this symbol table.
func (s *SymbolTable) SetPackage(name string) {
	s.forPackage = name
}

// ID returns the unique identifier for this symbol table.
func (s *SymbolTable) ID() uuid.UUID {
	if s == nil {
		return uuid.Nil
	}

	return s.id
}

// Names returns an array of strings containing the names of the
// symbols in the table, sorted in lexicographical order.
func (s *SymbolTable) Names() []string {
	s.Lock()
	defer s.Unlock()

	result := []string{}

	for k := range s.symbols {
		result = append(result, k)
	}

	// Sort the list so it is deterministic.
	sort.Strings(result)

	return result
}

// Size returns the number of symbols in the table.
func (s *SymbolTable) Size() int {
	s.RLock()
	defer s.RUnlock()

	return len(s.symbols)
}

// Root finds the root table for the symbol table, by searching up
// the tree of tables until it finds the root table.
func (s *SymbolTable) Root() *SymbolTable {
	s.RLock()
	defer s.RUnlock()

	st := s
	for !st.IsRoot() {
		st = st.parent
	}

	ui.Log(ui.SymbolLogger, "symbols.root", ui.A{
		"name":     s.Name,
		"id":       s.id,
		"parent":   st.Name,
		"parentid": st.id})

	return st
}
