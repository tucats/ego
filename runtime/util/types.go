package util

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/symbols"
)

func Initialize(s *symbols.SymbolTable) {
	var pkg *data.Package

	newpkg := data.NewPackageFromMap("util", map[string]interface{}{
		"Eval":         Eval,
		"Log":          Log,
		"Memory":       Memory,
		"Mode":         Mode,
		"Packages":     Packages,
		"SetLogger":    SetLogger,
		"Symbols":      Symbols,
		"SymbolTables": Tables,
	})

	pkg, _ = bytecode.GetPackage("util")
	pkg.Merge(newpkg)
	s.Root().SetAlways(newpkg.Name(), newpkg)
}
