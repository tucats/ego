// Package symbols provides symbol table functions for Ego.
//
// A symbol table is similar to a map in that it references
// elements using a string key. The symbol tables can be marked
// as shared, in which case they are always accessed in a
// thread-safe manner.
//
// Symbol tables can be nested, so a given table always has a
// parent table. This allows the language to support scope. A
// Get of the symbol table searches the current table and all
// it's parents. A create of a symbol is only done in the current
// scope.
//
// At the top level is a global symbol table, which is ultimately
// the parent of every other table (this allows setting global
// state information for the entire process in the global table).
package symbols

import (
	"strconv"
	"strings"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
)

const (
	noSlot   = -1
	notFound = "<not found>"
	elipses  = "..."
)

type UndefinedValue struct {
}

// Get retrieves a symbol from the current table or any parent
// table that exists.
func (s *SymbolTable) Get(name string) (interface{}, bool) {
	var v interface{}

	if s == nil {
		return nil, false
	}

	if s.shared {
		symbolTable := s.RLock()
		defer symbolTable.RUnlock()
	}

	attr, found := s.symbols[name]
	if found {
		v = s.GetValue(attr.slot)
	}

	if !found && !s.IsRoot() {
		if s.parent == nil || s.parent == s {
			panic("Symbol table parent infinite loop detected at " + s.Name)
		}

		if next := s.FindNextScope(); next == nil {
			return nil, false
		} else {
			return next.Get(name)
		}
	}

	if ui.IsActive(ui.SymbolLogger) {
		status := notFound
		attr = &SymbolAttribute{}

		if found {
			status = data.Format(v)
			if len(status) > 60 {
				status = status[:57] + elipses
			}
		}

		quotedName := strconv.Quote(name)
		ui.WriteLog(ui.SymbolLogger, "%-20s(%s), get       %-10s, slot %2d = %s",
			s.Name, s.id.String(), quotedName, attr.slot, status)
	}

	return v, found
}

// Get retrieves a symbol from the current table.
func (s *SymbolTable) GetLocal(name string) (interface{}, bool) {
	var v interface{}

	if s == nil {
		return nil, false
	}

	if s.shared {
		symbolTable := s.RLock()
		defer symbolTable.RUnlock()
	}

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
				status = status[:57] + elipses
			}
		}

		quotedName := strconv.Quote(name)
		ui.WriteLog(ui.SymbolLogger, "%-20s(%s), get       %-10s, slot %2d = %s",
			s.Name, s.id.String(), quotedName, attr.slot, status)
	}

	return v, found
}

// Get retrieves a symbol from the current table or any parent
// table that exists.
func (s *SymbolTable) GetWithAttributes(name string) (interface{}, *SymbolAttribute, bool) {
	var v interface{}

	if s == nil {
		return nil, nil, false
	}

	if s.shared {
		symbolTable := s.RLock()
		defer symbolTable.RUnlock()
	}

	attr, found := s.symbols[name]
	if found {
		v = s.GetValue(attr.slot)
	}

	if !found && !s.IsRoot() {
		if next := s.FindNextScope(); next == nil {
			return nil, nil, false
		} else {
			return next.GetWithAttributes(name)
		}
	}

	if ui.IsActive(ui.SymbolLogger) {
		status := notFound
		if found {
			status = data.Format(v)
			if len(status) > 60 {
				status = status[:57] + elipses
			}
		}

		quotedName := strconv.Quote(name)
		ui.WriteLog(ui.SymbolLogger, "%-20s(%s), get       %-10s, slot %2d = %s",
			s.Name, s.id.String(), quotedName, attr.slot, status)
	}

	return v, attr, found
}

// GetAddress retrieves the address of a symbol values from the
// current table or any parent table that exists.
func (s *SymbolTable) GetAddress(name string) (interface{}, bool) {
	var v interface{}

	if s == nil {
		return nil, false
	}

	if s.shared {
		symbolTable := s.RLock()
		defer symbolTable.RUnlock()
	}

	attr, found := s.symbols[name]
	if found {
		if name[0:1] == "_" {
			v = s.AddressOfImmuableValue(attr.slot)
		} else {
			v = s.AddressOfValue(attr.slot)
		}
	}

	if !found && !s.IsRoot() {
		if !found && !s.IsRoot() {
			if next := s.FindNextScope(); next == nil {
				return nil, false
			} else {
				return next.GetAddress(name)
			}
		}
	}

	ui.Log(ui.SymbolLogger, "%s(%s), get(&%s)", s.Name, s.id, name)

	return v, found
}

// SetConstant stores a constant for readonly use in the symbol table. Because this could be
// done from many different threads in a REST server mode, use a lock to serialize writes.
func (s *SymbolTable) SetConstant(name string, v interface{}) error {
	if s == nil {
		return errors.ErrNoSymbolTable.In("SetConstant")
	}

	if s.shared {
		s.Lock()
		defer s.Unlock()
	}

	// Wrap the value in the Immutable wrapper.
	v = data.Constant(v)

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
	if s == nil {
		return errors.ErrNoSymbolTable.In("SetReadOnly")
	}

	if s.shared {
		s.Lock()
		defer s.Unlock()
	}

	syms := s

	for syms != nil {
		attr, found := syms.symbols[name]
		if found {
			attr.Readonly = flag
			s.modified = true

			ui.Log(ui.SymbolLogger, "Marking %s in %s table, readonly=%v",
				name, syms.Name, flag)

			return nil
		}

		if !syms.IsRoot() {
			syms = s.FindNextScope()
		} else {
			break
		}
	}

	return errors.ErrUnknownSymbol.Context(name)
}

