package symbols

import (
	"sync"

	"github.com/google/uuid"
	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
)

// SymbolTable contains an abstract symbol table.
type SymbolTable struct {
	Name          string
	Parent        *SymbolTable
	Symbols       map[string]interface{}
	Constants     map[string]interface{}
	ScopeBoundary bool
}

// constLock is used to protect concurrent access to the constant
// symbol table, which is shared.
var constLock sync.Mutex

// RootSymbolTable is the parent of all other tables.
var RootSymbolTable = SymbolTable{
	Name:          "Root Symbol Table",
	Parent:        nil,
	ScopeBoundary: true,
	Symbols: map[string]interface{}{
		"_author":    "Tom Cole",
		"_copyright": "(c) Copyright 2020",
		"_session":   uuid.New().String(),
		"_config": map[string]interface{}{
			"disassemble": false,
			"trace":       false,
			datatypes.MetadataKey: map[string]interface{}{
				datatypes.TypeMDKey: "config",
			},
		},
	},
}

// NewSymbolTable generates a new symbol table.
func NewSymbolTable(name string) *SymbolTable {
	symbols := SymbolTable{
		Name:      name,
		Parent:    &RootSymbolTable,
		Symbols:   map[string]interface{}{},
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
		Symbols:   map[string]interface{}{},
		Constants: map[string]interface{}{},
	}

	return &symbols
}

// SetGlobal sets a symbol value in the global symbol table.
func (s *SymbolTable) SetGlobal(name string, value interface{}) error {
	_ = RootSymbolTable.Create(name)

	return RootSymbolTable.SetAlways(name, value)
}

// Get retrieves a symbol from the current table or any parent
// table that exists.
func (s *SymbolTable) Get(name string) (interface{}, bool) {
	v, f := s.Symbols[name]

	constLock.Lock()

	if !f {
		v, f = s.Constants[name]
	}

	constLock.Unlock()

	if !f && s.Parent != nil {
		return s.Parent.Get(name)
	}

	return v, f
}

// SetConstant stores a constant for readonly use in the symbol table. Because this could be
// done from many different threads in a REST server mode, use a lock to serialize writes.
func (s *SymbolTable) SetConstant(name string, v interface{}) error {
	constLock.Lock()

	if s.Constants == nil {
		s.Constants = map[string]interface{}{}
	}

	s.Constants[name] = v

	constLock.Unlock()

	return nil
}

// SetAlways stores a symbol value in the local table. No value in
// any parent table is affected. This can be used for functions and
// readonly values.
func (s *SymbolTable) SetAlways(name string, v interface{}) error {
	if s.Symbols == nil {
		s.Symbols = map[string]interface{}{}
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
		return errors.New(errors.ReadOnlyValueError).WithContext(name)
	}

	syms.Symbols[name] = v

	return nil
}

// Set stores a symbol value in the table where it was found.
func (s *SymbolTable) Set(name string, v interface{}) error {
	if s.Symbols == nil {
		s.Symbols = map[string]interface{}{}
	}

	old, found := s.Symbols[name]

	// If it was already there, we hae some additional checks to do
	// to be sure it's writable.
	if found {
		if old != nil && name[0:1] == "_" {
			return errors.New(errors.ReadOnlyValueError).WithContext(name)
		}
		// Check to be sure this isn't a restricted (function code) type
		if _, ok := old.(func(*SymbolTable, []interface{}) (interface{}, error)); ok {
			return errors.New(errors.ReadOnlyValueError).WithContext(name)
		}
	} else {
		// If there are no more tables, we have an error.
		if s.Parent == nil {
			return errors.New(errors.UnknownSymbolError).WithContext(name)
		}
		// Otherwise, ask the parent to try to set the value.
		return s.Parent.Set(name, v)
	}

	s.Symbols[name] = v

	return nil
}

// Delete removes a symbol from the table. Search from the local symbol
// up the parent tree until you find the symbol to delete.
func (s *SymbolTable) Delete(name string) error {
	if len(name) == 0 {
		return errors.New(errors.InvalidSymbolError)
	}

	if name[:1] == "_" {
		return errors.New(errors.ReadOnlyValueError).WithContext(name)
	}

	if s.Symbols == nil {
		return errors.New(errors.UnknownSymbolError).WithContext(name)
	}

	_, f := s.Symbols[name]
	if !f {
		if s.Parent == nil {
			return errors.New(errors.UnknownSymbolError).WithContext(name)
		}

		return s.Parent.Delete(name)
	}

	delete(s.Symbols, name)

	return nil
}

// DeleteAlways removes a symbol from the table. Search from the local symbol
// up the parent tree until you find the symbol to delete.
func (s *SymbolTable) DeleteAlways(name string) error {
	if len(name) == 0 {
		return errors.New(errors.InvalidSymbolError)
	}

	if s.Symbols == nil {
		return errors.New(errors.UnknownSymbolError).WithContext(name)
	}

	_, f := s.Symbols[name]
	if !f {
		if s.Parent == nil {
			return errors.New(errors.UnknownSymbolError).WithContext(name)
		}

		return s.Parent.DeleteAlways(name)
	}

	delete(s.Symbols, name)

	return nil
}

// Create creates a symbol name in the table.
func (s *SymbolTable) Create(name string) error {
	if len(name) == 0 {
		return errors.New(errors.InvalidSymbolError)
	}

	_, found := s.Symbols[name]
	if found {
		return errors.New(errors.SymbolExistsError).WithContext(name)
	}

	s.Symbols[name] = nil

	return nil
}

// IsConstant determines if a name is a constant value.
func (s *SymbolTable) IsConstant(name string) bool {
	if s.Constants != nil {
		constLock.Lock()

		_, found := s.Constants[name]

		constLock.Unlock()

		if found {
			return true
		}

		if s.Parent != nil {
			return s.Parent.IsConstant(name)
		}
	}

	return false
}
