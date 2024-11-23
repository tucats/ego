package symbols

import (
	"strconv"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
)

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
		v = s.getValue(attr.slot)
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
		v = s.getValue(attr.slot)
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
		v = s.getValue(attr.slot)
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
			v = s.addressOfImmuableValue(attr.slot)
		} else {
			v = s.addressOfValue(attr.slot)
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
