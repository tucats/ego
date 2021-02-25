package symbols

import (
	"sync"

	"github.com/google/uuid"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/util"
)

// This is the maximum number of symbols that can be created at any
// given scope. Exported because it can be set by a caller prior
// to constructing a symbol table.
var MaxSymbolsPerScope = 100

// No symbol table will be smaller than this size. Exported because
// it can be set by a caller prior to constructing a symbol table.
const MinSymbolTableSize = 25

// SymbolTable contains an abstract symbol table.
type SymbolTable struct {
	Name          string
	Parent        *SymbolTable
	Symbols       map[string]int
	Constants     map[string]interface{}
	Values        []interface{}
	ValueSize     int
	ScopeBoundary bool
	mutex         sync.Mutex
}

// This is the list of symbols that are initialized in the root
// symbol table. These must match the values in rootValues below.
// The slot numbers must be sequential starting at zero.
var rootNames = map[string]int{
	"_author":    0,
	"_copyright": 1,
	"_session":   2,
	"_config":    3,
}

// This is a list of the values that are initially stored in the
// root symbol table. This includes enough additional slots for
// the designated maximum symbol table size. Note that this size
// is set at initialization time, so the max slots cannot be changed
// at runtime for this table.
var rootValues = append([]interface{}{
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
}, make([]interface{}, MaxSymbolsPerScope-len(rootNames))...)

// RootSymbolTable is the parent of all other tables.
var RootSymbolTable = SymbolTable{
	Name:          "Root Symbol Table",
	Parent:        nil,
	ScopeBoundary: true,
	Symbols:       rootNames,
	ValueSize:     len(rootNames),
	Values:        rootValues,
}

// NewSymbolTable generates a new symbol table.
func NewSymbolTable(name string) *SymbolTable {
	symbols := SymbolTable{
		Name:      name,
		Parent:    &RootSymbolTable,
		Symbols:   map[string]int{},
		Values:    make([]interface{}, MaxSymbolsPerScope),
		Constants: map[string]interface{}{},
	}

	return &symbols
}

// NewChildSymbolTable generates a new symbol table with an assigned
// parent table.
func NewChildSymbolTable(name string, parent *SymbolTable) *SymbolTable {
	symbols := SymbolTable{
		Name:      name,
		Parent:    parent,
		Symbols:   map[string]int{},
		Values:    make([]interface{}, MaxSymbolsPerScope),
		Constants: map[string]interface{}{},
	}

	return &symbols
}

// SetGlobal sets a symbol value in the global symbol table.
func (s *SymbolTable) SetGlobal(name string, value interface{}) *errors.EgoError {
	_ = RootSymbolTable.Create(name)

	return RootSymbolTable.SetAlways(name, value)
}

// Get retrieves a symbol from the current table or any parent
// table that exists.
func (s *SymbolTable) Get(name string) (interface{}, bool) {
	var v interface{}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	vx, f := s.Symbols[name]
	if f {
		v = s.Values[vx]
	}

	if !f {
		v, f = s.Constants[name]
	}

	if !f && s.Parent != nil {
		return s.Parent.Get(name)
	}

	ui.Debug(ui.SymbolLogger, "+++ in table %s, get(%s) = %v [%d]",
		s.Name, name, v, vx)

	return v, f
}

// GetAddress retrieves the address of a symbol values from the
// current table or any parent table that exists.
func (s *SymbolTable) GetAddress(name string) (interface{}, bool) {
	var v interface{}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	vx, f := s.Symbols[name]
	if f {
		v = &s.Values[vx]
	}

	if !f && s.Parent != nil {
		return s.Parent.Get(name)
	}

	ui.Debug(ui.SymbolLogger, "+++ in table %s, get(&%s) = %v",
		s.Name, name, util.Format(v))

	return v, f
}

// SetConstant stores a constant for readonly use in the symbol table. Because this could be
// done from many different threads in a REST server mode, use a lock to serialize writes.
func (s *SymbolTable) SetConstant(name string, v interface{}) *errors.EgoError {
	s.mutex.Lock()

	defer s.mutex.Unlock()

	if s.Constants == nil {
		s.Constants = map[string]interface{}{}
	}

	s.Constants[name] = v

	return nil
}

