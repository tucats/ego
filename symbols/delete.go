package symbols

import (
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/errors"
)

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
		ui.WriteLog(ui.SymbolLogger, "symbols.delete", ui.A{
			"table": s.Name,
			"id":    s.id,
			"name":  name})
	}

	return nil
}
