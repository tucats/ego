package symbols

import (
	"fmt"
	"strings"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
)

const (
	noSlot   = -1
	notFound = "<not found>"
)

type UndefinedValue struct {
}

// Get retrieves a symbol from the current table or any parent
// table that exists.
func (s *SymbolTable) Get(name string) (interface{}, bool) {
	var v interface{}

	s.mutex.RLock()
	defer s.mutex.RUnlock()

	attr, found := s.symbols[name]
	if found {
		v = s.GetValue(attr.slot)
	}

	if !found && !s.IsRoot() {
		return s.parent.Get(name)
	}

	if ui.IsActive(ui.SymbolLogger) {
		status := notFound
		attr = &SymbolAttribute{}

		if found {
			status = data.Format(v)
			if len(status) > 60 {
				status = status[:57] + "..."
			}
		}

		quotedName := fmt.Sprintf("\"%s\"", name)
		ui.WriteLog(ui.SymbolLogger, "%-20s(%s), get       %-10s, slot %2d = %s",
			s.Name, s.id.String(), quotedName, attr.slot, status)
	}

	return v, found
}

// Get retrieves a symbol from the current table.
func (s *SymbolTable) GetLocal(name string) (interface{}, bool) {
	var v interface{}

	s.mutex.RLock()
	defer s.mutex.RUnlock()

	attr, found := s.symbols[name]
	if found {
		v = s.GetValue(attr.slot)
	}

	if ui.IsActive(ui.SymbolLogger) {
		status := notFound
		attr = &SymbolAttribute{}

		if found {
			status = data.Format(v)
			if len(status) > 60 {
				status = status[:57] + "..."
			}
		}

		quotedName := fmt.Sprintf("\"%s\"", name)
		ui.WriteLog(ui.SymbolLogger, "%-20s(%s), get       %-10s, slot %2d = %s",
			s.Name, s.id.String(), quotedName, attr.slot, status)
	}

	return v, found
}

// Get retrieves a symbol from the current table or any parent
// table that exists.
func (s *SymbolTable) GetWithAttributes(name string) (interface{}, *SymbolAttribute, bool) {
	var v interface{}

	s.mutex.RLock()
	defer s.mutex.RUnlock()

	attr, found := s.symbols[name]
	if found {
		v = s.GetValue(attr.slot)
	}

	if !found && !s.IsRoot() {
		return s.parent.GetWithAttributes(name)
	}

	if ui.IsActive(ui.SymbolLogger) {
		status := notFound
		if found {
			status = data.Format(v)
			if len(status) > 60 {
				status = status[:57] + "..."
			}
		}

		quotedName := fmt.Sprintf("\"%s\"", name)
		ui.WriteLog(ui.SymbolLogger, "%-20s(%s), get       %-10s, slot %2d = %s",
			s.Name, s.id.String(), quotedName, attr.slot, status)
	}

	return v, attr, found
}

// GetAddress retrieves the address of a symbol values from the
// current table or any parent table that exists.
func (s *SymbolTable) GetAddress(name string) (interface{}, bool) {
	var v interface{}

	s.mutex.RLock()
	defer s.mutex.RUnlock()

	attr, found := s.symbols[name]
	if found {
		v = s.AddressOfValue(attr.slot)
	}

	if !found && !s.IsRoot() {
		return s.parent.GetAddress(name)
	}

	ui.Log(ui.SymbolLogger, "%s(%s), get(&%s)", s.Name, s.id, name)

	return v, found
}

// SetConstant stores a constant for readonly use in the symbol table. Because this could be
// done from many different threads in a REST server mode, use a lock to serialize writes.
func (s *SymbolTable) SetConstant(name string, v interface{}) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Does it already exist and is it readonly? IF so, fail
	attr, ok := s.symbols[name]
	if ok && attr.Readonly {
		return errors.ErrReadOnlyValue.Context(name)
	}

	if !ok {
		attr = &SymbolAttribute{
			slot:     s.size,
			Readonly: true,
		}
		s.size++
		s.symbols[name] = attr
	}

	s.SetValue(attr.slot, v)

	if ui.IsActive(ui.SymbolLogger) {
		ui.WriteLog(ui.SymbolLogger, "%-20s(%s), constant  \"%s\" = %s",
			s.Name, s.id, name, data.Format(v))
	}

	return nil
}

