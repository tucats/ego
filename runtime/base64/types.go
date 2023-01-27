package base64

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/symbols"
)

func Initialize(s *symbols.SymbolTable) {
	newpkg := data.NewPackageFromMap("base64", map[string]interface{}{
		"Decode": data.Function{
			Declaration: &data.FunctionDeclaration{
				Name: "Decode",
				Parameters: []data.FunctionParameter{
					{
						Name:     "data",
						ParmType: data.StringType,
					},
				},
				ReturnTypes: []*data.Type{data.StringType},
				ArgCount:    data.Range{1, 1},
			},
			Value: Decode,
		},
		"Encode": data.Function{
			Declaration: &data.FunctionDeclaration{
				Name: "Encode",
				Parameters: []data.FunctionParameter{
					{
						Name:     "data",
						ParmType: data.StringType,
					},
				},
				ReturnTypes: []*data.Type{data.StringType},
				ArgCount:    data.Range{1, 1},
			},
			Value: Encode,
		},
	})

	pkg, _ := bytecode.GetPackage(newpkg.Name())
	pkg.Merge(newpkg)
	s.Root().SetAlways(newpkg.Name(), newpkg)
}
