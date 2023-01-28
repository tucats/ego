package os

import (
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/util"
)

func hostname(symbols *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	return util.Hostname(), nil
}
