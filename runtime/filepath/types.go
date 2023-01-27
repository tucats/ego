package filepath

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/symbols"
)

func Initialize(s *symbols.SymbolTable) {
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
			Value: Abs,
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
			Value: Base,
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
			Value: Clean,
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
			Value: Dir,
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
			Value: Ext,
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
			Value: Join,
		},
	}).SetBuiltins(true)

	pkg, _ := bytecode.GetPackage(newpkg.Name())
	pkg.Merge(newpkg)
	s.Root().SetAlways(newpkg.Name(), newpkg)
}
