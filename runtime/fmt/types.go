package fmt

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

	if _, found := s.Root().Get("fmt"); !found {
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
				Value: printList,
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
				Value: printFormat,
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
				Value: printLine,
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
				Value: stringPrintFormat,
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
				Value: stringScanFormat,
			},
			"Scan": data.Function{
				Declaration: &data.Declaration{
					Name: "Scan",
					Parameters: []data.Parameter{
						{
							Name: "data",
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
				Value: stringScan,
			},
		})

		pkg, _ := bytecode.GetPackage(newpkg.Name)
		pkg.Merge(newpkg)
		s.Root().SetAlways(newpkg.Name, newpkg)
	}
}
