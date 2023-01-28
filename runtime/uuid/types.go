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
			Declaration: &data.Declaration{
				Name:    "String",
				Type:    t,
				Returns: []*data.Type{data.StringType},
			},
			Value: toString,
		},
	})

	uuidTypeDef = t.SetPackage("uuid")

	newpkg := data.NewPackageFromMap("uuid", map[string]interface{}{
		"New": data.Function{
			Declaration: &data.Declaration{
				Name:    "New",
				Returns: []*data.Type{t},
			},
			Value: newUUID,
		},
		"Nil": data.Function{
			Declaration: &data.Declaration{
				Name:    "Nil",
				Returns: []*data.Type{t},
			},
			Value: nilUUID,
		},
		"Parse": data.Function{
			Declaration: &data.Declaration{
				Name: "Parse",
				Parameters: []data.Parameter{
					{
						Name: "text",
						Type: data.StringType,
					},
				},
				Returns: []*data.Type{t, data.ErrorType},
			},
			Value: parseUUID,
		},
		"UUID": t,
	}).SetBuiltins(true)

	pkg, _ := bytecode.GetPackage(newpkg.Name())
	pkg.Merge(newpkg)
	s.Root().SetAlways(newpkg.Name(), newpkg)
}
