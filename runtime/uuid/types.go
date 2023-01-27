package uuid

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/compiler"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/symbols"
)

// exec.Cmd type specification.
const uuidTypeSpec = `
	type UUID struct{
		UUID   interface{},
	}`

var uuidTypeDef *data.Type

func Initialize(s *symbols.SymbolTable) {
	t, _ := compiler.CompileTypeSpec(uuidTypeSpec)

	t.DefineFunctions(map[string]data.Function{
		"String": {
			Declaration: &data.FunctionDeclaration{
				Name:         "String",
				ReceiverType: t,
				ReturnTypes:  []*data.Type{data.StringType},
			},
			Value: String,
		},
	})

	uuidTypeDef = t.SetPackage("exec")

	newpkg := data.NewPackageFromMap("uuid", map[string]interface{}{
		"New":              New,
		"Nil":              Nil,
		"Parse":            Parse,
		"UUID":             t,
		data.TypeMDKey:     data.PackageType("exec"),
		data.ReadonlyMDKey: true,
	}).SetBuiltins(true)

	pkg, _ := bytecode.GetPackage(newpkg.Name())
	pkg.Merge(newpkg)
	s.Root().SetAlways(newpkg.Name(), newpkg)
}
