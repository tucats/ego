package symbols

import (
	"github.com/google/uuid"
	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
)

// This is the list of symbols that are initialized in the root
// symbol table. These must match the values in rootValues below.
// The slot numbers must be sequential starting at zero.
var rootNames = map[string]int{
	"_author":    0,
	"_copyright": 1,
	"_session":   2,
	"_config":    3,
}

var rootBaseValues = []interface{}{
	"Tom Cole",
	"(c) Copyright 2020, 2021",
	uuid.NewString(),
	map[string]interface{}{
		"disassemble": false,
		"trace":       false,
		datatypes.MetadataKey: map[string]interface{}{
			datatypes.TypeMDKey: "config",
		},
	},
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
	Name:          "Root Symbol Table",
	Parent:        nil,
	ScopeBoundary: true,
	Symbols:       rootNames,
	ValueSize:     len(rootNames),
	Values:        rootValues,
	Constants:     map[string]interface{}{},
	isRoot:        true,
}

// SetGlobal sets a symbol value in the global symbol table.
func (s *SymbolTable) SetGlobal(name string, value interface{}) *errors.EgoError {
	_ = RootSymbolTable.Create(name)

	return RootSymbolTable.SetAlways(name, value)
}
