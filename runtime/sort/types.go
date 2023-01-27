package sort

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/symbols"
)

func Initialize(s *symbols.SymbolTable) {
	var pkg *data.Package

	newpkg := data.NewPackageFromMap("sort", map[string]interface{}{
		"Bytes":    Bytes,
		"Float32s": Float32s,
		"Float64s": Float64s,
		"Int32s":   Int32s,
		"Int64s":   Int64s,
		"Ints":     Ints,
		"Slice":    Slice,
		"Sort":     Sort,
		"Strings":  Strings,
	}).SetBuiltins(true)

	pkg, _ = bytecode.GetPackage(newpkg.Name())
	pkg.Merge(newpkg)
	s.Root().SetAlways(newpkg.Name(), newpkg)
}
