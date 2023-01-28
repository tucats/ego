package fmt

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/symbols"
)

func Initialize(s *symbols.SymbolTable) {
	newpkg := data.NewPackageFromMap("fmt", map[string]interface{}{
		"Print": data.Function{
			Declaration: &data.Declaration{
				Name: "Print",
				Parameters: []data.Parameter{
					{
						Name: "item",
						Type: data.InterfaceType,
					},
				},
				Variadic: true,
				Returns:  []*data.Type{data.IntType},
			},
			Value: Print,
		},
		"Printf": data.Function{
			Declaration: &data.Declaration{
				Name: "Printf",
				Parameters: []data.Parameter{
					{
						Name: "format",
						Type: data.StringType,
					},
					{
						Name: "item",
						Type: data.InterfaceType,
					},
				},
				Variadic: true,
				Returns:  []*data.Type{data.IntType},
			},
			Value: Printf,
		},
		"Println": data.Function{
			Declaration: &data.Declaration{
				Name: "Println",
				Parameters: []data.Parameter{
					{
						Name: "item",
						Type: data.InterfaceType,
					},
				},
				Variadic: true,
				Returns:  []*data.Type{data.IntType},
			},
			Value: Println,
		},
		"Sprintf": data.Function{
			Declaration: &data.Declaration{
				Name: "Sprintf",
				Parameters: []data.Parameter{
					{
						Name: "format",
						Type: data.StringType,
					},
					{
						Name: "item",
						Type: data.InterfaceType,
					},
				},
				Variadic: true,
				Returns:  []*data.Type{data.StringType},
			},
			Value: Sprintf,
		},
		"Sscanf": data.Function{
			Declaration: &data.Declaration{
				Name: "Sscanf",
				Parameters: []data.Parameter{
					{
						Name: "data",
						Type: data.StringType,
					},
					{
						Name: "format",
						Type: data.StringType,
					},
					{
						Name: "item",
						Type: data.PointerType(data.InterfaceType),
					},
				},
				Variadic: true,
				Returns:  []*data.Type{data.IntType, data.ErrorType},
			},
			Value: Sscanf,
		},
	})

	pkg, _ := bytecode.GetPackage(newpkg.Name())
	pkg.Merge(newpkg)
	s.Root().SetAlways(newpkg.Name(), newpkg)
}
