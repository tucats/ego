package filepath

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

	if _, found := s.Root().Get("filepath"); !found {
		newpkg := data.NewPackageFromMap("filepath", map[string]interface{}{
			"Abs": data.Function{
				Declaration: &data.Declaration{
					Name: "Abs",
					Parameters: []data.Parameter{
						{
							Name: "partialPath",
							Type: data.StringType,
						},
					},
					Returns: []*data.Type{data.StringType},
				},
				Value: abs,
			},
			"Base": data.Function{
				Declaration: &data.Declaration{
					Name: "Base",
					Parameters: []data.Parameter{
						{
							Name: "path",
							Type: data.StringType,
						},
					},
					Returns: []*data.Type{data.StringType},
				},
				Value: base,
			},
			"Clean": data.Function{
				Declaration: &data.Declaration{
					Name: "Clean",
					Parameters: []data.Parameter{
						{
							Name: "path",
							Type: data.StringType,
						},
					},
					Returns: []*data.Type{data.StringType},
				},
				Value: clean,
			},
			"Dir": data.Function{
				Declaration: &data.Declaration{
					Name: "Dir",
					Parameters: []data.Parameter{
						{
							Name: "path",
							Type: data.StringType,
						},
					},
					Returns: []*data.Type{data.StringType},
				},
				Value: dir,
			},
			"Ext": data.Function{
				Declaration: &data.Declaration{
					Name: "Ext",
					Parameters: []data.Parameter{
						{
							Name: "path",
							Type: data.StringType,
						},
					},
					Returns: []*data.Type{data.StringType},
				},
				Value: ext,
			},
			"Join": data.Function{
				Declaration: &data.Declaration{
					Name: "Join",
					Parameters: []data.Parameter{
						{
							Name: "elements",
							Type: data.StringType,
						},
					},
					Variadic: true,
					Returns:  []*data.Type{data.StringType},
				},
				Value: join,
			},
		})

		pkg, _ := bytecode.GetPackage(newpkg.Name)
		pkg.Merge(newpkg)
		s.Root().SetAlways(newpkg.Name, newpkg)
	}
}
