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
// object in the main program, so we just put a placeholder here.  The
// instance UUID will be overwritten during server invocation if a server
// UUID is already defined, else it will be this initialized value.
var rootBaseValues = []interface{}{
	"<copyright>",
	uuid.New().String(),
	false,
}

// This is a list of the values that are initially stored in the
// root symbol table. This includes enough additional slots for
// the designated maximum symbol table size. Note that this size
// is set at initialization time, so the max slots cannot be changed
// at runtime for this table.
var rootGrowthValues = make([]interface{}, SymbolAllocationSize-len(rootNames))

var rootInitialBin = append(rootBaseValues, rootGrowthValues...)

var rootValues = []*[]interface{}{
	&rootInitialBin,
}

// RootSymbolTable is the parent of all other tables. It is populated
// by the initialized structures above.
var RootSymbolTable = SymbolTable{
	Name:          "root",
	parent:        nil,
	scopeBoundary: true,
	symbols:       rootNames,
	size:          len(rootNames),
	values:        rootValues,
	isRoot:        true,
	shared:        true,
}

// SetGlobal sets a symbol value in the global symbol table.
func (s *SymbolTable) SetGlobal(name string, value interface{}) error {
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
