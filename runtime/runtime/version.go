package runtime

import (
	"github.com/araddon/dateparse"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/symbols"
)

var VersionString string
var BuildTime string

func egoVersion(s *symbols.SymbolTable, args data.List) (any, error) {
	return "ego" + VersionString, nil
}

func egoBuildTime(s *symbols.SymbolTable, args data.List) (any, error) {
	t, err := dateparse.ParseAny(BuildTime)

	return data.NewList(t, err), err
}