// SetAlways stores a symbol value in the local table. No value in
// any parent table is affected. This can be used for functions and
// readonly values.
func (s *SymbolTable) SetAlways(name string, v interface{}) *SymbolTable {
	if s == nil {
		return s
	}

	// Hack. If this is the defs.RestResponseName variable, we have
	// to find the right table to put it in, which may be different
	// that were we started.
	symbolTable := s

	if name == defs.RestResponseName {
		for symbolTable.parent != nil && symbolTable.parent.parent != nil {
			symbolTable = symbolTable.parent
		}
	}

	if s.shared {
		symbolTable.Lock()
		defer symbolTable.Unlock()
	}

	readOnly := strings.HasPrefix(name, defs.ReadonlyVariablePrefix)

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

	if ui.IsActive(ui.SymbolLogger) && name != defs.LineVariable && name != defs.ModuleVariable {
		valueString := data.Format(v)
		if len(valueString) > 60 {
			valueString = valueString[:57] + elipses
		}

		quotedName := strconv.Quote(name)
		ui.WriteLog(ui.SymbolLogger, "%-20s(%s), setalways %-10s, slot %2d = %s",
			s.Name, s.id, quotedName, attr.slot, valueString)
	}

	return s
}

// SetAlways stores a symbol value in the local table. No value in
// any parent table is affected. This can be used for functions and
// readonly values.
func (s *SymbolTable) SetWithAttributes(name string, v interface{}, newAttr SymbolAttribute) error {
	if s == nil {
		return errors.ErrNoSymbolTable.In("SetWithAttributes")
	}

	// Hack. If this is the defs.RestResponseName variable, we have
	// to find the right table to put it in, which may be different
	// that were we started.
	symbolTable := s

	if name == defs.RestResponseName {
		for symbolTable.parent != nil && symbolTable.parent.parent != nil {
			symbolTable = symbolTable.parent
		}
	}

	if s.shared {
		symbolTable.Lock()
		defer symbolTable.Unlock()
	}

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

	if ui.IsActive(ui.SymbolLogger) && name != defs.LineVariable && name != defs.ModuleVariable {
		valueString := data.Format(v)
		if len(valueString) > 60 {
			valueString = valueString[:57] + elipses
		}

		quotedName := strconv.Quote(name)
		ui.WriteLog(ui.SymbolLogger, "%-20s(%s), setWithAttributes %-10s, slot %2d = %s, readonly=%v",
			s.Name, s.id, quotedName, attr.slot, valueString, attr.Readonly)
	}

	return nil
}

// Set stores a symbol value in the table where it was found.
func (s *SymbolTable) Set(name string, v interface{}) error {
	var old interface{}

	if s == nil {
		return errors.ErrNoSymbolTable.In("Set")
	}

	if s.shared {
		originalTable := s.Lock()
		defer originalTable.Unlock()
	}

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
		if next := s.FindNextScope(); next != nil {
			return next.Set(name, v)
		} else {
			return errors.ErrUnknownSymbol.Context(name)
		}
	}

	// If we are setting a readonly value, then make sure we are
	// setting a copy of the value, and for complex types, the value
	// is marked as readeonly.
	if strings.HasPrefix(name, defs.ReadonlyVariablePrefix) {
		attr.Readonly = true

		if _, ok := v.(data.Immutable); !ok {
			v = data.DeepCopy(v)
		}

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
			valueString = valueString[:57] + elipses
		}

		quotedName := strconv.Quote(name)
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
	if s == nil {
		return errors.ErrNoSymbolTable.In("Delete")
	}

	if len(name) == 0 {
		return errors.ErrInvalidSymbolName
	}

	if s.shared {
		originalTable := s.Lock()
		defer originalTable.Unlock()
	}

	attr, f := s.symbols[name]
	if !f {
		if s.IsRoot() {
			return errors.ErrUnknownSymbol.Context(name)
		}

		if next := s.FindNextScope(); next != nil {
			return next.Delete(name, always)
		} else {
			return errors.ErrUnknownSymbol.Context(name)
		}
	}

	if !always && attr.Readonly {
		return errors.ErrReadOnlyValue.Context(name)
	}

	delete(s.symbols, name)
	s.modified = true

	if ui.IsActive(ui.SymbolLogger) {
		ui.WriteLog(ui.SymbolLogger, "%s(%s), delete(%s)",
			s.Name, s.id, name)
	}

	return nil
}

// Create creates a symbol name in the table.
func (s *SymbolTable) Create(name string) error {
	if s == nil {
		return errors.ErrNoSymbolTable.In("Create")
	}

	if len(name) == 0 {
		return errors.ErrInvalidSymbolName
	}

	if s.shared {
		originalTable := s.Lock()
		defer originalTable.Unlock()
	}

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
	if s == nil {
		return false
	}

	if s.shared {
		originalTable := s.RLock()
		defer originalTable.RUnlock()
	}

	attr, found := s.symbols[name]
	if found {
		return attr.Readonly
	}

	if !s.IsRoot() {
		next := s.FindNextScope()
		if next != nil {
			return next.IsConstant(name)
		}
	}

	return false
}
