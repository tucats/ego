package json

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/symbols"
)

func Initialize(s *symbols.SymbolTable) {
	newpkg := data.NewPackageFromMap("json", map[string]interface{}{
		"Marshal": data.Function{
			Declaration: &data.Declaration{
				Name: "Marshal",
				Parameters: []data.Parameter{
					{
						Name: "any",
						Type: data.InterfaceType,
					},
				},
				Returns: []*data.Type{data.ArrayType(data.ByteType)},
			},
			Value: Marshal,
		},
		"MarshalIndent": data.Function{
			Declaration: &data.Declaration{
				Name: "MarshalIndent",
				Parameters: []data.Parameter{
					{
						Name: "any",
						Type: data.InterfaceType,
					},
					{
						Name: "prefix",
						Type: data.StringType,
					},
					{
						Name: "indent",
						Type: data.StringType,
					},
				},
				Returns: []*data.Type{data.ArrayType(data.ByteType)},
			},
			Value: MarshalIndent,
		},
		"Unmarshal": data.Function{
			Declaration: &data.Declaration{
				Name: "Unmarshal",
				Parameters: []data.Parameter{
					{
						Name: "data",
						Type: data.ArrayType(data.ByteType),
					},
					{
						Name: "value",
						Type: data.PointerType(data.InterfaceType),
					},
				},
				Returns: []*data.Type{data.ErrorType},
			},
			Value: Unmarshal,
		},
	})

	pkg, _ := bytecode.GetPackage(newpkg.Name())
	pkg.Merge(newpkg)
	s.Root().SetAlways(newpkg.Name(), newpkg)
}
