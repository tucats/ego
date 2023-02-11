package os

import (
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/util"
)

func hostname(symbols *symbols.SymbolTable, args data.List) (interface{}, error) {
	return util.Hostname(), nil
}
