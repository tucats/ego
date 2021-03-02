package symbols

import "sync"

// This is the number of symbols that can be added to a table at a
// given scope. Exported because it can be set by a caller prior
// to constructing a symbol table.
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
	ValueSize     int
	ScopeBoundary bool
	mutex         sync.Mutex
}

// NewSymbolTable generates a new symbol table.
func NewSymbolTable(name string) *SymbolTable {
	symbols := SymbolTable{
		Name:      name,
		Parent:    &RootSymbolTable,
		Symbols:   map[string]int{},
		Constants: map[string]interface{}{},
	}
	syms := &symbols
	syms.initializeValues()

	return syms
}

// NewChildSymbolTableWithSize generates a new symbol table with an assigned
// parent table. The table is created with a default capacity.
func NewChildSymbolTable(name string, parent *SymbolTable) *SymbolTable {
	symbols := SymbolTable{
		Name:      name,
		Parent:    parent,
		Symbols:   map[string]int{},
		Constants: map[string]interface{}{},
	}

	syms := &symbols
	syms.initializeValues()

	return &symbols
}
