package reflect

import (
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/symbols"
)

func getThis(s *symbols.SymbolTable) *data.Struct {
	if v, found := s.Get(defs.ThisVariable); found {
		if s, ok := v.(*data.Struct); ok {
			return s
		}
	}

	return nil
}
