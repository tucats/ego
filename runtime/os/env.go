package os

import (
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/symbols"
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