// SetReadOnly can be used to set the read-only attribute of a symbol.
// This code locates the symbol anywhere in the scope tree and sets its
// value. It returns nil if this was successful, else a symbol-not-found
// error is reported.
func (s *SymbolTable) SetReadOnly(name string, flag bool) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	syms := s

	for syms != nil {
		attr, found := syms.symbols[name]
		if found {
			attr.Readonly = flag

			ui.Log(ui.SymbolLogger, "Marking %s in %s table, readonly=%v",
				name, syms.Name, flag)

			return nil
		}

		if !syms.IsRoot() {
			syms = syms.parent
		} else {
			break
		}
	}

	return errors.ErrUnknownSymbol.Context(name)
}

// SetAlways stores a symbol value in the local table. No value in
// any parent table is affected. This can be used for functions and
// readonly values.
func (s *SymbolTable) SetAlways(name string, v interface{}) {
	// Hack. If this is the "_rest_response" variable, we have
	// to find the right table to put it in, which may be different
	// that were we started.
	symbolTable := s

	if name == "_rest_response" {
		for symbolTable.parent != nil && symbolTable.parent.parent != nil {
			symbolTable = symbolTable.parent
		}
	}

	symbolTable.mutex.Lock()
	defer symbolTable.mutex.Unlock()

	readOnly := strings.HasPrefix(name, "_")

	// IF this doesn't exist, allocate more space in the values array
	attr, ok := symbolTable.symbols[name]
	if !ok {
		attr = &SymbolAttribute{slot: s.size}
		symbolTable.symbols[name] = attr
		s.size++
	}

	if readOnly {
		attr.Readonly = true
	}

	symbolTable.SetValue(attr.slot, v)

	if ui.IsActive(ui.SymbolLogger) && name != defs.Line && name != defs.Module {
		valueString := data.Format(v)
		if len(valueString) > 60 {
			valueString = valueString[:57] + "..."
		}

		quotedName := fmt.Sprintf("\"%s\"", name)
		ui.WriteLog(ui.SymbolLogger, "%-20s(%s), setalways %-10s, slot %2d = %s",
			s.Name, s.id, quotedName, attr.slot, valueString)
	}
}

// SetAlways stores a symbol value in the local table. No value in
// any parent table is affected. This can be used for functions and
// readonly values.
func (s *SymbolTable) SetWithAttributes(name string, v interface{}, newAttr SymbolAttribute) error {
	// Hack. If this is the "_rest_response" variable, we have
	// to find the right table to put it in, which may be different
	// that were we started.
	symbolTable := s

	if name == "_rest_response" {
		for symbolTable.parent != nil && symbolTable.parent.parent != nil {
			symbolTable = symbolTable.parent
		}
	}

	symbolTable.mutex.Lock()
	defer symbolTable.mutex.Unlock()

	// IF this doesn't exist, allocate more space in the values array, and
	// add it to the symbol table slot.
	attr, ok := symbolTable.symbols[name]
	if !ok {
		attr = &SymbolAttribute{slot: s.size}
		symbolTable.symbols[name] = attr

		s.size++
	}

	// Copy the attributes other than slot from the new attribute
	// set to this attribute set.
	savedSlot := attr.slot
	attr = &newAttr
	attr.slot = savedSlot

	// Store the value, and update the symbol table entry.
	symbolTable.SetValue(attr.slot, v)

	if ui.IsActive(ui.SymbolLogger) && name != defs.Line && name != defs.Module {
		valueString := data.Format(v)
		if len(valueString) > 60 {
			valueString = valueString[:57] + "..."
		}

		quotedName := fmt.Sprintf("\"%s\"", name)
		ui.WriteLog(ui.SymbolLogger, "%-20s(%s), setWithAttributes %-10s, slot %2d = %s, readonly=%v",
			s.Name, s.id, quotedName, attr.slot, valueString, attr.Readonly)
	}

	return nil
}

