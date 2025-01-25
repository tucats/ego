package symbols

import (
	"strings"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
)

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

	s.setValue(attr.slot, v)

	if ui.IsActive(ui.SymbolLogger) {
		ui.WriteLog(ui.SymbolLogger, "symbols.set.constant", ui.A{
			"table": s.Name,
			"id":    s.id,
			"name":  name,
			"value": data.Format(v)})
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

			ui.Log(ui.SymbolLogger, "symbols.set.readonly", ui.A{
				"name":  name,
				"table": syms.Name,
				"flag":  flag})

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

	symbolTable.setValue(attr.slot, v)

	if ui.IsActive(ui.SymbolLogger) && name != defs.LineVariable && name != defs.ModuleVariable {
		ui.WriteLog(ui.SymbolLogger, "symbols.set.always", ui.A{
			"table": symbolTable.Name,
			"id":    symbolTable.id,
			"name":  name,
			"slot":  attr.slot,
			"value": v})
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
	symbolTable.setValue(attr.slot, v)

	if ui.IsActive(ui.SymbolLogger) && name != defs.LineVariable && name != defs.ModuleVariable {
		valueString := data.Format(v)
		if len(valueString) > 60 {
			valueString = valueString[:57] + elipses
		}

		ui.WriteLog(ui.SymbolLogger, "symbols.set.attr", ui.A{
			"table":    symbolTable.Name,
			"id":       symbolTable.id,
			"name":     name,
			"slot":     attr.slot,
			"value":    v,
			"readonly": attr.Readonly})
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
		old = s.getValue(attr.slot)
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
	s.setValue(attr.slot, v)

	if attr.Readonly {
		s.symbols[name] = attr
	}

	if ui.IsActive(ui.SymbolLogger) {
		ui.WriteLog(ui.SymbolLogger, "symbols.set.attr", ui.A{
			"table": s.Name,
			"id":    s.id,
			"name":  name,
			"slot":  attr.slot,
			"value": v})
	}

	return nil
}
