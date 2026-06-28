package os

import (
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/language/symbols"
)

// args implements os.args() which fetches command-line arguments from
// the Ego command invocation, if any.
func args(s *symbols.SymbolTable, args data.List) (any, error) {
	r, found := s.Get(defs.CLIArgumentListVariable)
	if !found {
		r = data.NewArray(data.StringType, 0)
	}

	return r, nil
}