// Set stores a symbol value in the table where it was found.
func (s *SymbolTable) Set(name string, v interface{}) error {
	var old interface{}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	attr, found := s.symbols[name]
	if found {
		// ff the value exists, isn't undefined, and is readonly, we can do no more.
		old = s.GetValue(attr.slot)
		if _, ok := old.(UndefinedValue); !ok && attr.Readonly {
			return errors.ErrReadOnlyValue.Context(name)
		}

		// Check to be sure this isn't a restricted (function code) type
		// that we are not allowed to write over, ever.
		if _, ok := old.(func(*SymbolTable, []interface{}) (interface{}, error)); ok {
			return errors.ErrReadOnlyValue.Context(name)
		}
	}

	// It wasn't found, so we are going to see if we can ask the parent
	// symbol table to do the honors.
	if !found {
		// If there are no more tables, we have an error.
		if s.IsRoot() {
			return errors.ErrUnknownSymbol.Context(name)
		}
		// Otherwise, ask the parent to try to set the value.
		return s.parent.Set(name, v)
	}

	// If we are setting a readonly value, then make sure we are
	// setting a copy of the value, and for complex types, the value
	// is marked as readeonly.
	if strings.HasPrefix(name, "_") {
		attr.Readonly = true
		v = data.DeepCopy(v)

		switch actual := v.(type) {
		case *data.Array:
			actual.SetReadonly(true)
			v = actual

		case *data.Map:
			actual.SetReadonly(true)
			v = actual

		case *data.Struct:
			actual.SetReadonly(true)
			v = actual
		}
	}

	// Store the value in the slot, and if it was readonly, write
	// the symbol map attribute value back.
	s.SetValue(attr.slot, v)

	if attr.Readonly {
		s.symbols[name] = attr
	}

	if ui.IsActive(ui.SymbolLogger) {
		valueString := data.Format(v)
		if len(valueString) > 60 {
			valueString = valueString[:57] + "..."
		}

		quotedName := fmt.Sprintf("\"%s\"", name)
		ui.WriteLog(ui.SymbolLogger, "%-20s(%s), set       %-10s, slot %2d = %s",
			s.Name, s.id, quotedName, attr.slot, valueString)
	}

	return nil
}

// Delete removes a symbol from the table. Search from the local symbol
// up the parent tree until you find the symbol to delete. If the always
// flag is set, this deletes even if the name is marked as a readonly
// variable ("_" as the first character).
func (s *SymbolTable) Delete(name string, always bool) error {
	if len(name) == 0 {
		return errors.ErrInvalidSymbolName
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	attr, f := s.symbols[name]
	if !f {
		if s.IsRoot() {
			return errors.ErrUnknownSymbol.Context(name)
		}

		return s.parent.Delete(name, always)
	}

	if !always && attr.Readonly {
		return errors.ErrReadOnlyValue.Context(name)
	}

	delete(s.symbols, name)

	if ui.IsActive(ui.SymbolLogger) {
		ui.WriteLog(ui.SymbolLogger, "%s(%s), delete(%s)",
			s.Name, s.id, name)
	}

	return nil
}

// Create creates a symbol name in the table.
func (s *SymbolTable) Create(name string) error {
	if len(name) == 0 {
		return errors.ErrInvalidSymbolName
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	if _, found := s.symbols[name]; found {
		return errors.ErrSymbolExists.Context(name)
	}

	s.symbols[name] = &SymbolAttribute{
		slot:     s.size,
		Readonly: false,
	}

	s.SetValue(s.size, UndefinedValue{})
	s.size++

	if ui.IsActive(ui.SymbolLogger) {
		ui.WriteLog(ui.SymbolLogger, "%s(%s), create(%s) = nil[%d]",
			s.Name, s.id, name, s.size-1)
	}

	return nil
}

// IsConstant determines if a name is a constant or readonly value.
func (s *SymbolTable) IsConstant(name string) bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	attr, found := s.symbols[name]
	if found {
		return attr.Readonly
	}

	if !s.IsRoot() {
		return s.parent.IsConstant(name)
	}

	return false
}
