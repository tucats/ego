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
	depth         int
	scopeBoundary bool
	isRoot        bool
	shared        bool
	boundary      bool
	isClone       bool
	modified      bool
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
		depth:   0,
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
		symbols: map[string]*SymbolAttribute{},
		id:      uuid.New(),
		shared:  alwaysShared,
	}

	symbols.SetParent(parent)

	if parent == nil {
		symbols.scopeBoundary = true
		symbols.isRoot = true
		symbols.depth = 0
	} else {
		symbols.depth = parent.depth + 1
	}

	symbols.initializeValues()

	return &symbols
}

func (s *SymbolTable) IsModified() bool {
	return s.modified
}

func (s *SymbolTable) IsClone() bool {
	return s.isClone
}

// Boundary sets the scope boundary of the symbol table. A scope boundary
// means that a search for a symbol will stop at this location, and then
// skip to the unbounded tables at the top of the tree.
func (s *SymbolTable) Boundary(flag bool) *SymbolTable {
	if s == nil {
		return s
	}

	s.boundary = flag

	return s
}

func (s *SymbolTable) GetBoundary() bool {
	if s == nil {
		return false
	}

	return s.boundary
}

// FindNextScope searches for the next parent scope that can be used
// within the current scope boundary. If we hit a scope boundary, then
// the function locations the top of the table that is unbounded.
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
		if p.boundary {
			lastBoundaryParent = p.parent
		}

		p = p.parent
	}

	ui.Log(ui.SymbolLogger, "[0] Symbol scope traversed boundary at %s (%d), skip to %s (%d)", s.Name, s.depth, lastBoundaryParent.Name, lastBoundaryParent.depth)

	return lastBoundaryParent
}

// Shared marke this symbol table as being able to be shared
// by multiple threads or go routines. When set, it causes
// extra read/write locking to be done on the table to prevent
// collisions in the table maps. Set the flag to true if you want
// this table (and all it's parents) to support sharing.
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

func (s *SymbolTable) IsShared() bool {
	if s == nil {
		return false
	}

	return s.shared
}

// SharedParent returns the symbol table in the tree where
// sharing starts. This can be used to reach up the tree to
// prune off the non-shared tables from the scope of a go
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

	ui.Log(ui.SymbolLogger, "Setting parent of table %s to %s", s.Name, pName)

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
// symbols in the table.
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

	ui.Log(ui.SymbolLogger, "+++ Root of %s(%s): %s(%s)",
		s.Name, s.id, st.Name, st.id)

	return st
}
