package symbols

import (
	"github.com/google/uuid"
	"github.com/tucats/ego/defs"
)

// This is the list of symbols that are initialized in the root
// symbol table. These must match the values in rootValues below.
// The slot numbers must be sequential starting at zero.
var rootNames = map[string]*SymbolAttribute{
	defs.CopyrightVariable: {
		slot:     0,
		Readonly: true,
	},
	defs.InstanceUUIDVariable: {
		slot:     1,
		Readonly: true,
	},
	defs.UserCodeRunningVariable: {
		slot:     2,
		Readonly: true,
	},
}

// The copyright string is re-written during initialization of the App
// object in the main program, so we just put a placeholder here. The
// instance UUID will be overwritten during server invocation if a server
// UUID is already defined, else it will be this initialized value.
var rootBaseValues = []any{
	"<copyright>",
	uuid.New().String(),
	false,
}

// This is a list of the values that are initially stored in the
// root symbol table. This includes enough additional slots for
// the designated maximum symbol table size. Note that this size
// is set at initialization time, so the max slots cannot be changed
// at runtime for this table.
var rootGrowthValues = make([]any, SymbolAllocationSize-len(rootNames))

var rootInitialBin = append(rootBaseValues, rootGrowthValues...)

var rootValues = []*[]any{
	&rootInitialBin,
}

// RootSymbolTable is the parent of all other tables. It is populated by values that are
// generated, in part, during initialization, from static values. This is the only table
// that is not created by the NewTable function.
var RootSymbolTable = SymbolTable{
	Name:     "root",
	parent:   nil,
	boundary: true,
	symbols:  rootNames,
	size:     len(rootNames),
	values:   rootValues,
	isRoot:   true,
}

// init is a Go-specific function that is called one time during the
// initialization of the package. It acquires the RootSymbolTable's mutex to ensure
// that only one goroutine can modify the table at a time. This is necessary because
// concurrent writes to the table would otherwise lead to data races.
func init() {
	// RootSymbolTable is a global resource accessed by concurrent goroutines (HTTP
	// handlers compile and execute code simultaneously). Marking it shared here ensures
	// all Get/Set operations on it acquire the table's mutex. This cannot be done in the
	// var declaration because atomic.Bool cannot be set in a struct literal.
	RootSymbolTable.shared.Store(true)
}

// SetGlobal creates (if it does not already exist) and then sets a symbol in the
// process-wide root symbol table. Because RootSymbolTable is the ancestor of every
// other table, values stored here are visible throughout the entire program. Returns
// an error only if Create found the symbol already existed; the value is always written.
func (s *SymbolTable) SetGlobal(name string, value any) error {
	err := RootSymbolTable.Create(name)

	RootSymbolTable.SetAlways(name, value)

	return err
}

// IsRoot determines if the current symbol table is the root table, or
// is a root table because it has no parent.
func (s *SymbolTable) IsRoot() bool {
	if s.isRoot {
		return true
	}

	return s.parent == nil
}
