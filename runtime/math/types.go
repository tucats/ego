package math

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/symbols"
)

func Initialize(s *symbols.SymbolTable) {
	newpkg := data.NewPackageFromMap("math", map[string]interface{}{
		"Abs": data.Function{
			Declaration: &data.Declaration{
				Name: "Abs",
				Parameters: []data.Parameter{
					{
						Name: "any",
						Type: data.InterfaceType,
					},
				},
				Returns: []*data.Type{data.InterfaceType},
			},
			Value: Abs,
		},
		"Log": data.Function{
			Declaration: &data.Declaration{
				Name: "Log",
				Parameters: []data.Parameter{
					{
						Name: "f",
						Type: data.Float64Type,
					},
				},
				Returns: []*data.Type{data.Float64Type},
			},
			Value: Log,
		},
		"Max": data.Function{
			Declaration: &data.Declaration{
				Name: "Max",
				Parameters: []data.Parameter{
					{
						Name: "any",
						Type: data.InterfaceType,
					},
				},
				Variadic: true,
				Returns:  []*data.Type{data.InterfaceType},
			},
			Value: Max,
		},
		"Min": data.Function{
			Declaration: &data.Declaration{
				Name: "Min",
				Parameters: []data.Parameter{
					{
						Name: "any",
						Type: data.InterfaceType,
					},
				},
				Variadic: true,
				Returns:  []*data.Type{data.InterfaceType},
			},
			Value: Min,
		},
		"Normalize": data.Function{
			Declaration: &data.Declaration{
				Name: "Max",
				Parameters: []data.Parameter{
					{
						Name: "a",
						Type: data.InterfaceType,
					},
					{
						Name: "b",
						Type: data.InterfaceType,
					},
				},
				Variadic: true,
				Returns:  []*data.Type{data.InterfaceType, data.InterfaceType},
			},
			Value: Normalize,
		},
		"Random": data.Function{
			Declaration: &data.Declaration{
				Name: "Random",
				Parameters: []data.Parameter{
					{
						Name: "maxvalue",
						Type: data.IntType,
					},
				},
				Returns: []*data.Type{data.IntType},
			},
			Value: Random,
		},
		"Sqrt": data.Function{
			Declaration: &data.Declaration{
				Name: "Sqrt",
				Parameters: []data.Parameter{
					{
						Name: "f",
						Type: data.Float64Type,
					},
				},
				Returns: []*data.Type{data.Float64Type},
			},
			Value: Sqrt,
		},
		"Sum": data.Function{
			Declaration: &data.Declaration{
				Name: "Sum",
				Parameters: []data.Parameter{
					{
						Name: "any",
						Type: data.InterfaceType,
					},
				},
				Variadic: true,
				Returns:  []*data.Type{data.InterfaceType},
			},
			Value: Sum,
		},
	})

	pkg, _ := bytecode.GetPackage(newpkg.Name())
	pkg.Merge(newpkg)
	s.Root().SetAlways(newpkg.Name(), newpkg)
}
