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

// No symbol table allocation extent will be smaller than this size.
// Exported because it is referenced by CLI handlers.
const MinSymbolAllocationSize = 16

type SymbolAttribute struct {
	Slot     int
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
	}

	if parent == nil {
		symbols.scopeBoundary = true
		symbols.isRoot = true
	}

	symbols.initializeValues()

	return &symbols
}

// Lock locks the symbol table so it cannot be used concurrently.
func (s *SymbolTable) Lock() {
	s.mutex.Lock()
}

// Unlock unlocks the symbol table for concurrent use.
func (s *SymbolTable) Unlock() {
	s.mutex.Unlock()
}

// Parent retrieves the parent symbol table of this table. If there
// is no parent table, nil is returned.
func (s *SymbolTable) Parent() *SymbolTable {
	return s.parent
}

// SetParent sets the parent of the currnent table to the provided
// table.
func (s *SymbolTable) SetParent(p *SymbolTable) *SymbolTable {
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
	result := make([]string, s.size)
	index := 0

	for k := range s.symbols {
		result[index] = k
		index++
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
	return len(s.symbols)
}

// Root finds the root table for the symbol table, by searching up
// the tree of tables until it finds the root table.
func (s *SymbolTable) Root() *SymbolTable {
	st := s
	for !st.IsRoot() {
		st = st.parent
	}

	ui.Debug(ui.SymbolLogger, "+++ Root of %s(%s): %s(%s)",
		s.Name, s.id, st.Name, st.id)

	return st
}
