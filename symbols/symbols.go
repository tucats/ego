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
	}

	if !found && !s.IsRoot() {
		return s.Parent.Get(name)
	}

	if ui.LoggerIsActive(ui.SymbolLogger) {
		status := "<not found>"
		attr = &SymbolAttribute{}

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

// Get retrieves a symbol from the current table or any parent
// table that exists.
func (s *SymbolTable) GetWithAttributes(name string) (interface{}, *SymbolAttribute, bool) {
	var v interface{}

	s.mutex.RLock()
	defer s.mutex.RUnlock()

	attr, found := s.Symbols[name]
	if found {
		v = s.GetValue(attr.Slot)
	}

	if !found && !s.IsRoot() {
		return s.Parent.GetWithAttributes(name)
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

	return v, attr, found
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

	// Does it already exist and is it readonly? IF so, fail
	attr, ok := s.Symbols[name]
	if ok && attr.Readonly {
		return errors.New(errors.ErrReadOnlyValue).Context(name)
	}

	if !ok {
		attr = &SymbolAttribute{
			Slot:     s.ValueSize,
			Readonly: true,
		}
		s.ValueSize++
		s.Symbols[name] = attr
	}

	s.SetValue(attr.Slot, v)

	if ui.LoggerIsActive(ui.SymbolLogger) {
		ui.Debug(ui.SymbolLogger, "%-20s(%s), constant  \"%s\" = %s",
			s.Name, s.ID, name, datatypes.Format(v))
	}

	return nil
}

// SetReadOnly can be used to set the read-only attribute of a symbol.
// This code locates the symbol anywhere in the scope tree and sets its
// value. It returns nil if this was successful, else a symbol-not-found
// error is reported.
func (s *SymbolTable) SetReadOnly(name string, flag bool) *errors.EgoError {
	syms := s

	for syms != nil {
		attr, found := syms.Symbols[name]
		if found {
			attr.Readonly = flag

			ui.Debug(ui.SymbolLogger, "Marking %s in %s table, readonly=%v",
				name, syms.Name, flag)

			return nil
		}

		if !syms.IsRoot() {
			syms = syms.Parent
		} else {
			break
		}
	}

	return errors.New(errors.ErrUnknownSymbol).Context(name)
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

	s.mutex.Lock()
	defer s.mutex.Unlock()

	readOnly := strings.HasPrefix(name, "_")

	// IF this doesn't exist, allocate more space in the values array
	attr, ok := symbolTable.Symbols[name]
	if !ok {
		attr = &SymbolAttribute{Slot: s.ValueSize}
		symbolTable.Symbols[name] = attr
		s.ValueSize++
	}

	if readOnly {
		attr.Readonly = true
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

// SetAlways stores a symbol value in the local table. No value in
// any parent table is affected. This can be used for functions and
// readonly values.
func (s *SymbolTable) SetWithAttributes(name string, v interface{}, newAttr SymbolAttribute) *errors.EgoError {
	// Hack. If this is the "_rest_response" variable, we have
	// to find the right table to put it in, which may be different
	// that were we started.
	symbolTable := s

	if name == "_rest_response" {
		for symbolTable.Parent != nil && symbolTable.Parent.Parent != nil {
			symbolTable = symbolTable.Parent
		}
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	// IF this doesn't exist, allocate more space in the values array, and
	// add it to the symbol table slot.
	attr, ok := symbolTable.Symbols[name]
	if !ok {
		attr = &SymbolAttribute{Slot: s.ValueSize}
		symbolTable.Symbols[name] = attr

		s.ValueSize++
	}

	// Copy the attributes other than slot from the new attribute
	// set to this attribute set.
	attr.Readonly = newAttr.Readonly

	// Store the value, and update the symbol table entry.
	symbolTable.SetValue(attr.Slot, v)

	if ui.LoggerIsActive(ui.SymbolLogger) && name != "__line" && name != "__module" {
		valueString := datatypes.Format(v)
		if len(valueString) > 60 {
			valueString = valueString[:57] + "..."
		}

		quotedName := fmt.Sprintf("\"%s\"", name)
		ui.Debug(ui.SymbolLogger, "%-20s(%s), setWithAttributes %-10s, slot %2d = %s, readonly=%v",
			s.Name, s.ID, quotedName, attr.Slot, valueString, attr.Readonly)
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
		// Of the value exists and is readonly, we can do no more.
		if attr.Readonly {
			return errors.New(errors.ErrReadOnlyValue).Context(name)
		}

		// Check to be sure this isn't a restricted (function code) type
		// that we are not allowed to write over, ever.
		old = s.GetValue(attr.Slot)
		if _, ok := old.(func(*SymbolTable, []interface{}) (interface{}, error)); ok {
			return errors.New(errors.ErrReadOnlyValue).Context(name)
		}
	}

	if !found {
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

	s.Symbols[name] = &SymbolAttribute{
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

// IsConstant determines if a name is a constant or readonly value.
func (s *SymbolTable) IsConstant(name string) bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	attr, found := s.Symbols[name]
	if found {
		return attr.Readonly
	}

	if !s.IsRoot() {
		return s.Parent.IsConstant(name)
	} else {
		return false
	}
}
