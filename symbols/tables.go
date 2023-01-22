package symbols

import (
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
var alwaysShared = false

// No symbol table allocation extent will be smaller than this size.
// Exported because it is referenced by CLI handlers.
const MinSymbolAllocationSize = 16

// SymbolAttribute is the object that defines information about a
// symbol. This includes private data that indicates where the value
// is stored, as well as metadata about the symbol.
type SymbolAttribute struct {
	slot     int
	Readonly bool
}

// SymbolTable contains an abstract symbol table.
type SymbolTable struct {
	Name          string
	forPackage    string
	parent        *SymbolTable
	symbols       map[string]*SymbolAttribute
	values        []*[]interface{}
	id            uuid.UUID
	size          int
	scopeBoundary bool
	isRoot        bool
	shared        bool
	mutex         sync.RWMutex
}

func NewRootSymbolTable(name string) *SymbolTable {
	return NewChildSymbolTable(name, nil)
}

// NewSymbolTable generates a new symbol table.
func NewSymbolTable(name string) *SymbolTable {
	symbols := SymbolTable{
		Name:    name,
		parent:  &RootSymbolTable,
		symbols: map[string]*SymbolAttribute{},
		id:      uuid.New(),
		shared:  alwaysShared,
	}
	symbols.initializeValues()

	return &symbols
}

// NewChildSymbolTableWithSize generates a new symbol table with an assigned
// parent table. The table is created with a default capacity.
func NewChildSymbolTable(name string, parent *SymbolTable) *SymbolTable {
	symbols := SymbolTable{
		Name:    name,
		parent:  parent,
		symbols: map[string]*SymbolAttribute{},
		id:      uuid.New(),
		shared:  alwaysShared,
	}

	if parent == nil {
		symbols.scopeBoundary = true
		symbols.isRoot = true
	}

	symbols.initializeValues()

	return &symbols
}

// Shared marke this symbol table as being able to be shared
// by multiple threads or go routines. When set, it causes
// extra read/write locking to be done on the table to prevent
// collisions in the table maps. Set the flag to true if you want
// this table (and all it's parents) to support sharing.
func (s *SymbolTable) Shared(flag bool) *SymbolTable {
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

func (s *SymbolTable) IsShared() bool {
	return s.shared
}

// SharedParent returns the symbol table in the tree where
// sharing starts. This can be used to reach up the tree to
// prune off the non-shared tables from the scope of a go
// routine, for example.
func (s *SymbolTable) SharedParent() *SymbolTable {
	for s != nil && !s.shared {
		s = s.parent
	}

	return s
}

// Lock locks the symbol table so it cannot be used concurrently.
func (s *SymbolTable) Lock() *SymbolTable {
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
	return s.parent
}

// SetParent sets the parent of the currnent table to the provided
// table.
func (s *SymbolTable) SetParent(p *SymbolTable) *SymbolTable {
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
	return s.id
}

// Names returns an array of strings containing the names of the
// symbols in the table.
func (s *SymbolTable) Names() []string {
	s.Lock()
	defer s.Unlock()

	result := []string{}

	for k := range s.symbols {
		result = append(result, k)
	}

	return result
}

// ScopeBoundary returns a flag indicating if this symbol table
// represents a boundary for scope checking. It returns true if
// any traversal searching for symbols should be stopped at this
// point in the scope list.
func (s *SymbolTable) ScopeBoundary() bool {
	return s.scopeBoundary
}

// SetScopeBoundary indicates that this symbol table is meant to
// be a boundary point beyond which symbol scope cannot be examined.
func (s *SymbolTable) SetScopeBoundary(flag bool) {
	s.scopeBoundary = flag
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

	ui.Log(ui.SymbolLogger, "+++ Root of %s(%s): %s(%s)",
		s.Name, s.id, st.Name, st.id)

	return st
}
