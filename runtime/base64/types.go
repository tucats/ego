package base64

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

	if _, found := s.Root().Get("base64"); !found {
		newpkg := data.NewPackageFromMap("base64", map[string]interface{}{
			"Decode": data.Function{
				Declaration: &data.Declaration{
					Name: "Decode",
					Parameters: []data.Parameter{
						{
							Name: "data",
							Type: data.StringType,
						},
					},
					Returns:  []*data.Type{data.StringType},
					ArgCount: data.Range{1, 1},
				},
				Value: decode,
			},
			"Encode": data.Function{
				Declaration: &data.Declaration{
					Name: "Encode",
					Parameters: []data.Parameter{
						{
							Name: "data",
							Type: data.StringType,
						},
					},
					Returns:  []*data.Type{data.StringType},
					ArgCount: data.Range{1, 1},
				},
				Value: encode,
			},
		})

		pkg, _ := bytecode.GetPackage(newpkg.Name)
		pkg.Merge(newpkg)
		s.Root().SetAlways(newpkg.Name, newpkg)
	}
}
