package symbols

import (
	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/language/data"
)

// GetAnyScope retrieves a symbol from the current table or any parent
// table that exists. Unlike Get(), this ignores scope boundaries and
// searches all parent scopes.
func (s *SymbolTable) GetAnyScope(name string) (any, bool) {
	var v any

	if s == nil {
		return nil, false
	}

	if s.shared.Load() {
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
			"id":    s.id,
			"name":  name,
			"slot":  attr.slot,
			"value": status})
	}

	return v, found
}

// Get retrieves a symbol from the current table or any parent
// table that exists.
func (s *SymbolTable) Get(name string) (any, bool) {
	var v any

	if s == nil {
		return nil, false
	}

	if s.shared.Load() {
		symbolTable := s.RLock()
		defer symbolTable.RUnlock()
	}

	attr, found := s.symbols[name]
	if found {
		v = s.getValue(attr.slot)
	}

	// docs/SLOTS.md introspection: a slotted local is not in the symbols map;
	// resolve it from this table's own bank so it is visible by name (debugger,
	// error formatting, util.Symbols). This is checked before the parent walk so
	// a local correctly shadows a same-named outer variable.
	if !found && s.localNames != nil {
		if sv, ok := s.slotValueByName(name); ok {
			v, found = sv, true
		}
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
			"id":    s.id,
			"name":  name,
			"slot":  attr.slot,
			"value": status})
	}

	return v, found
}

// GetLocal retrieves a symbol only from the current table, without searching
// parent scopes. Returns the value and true if found, or nil and false otherwise.
func (s *SymbolTable) GetLocal(name string) (any, bool) {
	var v any

	if s == nil {
		return nil, false
	}

	if s.shared.Load() {
		symbolTable := s.RLock()
		defer symbolTable.RUnlock()
	}

	attr, found := s.symbols[name]
	if found {
		v = s.getValue(attr.slot)
	}

	// docs/SLOTS.md introspection: resolve a slotted local by name from this
	// table's own bank (GetLocal never walks parents, so this is the only place
	// a slotted local can surface for it).
	if !found && s.localNames != nil {
		if sv, ok := s.slotValueByName(name); ok {
			v, found = sv, true
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
			"id":    s.id,
			"name":  name,
			"slot":  attr.slot,
			"value": status})
	}

	return v, found
}

// Get retrieves a symbol from the current table or any parent
// table that exists.
func (s *SymbolTable) GetWithAttributes(name string) (any, *SymbolAttribute, bool) {
	var v any

	if s == nil {
		return nil, nil, false
	}

	if s.shared.Load() {
		symbolTable := s.RLock()
		defer symbolTable.RUnlock()
	}

	attr, found := s.symbols[name]
	if found {
		v = s.getValue(attr.slot)
	}

	// docs/SLOTS.md introspection: surface a slotted local by name, with a
	// synthetic attribute carrying its slot index (slotted locals are never
	// readonly or ephemeral in this cut).
	if !found && s.localNames != nil {
		if idx, ok := s.slotIndexByName(name); ok {
			v = s.locals[idx]
			attr = &SymbolAttribute{slot: idx}
			found = true
		}
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
			"id":    s.id,
			"name":  name,
			"slot":  attr.slot,
			"value": status})
	}

	return v, attr, found
}

// GetAddress retrieves the address of a symbol values from the
// current table or any parent table that exists.
func (s *SymbolTable) GetAddress(name string) (any, bool) {
	var v any

	if s == nil {
		return nil, false
	}

	if s.shared.Load() {
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

	if ui.IsActive(ui.SymbolLogger) {
		ui.Log(ui.SymbolLogger, "symbols.get.addr", ui.A{
			"table": s.Name,
			"id":    s.id,
			"name":  name})
	}

	return v, found
}

// FindTable walks the same scope chain Get()/Set()/GetAddress() use to find
// which table currently owns name, without extracting or mutating any value.
// Used by the Finding 17 Tier 2 runtime cache (see docs/internals/GLOBALS.md,
// internal/language/bytecode/globalcache.go): on a cache miss, the opcode
// handler calls this once, after the ordinary Get/Set/GetAddress call, to
// learn which table the name resolved to, so a future execution of the same
// instruction can be cached to skip straight to it if that table is a global
// singleton. This intentionally duplicates Get()'s walk rather than changing
// Get() itself to also return a table -- Get()/Set()/GetAddress() are
// unmodified, so this addition carries zero risk to their existing,
// well-tested behavior.
func (s *SymbolTable) FindTable(name string) (*SymbolTable, bool) {
	if s == nil {
		return nil, false
	}

	if s.shared.Load() {
		symbolTable := s.RLock()
		defer symbolTable.RUnlock()
	}

	if _, found := s.symbols[name]; found {
		return s, true
	}

	// docs/SLOTS.md introspection: a slotted local is not in the symbols map,
	// but is still "found here" for FindTable's purposes -- mirrors Get()'s
	// own localNames check.
	if s.localNames != nil {
		if _, ok := s.slotIndexByName(name); ok {
			return s, true
		}
	}

	if !s.IsRoot() {
		if next := s.FindNextScope(); next != nil {
			return next.FindTable(name)
		}
	}

	return nil, false
}

// IsConstant determines if a name is a constant or readonly value.
func (s *SymbolTable) IsConstant(name string) bool {
	if s == nil {
		return false
	}

	if s.shared.Load() {
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
