package profile

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
			Value: deleteKey,
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
			Value: getKey,
		},
		"Keys": data.Function{
			Declaration: &data.Declaration{
				Name:    "Keys",
				Returns: []*data.Type{data.ArrayType(data.StringType)},
			},
			Value: getKeys,
		},
		"Config": data.Function{
			Declaration: &data.Declaration{
				Name:    "Config",
				Returns: []*data.Type{data.MapType(data.StringType, data.StringType)},
			},
			Value: getConfig,
		},
		"Set": data.Function{
			Declaration: &data.Declaration{
				Name: "Set",
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
			Value: setKey,
		},
	})

	pkg, _ := bytecode.GetPackage(newpkg.Name)
	pkg.Merge(newpkg)
	s.Root().SetAlways(newpkg.Name, newpkg)
}
