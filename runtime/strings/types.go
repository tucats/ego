package strings

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/symbols"
)

func Initialize(s *symbols.SymbolTable) {
	newpkg := data.NewPackageFromMap("strings", map[string]interface{}{
		"Chars": data.Function{
			Declaration: &data.Declaration{
				Name:     "Chars",
				ArgCount: data.Range{1, 1},
				Parameters: []data.Parameter{
					{
						Name: "text",
						Type: data.StringType,
					},
				},
				Returns: []*data.Type{data.ArrayType(data.StringType)},
			},
			Value: chars,
		},
		"Compare": data.Function{
			Declaration: &data.Declaration{
				Name:     "Compare",
				ArgCount: data.Range{2, 2},
				Parameters: []data.Parameter{
					{
						Name: "a",
						Type: data.StringType,
					},
					{
						Name: "b",
						Type: data.StringType,
					},
				},
				Returns: []*data.Type{data.IntType},
			},
			Value: compare,
		},
		"Contains": data.Function{
			Declaration: &data.Declaration{
				Name:     "Contains",
				ArgCount: data.Range{2, 2},
				Parameters: []data.Parameter{
					{
						Name: "text",
						Type: data.StringType,
					},
					{
						Name: "search",
						Type: data.StringType,
					},
				},
				Returns: []*data.Type{data.BoolType},
			},
			Value: contains,
		},
		"ContainsAny": data.Function{
			Declaration: &data.Declaration{
				Name:     "Contains",
				ArgCount: data.Range{2, 2},
				Parameters: []data.Parameter{
					{
						Name: "text",
						Type: data.StringType,
					},
					{
						Name: "chars",
						Type: data.StringType,
					},
				},
				Returns: []*data.Type{data.BoolType},
			},
			Value: containsAny,
		},
		"EqualFold": data.Function{
			Declaration: &data.Declaration{
				Name:     "EqualFold",
				ArgCount: data.Range{2, 2},
				Parameters: []data.Parameter{
					{
						Name: "a",
						Type: data.StringType,
					},
					{
						Name: "b",
						Type: data.StringType,
					},
				},
				Returns: []*data.Type{data.StringType},
			},
			Value: equalFold,
		},
		"Fields": data.Function{
			Declaration: &data.Declaration{
				Name:     "Fields",
				ArgCount: data.Range{1, 1},
				Parameters: []data.Parameter{
					{
						Name: "text",
						Type: data.StringType,
					},
				},
				Returns: []*data.Type{data.ArrayType(data.StringType)},
			},
			Value: fields,
		},
		"Format": data.Function{
			Declaration: &data.Declaration{
				Name:     "Format",
				ArgCount: data.Range{2, 2},
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
			Value: format,
		},
		"Index": data.Function{
			Declaration: &data.Declaration{
				Name:     "Contains",
				ArgCount: data.Range{2, 2},
				Parameters: []data.Parameter{
					{
						Name: "text",
						Type: data.StringType,
					},
					{
						Name: "substr",
						Type: data.StringType,
					},
				},
				Returns: []*data.Type{data.IntType},
			},
			Value: index,
		},
		"Ints": data.Function{
			Declaration: &data.Declaration{
				Name:     "Ints",
				ArgCount: data.Range{1, 1},
				Parameters: []data.Parameter{
					{
						Name: "text",
						Type: data.StringType,
					},
				},
				Returns: []*data.Type{data.IntType},
			},
			Value: extractInts,
		},
		"Join": data.Function{
			Declaration: &data.Declaration{
				Name:     "Join",
				ArgCount: data.Range{2, 2},
				Parameters: []data.Parameter{
					{
						Name: "text",
						Type: data.ArrayType(data.StringType),
					},
					{
						Name: "separator",
						Type: data.StringType,
					},
				},
				Returns: []*data.Type{data.StringType},
			},
			Value: join,
		},
		"Left": data.Function{
			Declaration: &data.Declaration{
				Name:     "Left",
				ArgCount: data.Range{2, 2},
				Parameters: []data.Parameter{
					{
						Name: "text",
						Type: data.StringType,
					},
					{
						Name: "position",
						Type: data.IntType,
					},
				},
				Returns: []*data.Type{data.StringType},
			},
			Value: leftSubstring,
		},
		"Length": data.Function{
			Declaration: &data.Declaration{
				Name:     "Length",
				ArgCount: data.Range{1, 1},
				Parameters: []data.Parameter{
					{
						Name: "text",
						Type: data.StringType,
					},
				},
				Returns: []*data.Type{data.IntType},
			},
			Value: length,
		},
		"Right": data.Function{
			Declaration: &data.Declaration{
				Name:     "Right",
				ArgCount: data.Range{2, 2},
				Parameters: []data.Parameter{
					{
						Name: "text",
						Type: data.StringType,
					},
					{
						Name: "position",
						Type: data.IntType,
					},
				},
				Returns: []*data.Type{data.StringType},
			},
			Value: rightSubstring,
		},
		"Split": data.Function{
			Declaration: &data.Declaration{
				Name:     "Split",
				ArgCount: data.Range{2, 2},
				Parameters: []data.Parameter{
					{
						Name: "text",
						Type: data.StringType,
					},
					{
						Name: "separator",
						Type: data.StringType,
					},
				},
				Returns: []*data.Type{data.ArrayType(data.StringType)},
			},
			Value: splitString,
		},
		"String": data.Function{
			Declaration: &data.Declaration{
				Name:     "String",
				ArgCount: data.Range{1, 1},
				Parameters: []data.Parameter{
					{
						Name: "any",
						Type: data.InterfaceType,
					},
				},
				Returns: []*data.Type{data.StringType},
			},
			Value: toString,
		},
		"Substring": data.Function{
			Declaration: &data.Declaration{
				Name:     "Substring",
				ArgCount: data.Range{3, 3},
				Parameters: []data.Parameter{
					{
						Name: "text",
						Type: data.StringType,
					},
					{
						Name: "position",
						Type: data.IntType,
					},
					{
						Name: "count",
						Type: data.IntType,
					},
				},
				Returns: []*data.Type{data.StringType},
			},
			Value: substring,
		},
		"Template": data.Function{
			Declaration: &data.Declaration{
				Name: "Template",
				Parameters: []data.Parameter{
					{
						Name: "name",
						Type: data.StringType,
					},
					{
						Name: "parameters",
						Type: data.MapType(data.StringType, data.InterfaceType),
					},
				},
				ArgCount: data.Range{1, 2},
				Scope:    true,
				Returns:  []*data.Type{data.StringType, data.ErrorType},
			},
			Value: evaluateTemplate,
		},
		"ToLower": data.Function{
			Declaration: &data.Declaration{
				Name:     "ToLower",
				ArgCount: data.Range{1, 1},
				Parameters: []data.Parameter{
					{
						Name: "text",
						Type: data.StringType,
					},
				},
				Returns: []*data.Type{data.StringType},
			},
			Value: toLower,
		},
		"ToUpper": data.Function{
			Declaration: &data.Declaration{
				Name:     "ToUpper",
				ArgCount: data.Range{1, 1},
				Parameters: []data.Parameter{
					{
						Name: "text",
						Type: data.StringType,
					},
				},
				Returns: []*data.Type{data.StringType},
			},
			Value: toUpper,
		},
		"Tokenize": data.Function{
			Declaration: &data.Declaration{
				Name:     "Tokenize",
				ArgCount: data.Range{1, 1},
				Parameters: []data.Parameter{
					{
						Name: "text",
						Type: data.StringType,
					},
				},
				Returns: []*data.Type{data.ArrayType(data.StringType)},
			},
			Value: tokenize,
		},
		"TrimPrefix": data.Function{
			Declaration: &data.Declaration{
				Name:     "TrimPrefix",
				ArgCount: data.Range{2, 2},
				Parameters: []data.Parameter{
					{
						Name: "text",
						Type: data.StringType,
					},
					{
						Name: "prefix",
						Type: data.StringType,
					},
				},
				Returns: []*data.Type{data.StringType},
			},
			Value: trimPrefix,
		},
		"TrimSuffix": data.Function{
			Declaration: &data.Declaration{
				Name:     "TrimSuffix",
				ArgCount: data.Range{2, 2},
				Parameters: []data.Parameter{
					{
						Name: "text",
						Type: data.StringType,
					},
					{
						Name: "suffix",
						Type: data.StringType,
					},
				},
				Returns: []*data.Type{data.StringType},
			},
			Value: trimSuffix,
		},
		"TrimSpace": data.Function{
			Declaration: &data.Declaration{
				Name:     "TrimSpace",
				ArgCount: data.Range{1, 1},
				Parameters: []data.Parameter{
					{
						Name: "text",
						Type: data.StringType,
					},
				},
				Returns: []*data.Type{data.StringType},
			},
			Value: trimSpace,
		},
		"Truncate": data.Function{
			Declaration: &data.Declaration{
				Name:     "Truncate",
				ArgCount: data.Range{2, 2},
				Parameters: []data.Parameter{
					{
						Name: "text",
						Type: data.StringType,
					},
					{
						Name: "maxlength",
						Type: data.IntType,
					},
				},
				Returns: []*data.Type{data.StringType},
			},
			Value: truncate,
		},
		"URLPattern": data.Function{
			Declaration: &data.Declaration{
				Name:     "URLPattern",
				ArgCount: data.Range{2, 2},
				Parameters: []data.Parameter{
					{
						Name: "url",
						Type: data.StringType,
					},
					{
						Name: "pattern",
						Type: data.StringType,
					},
				},
				Returns: []*data.Type{data.MapType(data.StringType, data.InterfaceType)},
			},
			Value: URLPattern,
		},
	})

	pkg, _ := bytecode.GetPackage(newpkg.Name)
	pkg.Merge(newpkg)
	s.Root().SetAlways(newpkg.Name, newpkg)
}
