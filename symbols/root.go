package symbols

import (
	"github.com/google/uuid"
)

// This is the list of symbols that are initialized in the root
// symbol table. These must match the values in rootValues below.
// The slot numbers must be sequential starting at zero.
var rootNames = map[string]*SymbolAttribute{
	"_copyright": {
		Slot:     0,
		Readonly: true,
	},
	"_server_instance": {
		Slot:     1,
		Readonly: true,
	},
}

var rootBaseValues = []interface{}{
	"(c) Copyright 2020, 2021, 2022",
	uuid.NewString(),
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
	Parent:        nil,
	ScopeBoundary: true,
	Symbols:       rootNames,
	size:          len(rootNames),
	values:        rootValues,
	isRoot:        true,
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

	return s.Parent == nil
}
