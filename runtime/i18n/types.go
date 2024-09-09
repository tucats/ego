package i18n

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

	if _, found := s.Root().Get("i18n"); !found {
		newpkg := data.NewPackageFromMap("i18n", map[string]interface{}{
			"Language": data.Function{
				Declaration: &data.Declaration{
					Name:    "Language",
					Returns: []*data.Type{data.StringType},
				},
				Value: language,
			},
			"T": data.Function{
				Declaration: &data.Declaration{
					Name: "T",
					Parameters: []data.Parameter{
						{
							Name: "key",
							Type: data.StringType,
						},
						{
							Name: "parameters",
							Type: data.MapType(data.StringType, data.InterfaceType),
						},
						{
							Name: "language",
							Type: data.StringType,
						},
					},
					ArgCount: data.Range{1, 3},
					Returns:  []*data.Type{data.StringType},
				},
				Value: translation,
			},
		})

		pkg, _ := bytecode.GetPackage(newpkg.Name)
		pkg.Merge(newpkg)
		s.Root().SetAlways(newpkg.Name, newpkg)
	}
}
