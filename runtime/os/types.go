package os

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/symbols"
)

func Initialize(s *symbols.SymbolTable) {
	newpkg := data.NewPackageFromMap("os", map[string]interface{}{
		"Args":             Args,
		"Chmod":            Chmod,
		"Chown":            Chown,
		"Chdir":            Chdir,
		"Writefile":        Writefile,
		"Readfile":         Readfile,
		"Remove":           Remove,
		"Clearenv":         Clearenv,
		"Environ":          Environ,
		"Executable":       Executable,
		"Exit":             Exit,
		"Getenv":           Getenv,
		"Hostname":         Hostname,
		data.TypeMDKey:     data.PackageType("os"),
		data.ReadonlyMDKey: true,
	})

	pkg, _ := bytecode.GetPackage(newpkg.Name())
	pkg.Merge(newpkg)
	s.Root().SetAlways(newpkg.Name(), newpkg)
}
