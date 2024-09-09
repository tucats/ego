package math

import (
	"sync"

	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/symbols"
)

var initLock sync.Mutex

func Initialize(s *symbols.SymbolTable) {
	initLock.Lock()
	defer initLock.Unlock()

	if _, found := s.Root().Get("math"); !found {
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
				Value: abs,
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
				Value: log,
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
				Value: maximum,
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
				Value: minimum,
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
				Value: normalize,
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
				Value: random,
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
				Value: squareRoot,
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
				Value: sum,
			},
		})

		pkg, _ := bytecode.GetPackage(newpkg.Name)
		pkg.Merge(newpkg)
		s.Root().SetAlways(newpkg.Name, newpkg)
	}
}
