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
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/errors"
)

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

	s.setValue(s.size, UndefinedValue{})

	s.size++

	if ui.IsActive(ui.SymbolLogger) {
		ui.WriteLog(ui.SymbolLogger, "symbols.create", ui.A{
			"table": s.Name,
			"id":    s.id,
			"name":  name,
			"slot":  s.size - 1})
	}

	return nil
}
