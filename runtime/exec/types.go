package exec

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/compiler"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/symbols"
)

// exec.Cmd type specification.
const commandTypeSpec = `
	type Cmd struct {
		cmd         interface{},
		Dir         string,
		Path		string,
		Args		[]string,
		Env			[]string,
		Stdout      []string,
		Stdin       []string,
	}`

var commandTypeDef *data.Type

func Initialize(s *symbols.SymbolTable) {
	t, _ := compiler.CompileTypeSpec(commandTypeSpec)

	t.DefineFunctions(map[string]data.Function{
		"Output": {Value: Output},
		"Run":    {Value: Run},
	})

	commandTypeDef = t.SetPackage("exec")

	newpkg := data.NewPackageFromMap("exec", map[string]interface{}{
		"Command":          Command,
		"LookPath":         LookPath,
		"Cmd":              t,
		data.TypeMDKey:     data.PackageType("exec"),
		data.ReadonlyMDKey: true,
	}).SetBuiltins(true)

	pkg, _ := bytecode.GetPackage(newpkg.Name())
	pkg.Merge(newpkg)
	s.Root().SetAlways(newpkg.Name(), newpkg)
}