// SetAlways stores a symbol value in the local table. No value in
// any parent table is affected. This can be used for functions and
// readonly values.
func (s *SymbolTable) SetAlways(name string, v interface{}) *errors.EgoError {
	if s.Symbols == nil {
		s.Symbols = map[string]int{}
		s.Values = make([]interface{}, MaxSymbolsPerScope)
	}

	// Hack. If this is the "_rest_response" variable, we have
	// to find the right table to put it in, which may be different
	// that were we started.
	syms := s

	if name == "_rest_response" {
		for syms.Parent != nil && syms.Parent.Parent != nil {
			syms = syms.Parent
		}
	}

	// See if it's in the current constants table.
	if syms.IsConstant(name) {
		return errors.New(errors.ReadOnlyValueError).Context(name)
	}

	// IF this doesn't exist, allocate more space in the values array
	vx, ok := syms.Symbols[name]
	if !ok {
		if s.ValueSize >= len(s.Values) {
			return errors.New(errors.TooManyLocalSymbols)
		}

		vx = s.ValueSize
		syms.Symbols[name] = s.ValueSize
		s.ValueSize++
	}

	syms.Values[vx] = v

	ui.Debug(ui.SymbolLogger, "+++ in table %s, setalways(%s) = %v [%d]",
		s.Name, name, util.Format(v), vx)

	return nil
}

// Set stores a symbol value in the table where it was found.
func (s *SymbolTable) Set(name string, v interface{}) *errors.EgoError {
	if s.Symbols == nil {
		s.Symbols = map[string]int{}
		s.Values = make([]interface{}, MaxSymbolsPerScope)
	}

	var old interface{}

	oldx, found := s.Symbols[name]
	if found {
		old = s.Values[oldx]
	}

	// If it was already there, we hae some additional checks to do
	// to be sure it's writable.
	if found {
		if old != nil && name[0:1] == "_" {
			return errors.New(errors.ReadOnlyValueError).Context(name)
		}
		// Check to be sure this isn't a restricted (function code) type
		if _, ok := old.(func(*SymbolTable, []interface{}) (interface{}, error)); ok {
			return errors.New(errors.ReadOnlyValueError).Context(name)
		}
	} else {
		// If there are no more tables, we have an error.
		if s.Parent == nil {
			return errors.New(errors.UnknownSymbolError).Context(name)
		}
		// Otherwise, ask the parent to try to set the value.
		return s.Parent.Set(name, v)
	}

	s.Values[oldx] = v

	ui.Debug(ui.SymbolLogger, "+++ in table %s, set(%s) = %v [%d]",
		s.Name, name, util.Format(v), oldx)

	return nil
}

// Delete removes a symbol from the table. Search from the local symbol
// up the parent tree until you find the symbol to delete.
func (s *SymbolTable) Delete(name string) *errors.EgoError {
	if len(name) == 0 {
		return errors.New(errors.InvalidSymbolError)
	}

	if name[:1] == "_" {
		return errors.New(errors.ReadOnlyValueError).Context(name)
	}

	if s.Symbols == nil {
		return errors.New(errors.UnknownSymbolError).Context(name)
	}

	_, f := s.Symbols[name]
	if !f {
		if s.Parent == nil {
			return errors.New(errors.UnknownSymbolError).Context(name)
		}

		return s.Parent.Delete(name)
	}

	delete(s.Symbols, name)

	return nil
}

// DeleteAlways removes a symbol from the table. Search from the local symbol
// up the parent tree until you find the symbol to delete.
func (s *SymbolTable) DeleteAlways(name string) *errors.EgoError {
	if len(name) == 0 {
		return errors.New(errors.InvalidSymbolError)
	}

	if s.Symbols == nil {
		return errors.New(errors.UnknownSymbolError).Context(name)
	}

	_, f := s.Symbols[name]
	if !f {
		if s.Parent == nil {
			return errors.New(errors.UnknownSymbolError).Context(name)
		}

		return s.Parent.DeleteAlways(name)
	}

	delete(s.Symbols, name)

	return nil
}

// Create creates a symbol name in the table.
func (s *SymbolTable) Create(name string) *errors.EgoError {
	if len(name) == 0 {
		return errors.New(errors.InvalidSymbolError)
	}

	_, found := s.Symbols[name]
	if found {
		return errors.New(errors.SymbolExistsError).Context(name)
	}

	if s.ValueSize >= len(s.Values) {
		return errors.New(errors.TooManyLocalSymbols)
	}

	s.Symbols[name] = s.ValueSize
	s.Values[s.ValueSize] = nil
	s.ValueSize++

	return nil
}

// IsConstant determines if a name is a constant value.
func (s *SymbolTable) IsConstant(name string) bool {
	if s.Constants != nil {
		s.mutex.Lock()

		defer s.mutex.Unlock()

		_, found := s.Constants[name]

		if found {
			return true
		}

		if s.Parent != nil {
			return s.Parent.IsConstant(name)
		}
	}

	return false
}
