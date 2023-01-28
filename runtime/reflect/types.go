package reflect

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/symbols"
)

var MaxDeepCopyDepth int = 100

func Initialize(s *symbols.SymbolTable) {
	newpkg := data.NewPackageFromMap("reflect", map[string]interface{}{
		"DeepCopy": data.Function{
			Declaration: &data.Declaration{
				Name: "DeepCopy",
				Parameters: []data.Parameter{
					{
						Name: "any",
						Type: data.InterfaceType,
					},
					{
						Name: "depth",
						Type: data.IntType,
					},
				},
				ArgCount: data.Range{1, 2},
				Returns:  []*data.Type{data.InterfaceType},
			},
			Value: deepCopy,
		},
		"InstanceOf": data.Function{
			Declaration: &data.Declaration{
				Name: "InstanceOf",
				Parameters: []data.Parameter{
					{
						Name: "any",
						Type: data.InterfaceType,
					},
				},
				Returns: []*data.Type{data.InterfaceType},
			},
			Value: instanceOf,
		},
		"Members": data.Function{
			Declaration: &data.Declaration{
				Name: "Members",
				Parameters: []data.Parameter{
					{
						Name: "any",
						Type: data.InterfaceType,
					},
				},
				Returns: []*data.Type{data.ArrayType(data.StringType)},
			},
			Value: members,
		},
		"Reflect": data.Function{
			Declaration: &data.Declaration{
				Name: "Reflect",
				Parameters: []data.Parameter{
					{
						Name: "any",
						Type: data.InterfaceType,
					},
				},
				Returns: []*data.Type{data.StructType},
			},
			Value: describe,
		},
		"Type": data.Function{
			Declaration: &data.Declaration{
				Name: "Type",
				Parameters: []data.Parameter{
					{
						Name: "any",
						Type: data.InterfaceType,
					},
				},
				Returns: []*data.Type{data.StringType},
			},
			Value: describeType,
		},
	})

	pkg, _ := bytecode.GetPackage(newpkg.Name())
	pkg.Merge(newpkg)
	s.Root().SetAlways(newpkg.Name(), newpkg)
}
