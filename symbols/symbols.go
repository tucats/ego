package symbols

import (
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/util"
)

// Get retrieves a symbol from the current table or any parent
// table that exists.
func (s *SymbolTable) Get(name string) (interface{}, bool) {
	var v interface{}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	vx, f := s.Symbols[name]
	if f {
		v = s.GetValue(vx)
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
		v = s.AddressOfValue(vx)
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
		vx = s.ValueSize
		syms.Symbols[name] = s.ValueSize
		s.ValueSize++
	}

	syms.SetValue(vx, v)

	ui.Debug(ui.SymbolLogger, "+++ in table %s, setalways(%s) = %v [%d]",
		s.Name, name, util.Format(v), vx)

	return nil
}

// Set stores a symbol value in the table where it was found.
func (s *SymbolTable) Set(name string, v interface{}) *errors.EgoError {
	var old interface{}

	oldx, found := s.Symbols[name]
	if found {
		old = s.GetValue(oldx)
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

	s.SetValue(oldx, v)

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

	_, f := s.Symbols[name]
	if !f {
		if s.Parent == nil {
			return errors.New(errors.UnknownSymbolError).Context(name)
		}

		return s.Parent.Delete(name)
	}

	delete(s.Symbols, name)
	ui.Debug(ui.SymbolLogger, "+++ in table %s, delete(%s)",
		s.Name, name)

	return nil
}

// DeleteAlways removes a symbol from the table. Search from the local symbol
// up the parent tree until you find the symbol to delete.
func (s *SymbolTable) DeleteAlways(name string) *errors.EgoError {
	if len(name) == 0 {
		return errors.New(errors.InvalidSymbolError)
	}

	_, f := s.Symbols[name]
	if !f {
		if s.Parent == nil {
			return errors.New(errors.UnknownSymbolError).Context(name)
		}

		return s.Parent.DeleteAlways(name)
	}

	delete(s.Symbols, name)
	ui.Debug(ui.SymbolLogger, "+++ in table %s, delete(%s)",
		s.Name, name)

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

	s.Symbols[name] = s.ValueSize
	s.SetValue(s.ValueSize, nil)
	s.ValueSize++

	ui.Debug(ui.SymbolLogger, "+++ in table %s, create(%s) = nil[%d]",
		s.Name, name, s.ValueSize-1)

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
