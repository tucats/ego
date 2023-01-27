package filepath

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/symbols"
)

func Initialize(s *symbols.SymbolTable) {
	newpkg := data.NewPackageFromMap("filepath", map[string]interface{}{
		"Abs":   Abs,
		"Base":  Base,
		"Clean": Clean,
		"Dir":   Dir,
		"Ext":   Ext,
		"Join":  Join,
	}).SetBuiltins(true)

	pkg, _ := bytecode.GetPackage(newpkg.Name())
	pkg.Merge(newpkg)
	s.Root().SetAlways(newpkg.Name(), newpkg)
}
