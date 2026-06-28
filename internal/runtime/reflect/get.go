package reflect

import (
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/language/symbols"
)

func getThis(s *symbols.SymbolTable) *data.Struct {
	if v, found := s.Get(defs.ThisVariable); found {
		if s, ok := v.(*data.Struct); ok {
			return s
		}
	}

	return nil
}
