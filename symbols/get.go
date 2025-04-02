package symbols

import (
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
)

// GetAnyScope retrieves a symbol from the current table or any parent
// table that exists. Unlike Get(), this ignores scope boundaries and
// searches all parent scopes.
func (s *SymbolTable) GetAnyScope(name string) (interface{}, bool) {
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

		return s.parent.GetAnyScope(name)
	}

	if ui.IsActive(ui.SymbolLogger) {
		status := notFound
		attr = &SymbolAttribute{}

		if found {
			status = data.Format(v)
			if len(status) > 60 {
				status = status[:57] + ellipses
			}
		}

		ui.WriteLog(ui.SymbolLogger, "symbols.get.any", ui.A{
			"table": s.Name,
			"id":    s.id.String(),
			"name":  name,
			"slot":  attr.slot,
			"value": status})
	}

	return v, found
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
				status = status[:57] + ellipses
			}
		}

		ui.WriteLog(ui.SymbolLogger, "symbols.get", ui.A{
			"table": s.Name,
			"id":    s.id.String(),
			"name":  name,
			"slot":  attr.slot,
			"value": status})
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
				status = status[:57] + ellipses
			}
		}

		ui.WriteLog(ui.SymbolLogger, "symbols.get", ui.A{
			"table": s.Name,
			"id":    s.id.String(),
			"name":  name,
			"slot":  attr.slot,
			"value": status})
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
				status = status[:57] + ellipses
			}
		}

		ui.WriteLog(ui.SymbolLogger, "symbols.get", ui.A{
			"table": s.Name,
			"id":    s.id.String(),
			"name":  name,
			"slot":  attr.slot,
			"value": status})
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
			v = s.addressOfImmutableValue(attr.slot)
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

	ui.Log(ui.SymbolLogger, "symbols.get.addr", ui.A{
		"table": s.Name,
		"id":    s.id.String(),
		"name":  name})

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
