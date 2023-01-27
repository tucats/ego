package filepath

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/symbols"
)

func Initialize(s *symbols.SymbolTable) {
	newpkg := data.NewPackageFromMap("filepath", map[string]interface{}{
		"Abs": data.Function{
			Declaration: &data.FunctionDeclaration{
				Name: "Abs",
				Parameters: []data.FunctionParameter{
					{
						Name:     "partialPath",
						ParmType: data.StringType,
					},
				},
				ReturnTypes: []*data.Type{data.StringType},
			},
			Value: Abs,
		},
		"Base": data.Function{
			Declaration: &data.FunctionDeclaration{
				Name: "Base",
				Parameters: []data.FunctionParameter{
					{
						Name:     "path",
						ParmType: data.StringType,
					},
				},
				ReturnTypes: []*data.Type{data.StringType},
			},
			Value: Base,
		},
		"Clean": data.Function{
			Declaration: &data.FunctionDeclaration{
				Name: "Clean",
				Parameters: []data.FunctionParameter{
					{
						Name:     "path",
						ParmType: data.StringType,
					},
				},
				ReturnTypes: []*data.Type{data.StringType},
			},
			Value: Clean,
		},
		"Dir": data.Function{
			Declaration: &data.FunctionDeclaration{
				Name: "Dir",
				Parameters: []data.FunctionParameter{
					{
						Name:     "path",
						ParmType: data.StringType,
					},
				},
				ReturnTypes: []*data.Type{data.StringType},
			},
			Value: Dir,
		},
		"Ext": data.Function{
			Declaration: &data.FunctionDeclaration{
				Name: "Ext",
				Parameters: []data.FunctionParameter{
					{
						Name:     "path",
						ParmType: data.StringType,
					},
				},
				ReturnTypes: []*data.Type{data.StringType},
			},
			Value: Ext,
		},
		"Join": data.Function{
			Declaration: &data.FunctionDeclaration{
				Name: "Join",
				Parameters: []data.FunctionParameter{
					{
						Name:     "elements",
						ParmType: data.StringType,
					},
				},
				Variadic:    true,
				ReturnTypes: []*data.Type{data.StringType},
			},
			Value: Join,
		},
	}).SetBuiltins(true)

	pkg, _ := bytecode.GetPackage(newpkg.Name())
	pkg.Merge(newpkg)
	s.Root().SetAlways(newpkg.Name(), newpkg)
}
