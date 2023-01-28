package profile

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/symbols"
)

func Initialize(s *symbols.SymbolTable) {
	newpkg := data.NewPackageFromMap("profile", map[string]interface{}{
		"Delete": data.Function{
			Declaration: &data.Declaration{
				Name: "Delete",
				Parameters: []data.Parameter{
					{
						Name: "key",
						Type: data.StringType,
					},
				},
			},
			Value: Delete,
		},
		"Get": data.Function{
			Declaration: &data.Declaration{
				Name: "Get",
				Parameters: []data.Parameter{
					{
						Name: "key",
						Type: data.StringType,
					},
				},
				Returns: []*data.Type{data.StringType},
			},
			Value: Get,
		},
		"Keys": data.Function{
			Declaration: &data.Declaration{
				Name:    "Get",
				Returns: []*data.Type{data.ArrayType(data.StringType)},
			},
			Value: Keys,
		},
		"Set": data.Function{
			Declaration: &data.Declaration{
				Name: "Get",
				Parameters: []data.Parameter{
					{
						Name: "key",
						Type: data.StringType,
					},
					{
						Name: "value",
						Type: data.StringType,
					},
				},
				Scope: true,
			},
			Value: Set,
		},
	})

	pkg, _ := bytecode.GetPackage(newpkg.Name())
	pkg.Merge(newpkg)
	s.Root().SetAlways(newpkg.Name(), newpkg)
}
