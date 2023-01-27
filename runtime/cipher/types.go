package cipher

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/symbols"
)

func Initialize(s *symbols.SymbolTable) {
	newpkg := data.NewPackageFromMap("cipher", map[string]interface{}{
		"Create":   Create,
		"Decrypt":  Decrypt,
		"Encrypt":  Encrypt,
		"Hash":     Hash,
		"Random":   Random,
		"Token":    Token,
		"Validate": Validate,
	}).SetBuiltins(true)

	pkg, _ := bytecode.GetPackage(newpkg.Name())
	pkg.Merge(newpkg)
	s.Root().SetAlways(newpkg.Name(), newpkg)
}
