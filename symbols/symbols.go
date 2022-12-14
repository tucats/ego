package symbols

import (
	"fmt"
	"strings"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
)

const NoSlot = -1

// Get retrieves a symbol from the current table or any parent
// table that exists.
func (s *SymbolTable) Get(name string) (interface{}, bool) {
	var v interface{}

	s.mutex.RLock()
	defer s.mutex.RUnlock()

	attr, found := s.Symbols[name]
	if found {
		v = s.GetValue(attr.Slot)
	} else {
		v, found = s.Constants[name]
		attr.Slot = NoSlot
	}

	if !found && !s.IsRoot() {
		return s.Parent.Get(name)
	}

	if ui.LoggerIsActive(ui.SymbolLogger) {
		status := "<not found>"
		if found {
			status = datatypes.Format(v)
			if len(status) > 60 {
				status = status[:57] + "..."
			}
		}

		quotedName := fmt.Sprintf("\"%s\"", name)
		ui.Debug(ui.SymbolLogger, "%-20s(%s), get       %-10s, slot %2d = %s",
			s.Name, s.ID.String(), quotedName, attr.Slot, status)
	}

	return v, found
}

// GetAddress retrieves the address of a symbol values from the
// current table or any parent table that exists.
func (s *SymbolTable) GetAddress(name string) (interface{}, bool) {
	var v interface{}

	s.mutex.RLock()
	defer s.mutex.RUnlock()

	attr, found := s.Symbols[name]
	if found {
		v = s.AddressOfValue(attr.Slot)
	}

	if !found && !s.IsRoot() {
		return s.Parent.GetAddress(name)
	}

	if ui.LoggerIsActive(ui.SymbolLogger) {
		ui.Debug(ui.SymbolLogger, "%s(%s), get(&%s)",
			s.Name, s.ID, name)
	}

	return v, found
}

// SetConstant stores a constant for readonly use in the symbol table. Because this could be
// done from many different threads in a REST server mode, use a lock to serialize writes.
func (s *SymbolTable) SetConstant(name string, v interface{}) *errors.EgoError {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.Constants[name] = v

	if ui.LoggerIsActive(ui.SymbolLogger) {
		ui.Debug(ui.SymbolLogger, "%-20s(%s), constant  \"%s\" = %s",
			s.Name, s.ID, name, datatypes.Format(v))
	}

	return nil
}

// SetAlways stores a symbol value in the local table. No value in
// any parent table is affected. This can be used for functions and
// readonly values.
func (s *SymbolTable) SetAlways(name string, v interface{}) *errors.EgoError {
	// Hack. If this is the "_rest_response" variable, we have
	// to find the right table to put it in, which may be different
	// that were we started.
	symbolTable := s

	if name == "_rest_response" {
		for symbolTable.Parent != nil && symbolTable.Parent.Parent != nil {
			symbolTable = symbolTable.Parent
		}
	}

	// See if it's in the current constants table.
	if symbolTable.IsConstant(name) {
		return errors.New(errors.ErrReadOnlyValue).Context(name)
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	// IF this doesn't exist, allocate more space in the values array
	attr, ok := symbolTable.Symbols[name]
	if !ok {
		attr.Slot = s.ValueSize
		symbolTable.Symbols[name] = attr
		s.ValueSize++
	}

	symbolTable.SetValue(attr.Slot, v)

	if ui.LoggerIsActive(ui.SymbolLogger) && name != "__line" && name != "__module" {
		valueString := datatypes.Format(v)
		if len(valueString) > 60 {
			valueString = valueString[:57] + "..."
		}

		quotedName := fmt.Sprintf("\"%s\"", name)
		ui.Debug(ui.SymbolLogger, "%-20s(%s), setalways %-10s, slot %2d = %s",
			s.Name, s.ID, quotedName, attr.Slot, valueString)
	}

	return nil
}

// Set stores a symbol value in the table where it was found.
func (s *SymbolTable) Set(name string, v interface{}) *errors.EgoError {
	var old interface{}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	attr, found := s.Symbols[name]
	if found {
		old = s.GetValue(attr.Slot)

		if attr.Readonly {
			return errors.New(errors.ErrReadOnlyValue).Context(name)
		}
	}

	// If it was already there, we hae some additional checks to do
	// to be sure it's writable.
	if found {
		// Check to be sure this isn't a restricted (function code) type
		if _, ok := old.(func(*SymbolTable, []interface{}) (interface{}, error)); ok {
			return errors.New(errors.ErrReadOnlyValue).Context(name)
		}
	} else {
		// If there are no more tables, we have an error.
		if s.IsRoot() {
			return errors.New(errors.ErrUnknownSymbol).Context(name)
		}
		// Otherwise, ask the parent to try to set the value.
		return s.Parent.Set(name, v)
	}

	s.SetValue(attr.Slot, v)

	if strings.HasPrefix(name, "_") {
		attr.Readonly = true
		s.Symbols[name] = attr
	}

	if ui.LoggerIsActive(ui.SymbolLogger) {
		valueString := datatypes.Format(v)
		if len(valueString) > 60 {
			valueString = valueString[:57] + "..."
		}

		quotedName := fmt.Sprintf("\"%s\"", name)
		ui.Debug(ui.SymbolLogger, "%-20s(%s), set       %-10s, slot %2d = %s",
			s.Name, s.ID, quotedName, attr.Slot, valueString)
	}

	return nil
}

// Delete removes a symbol from the table. Search from the local symbol
// up the parent tree until you find the symbol to delete. If the always
// flag is set, this deletes even if the name is marked as a readonly
// variable ("_" as the first character).
func (s *SymbolTable) Delete(name string, always bool) *errors.EgoError {
	if len(name) == 0 {
		return errors.New(errors.ErrInvalidSymbolName)
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	attr, f := s.Symbols[name]
	if !f {
		if s.IsRoot() {
			return errors.New(errors.ErrUnknownSymbol).Context(name)
		}

		return s.Parent.Delete(name, always)
	}

	if !always && attr.Readonly {
		return errors.New(errors.ErrReadOnlyValue).Context(name)
	}

	delete(s.Symbols, name)

	if ui.LoggerIsActive(ui.SymbolLogger) {
		ui.Debug(ui.SymbolLogger, "%s(%s), delete(%s)",
			s.Name, s.ID, name)
	}

	return nil
}

// Create creates a symbol name in the table.
func (s *SymbolTable) Create(name string) *errors.EgoError {
	if len(name) == 0 {
		return errors.New(errors.ErrInvalidSymbolName)
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	if _, found := s.Symbols[name]; found {
		return errors.New(errors.ErrSymbolExists).Context(name)
	}

	s.Symbols[name] = SymbolAttribute{
		Slot:     s.ValueSize,
		Readonly: false,
	}

	s.SetValue(s.ValueSize, nil)
	s.ValueSize++

	if ui.LoggerIsActive(ui.SymbolLogger) {
		ui.Debug(ui.SymbolLogger, "%s(%s), create(%s) = nil[%d]",
			s.Name, s.ID, name, s.ValueSize-1)
	}

	return nil
}

// IsConstant determines if a name is a constant value.
func (s *SymbolTable) IsConstant(name string) bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if _, found := s.Constants[name]; found {
		return true
	} else if s.Parent != nil {
		return s.Parent.IsConstant(name)
	} else {
		return false
	}
}
