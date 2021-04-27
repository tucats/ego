package symbols

import (
	"sync"

	"github.com/google/uuid"
	"github.com/tucats/ego/app-cli/ui"
)

// SymbolAllocationSize is the number of symbols that can be added
// to a table at a given scope. Exported because it can be set by a
// caller prior to constructing a symbol table.
var SymbolAllocationSize = 64

// No symbol table allocation extent will be smaller than this size.
// Exported because it is referenced by CLI handlers.
const MinSymbolAllocationSize = 16

// SymbolTable contains an abstract symbol table.
type SymbolTable struct {
	Name          string
	Package       string
	Parent        *SymbolTable
	Symbols       map[string]int
	Constants     map[string]interface{}
	Values        []*[]interface{}
	ID            uuid.UUID
	ValueSize     int
	ScopeBoundary bool
	isRoot        bool
	mutex         sync.Mutex
}

func NewRootSymbolTable(name string) *SymbolTable {
	return NewChildSymbolTable(name, nil)
}

// NewSymbolTable generates a new symbol table.
func NewSymbolTable(name string) *SymbolTable {
	symbols := SymbolTable{
		Name:      name,
		Parent:    &RootSymbolTable,
		Symbols:   map[string]int{},
		Constants: map[string]interface{}{},
		ID:        uuid.New(),
	}
	symbols.initializeValues()

	return &symbols
}

// NewChildSymbolTableWithSize generates a new symbol table with an assigned
// parent table. The table is created with a default capacity.
func NewChildSymbolTable(name string, parent *SymbolTable) *SymbolTable {
	symbols := SymbolTable{
		Name:      name,
		Parent:    parent,
		Symbols:   map[string]int{},
		Constants: map[string]interface{}{},
		ID:        uuid.New(),
	}

	if parent == nil {
		symbols.ScopeBoundary = true
		symbols.isRoot = true
	}

	symbols.initializeValues()

	return &symbols
}

func (s *SymbolTable) Lock() {
	s.mutex.Lock()
}

func (s *SymbolTable) Unlock() {
	s.mutex.Unlock()
}

// Find the root table for this symbol table.
func (s *SymbolTable) Root() *SymbolTable {
	st := s
	for !st.isRoot && s.Parent != nil {
		st = st.Parent
	}

	ui.Debug(ui.SymbolLogger, "+++ Root of %s(%s): %s(%s)",
		s.Name, s.ID, st.Name, st.ID)

	return st
}
