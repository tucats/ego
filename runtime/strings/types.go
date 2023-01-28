package strings

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/symbols"
)

func Initialize(s *symbols.SymbolTable) {
	newpkg := data.NewPackageFromMap("strings", map[string]interface{}{
		"Blockfonts": data.Function{
			Declaration: &data.Declaration{
				Name:    "Blockfonts",
				Returns: []*data.Type{data.ArrayType(data.StringType)},
			},
			Value: BlockFonts,
		},
		"Blockprint": data.Function{
			Declaration: &data.Declaration{
				Name: "Blockprint",
				Parameters: []data.Parameter{
					{
						Name: "text",
						Type: data.StringType,
					},
					{
						Name: "fontName",
						Type: data.StringType,
					},
				},
				ArgCount: data.Range{1, 2},
				Returns:  []*data.Type{data.ArrayType(data.StringType)},
			},
			Value: BlockPrint,
		},
		"Chars": data.Function{
			Declaration: &data.Declaration{
				Name: "Chars",
				Parameters: []data.Parameter{
					{
						Name: "text",
						Type: data.StringType,
					},
				},
				Returns: []*data.Type{data.ArrayType(data.StringType)},
			},
			Value: Chars,
		},
		"Compare": data.Function{
			Declaration: &data.Declaration{
				Name: "Compare",
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
			Value: Compare,
		},
		"Contains": data.Function{
			Declaration: &data.Declaration{
				Name: "Contains",
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
			Value: Contains,
		},
		"ContainsAny": data.Function{
			Declaration: &data.Declaration{
				Name: "Contains",
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
			Value: ContainsAny,
		},
		"EqualFold": data.Function{
			Declaration: &data.Declaration{
				Name: "EqualFold",
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
			Value: EqualFold,
		},
		"Fields": data.Function{
			Declaration: &data.Declaration{
				Name: "Fields",
				Parameters: []data.Parameter{
					{
						Name: "text",
						Type: data.StringType,
					},
				},
				Returns: []*data.Type{data.ArrayType(data.StringType)},
			},
			Value: Fields,
		},
		"Format": data.Function{
			Declaration: &data.Declaration{
				Name: "Format",
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
				ArgCount: data.Range{1, 2},
				Variadic: true,
				Returns:  []*data.Type{data.StringType},
			},
			Value: Format,
		},
		"Index": data.Function{
			Declaration: &data.Declaration{
				Name: "Contains",
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
			Value: Index,
		},
		"Ints": data.Function{
			Declaration: &data.Declaration{
				Name: "Ints",
				Parameters: []data.Parameter{
					{
						Name: "text",
						Type: data.StringType,
					},
				},
				Returns: []*data.Type{data.IntType},
			},
			Value: Ints,
		},
		"Join": data.Function{
			Declaration: &data.Declaration{
				Name: "Join",
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
			Value: Join,
		},
		"Left": data.Function{
			Declaration: &data.Declaration{
				Name: "Left",
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
			Value: Left,
		},
		"Length": data.Function{
			Declaration: &data.Declaration{
				Name: "Length",
				Parameters: []data.Parameter{
					{
						Name: "text",
						Type: data.StringType,
					},
				},
				Returns: []*data.Type{data.IntType},
			},
			Value: Length,
		},
		"Right": data.Function{
			Declaration: &data.Declaration{
				Name: "Right",
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
			Value: Right,
		},
		"Split": data.Function{
			Declaration: &data.Declaration{
				Name: "Split",
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
			Value: Split,
		},
		"String": data.Function{
			Declaration: &data.Declaration{
				Name: "String",
				Parameters: []data.Parameter{
					{
						Name: "any",
						Type: data.InterfaceType,
					},
				},
				Returns: []*data.Type{data.StringType},
			},
			Value: String,
		},
		"Substring": data.Function{
			Declaration: &data.Declaration{
				Name: "Substring",
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
			Value: Substring,
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
			Value: Template,
		},
		"ToLower": data.Function{
			Declaration: &data.Declaration{
				Name: "ToLower",
				Parameters: []data.Parameter{
					{
						Name: "text",
						Type: data.StringType,
					},
				},
				Returns: []*data.Type{data.StringType},
			},
			Value: ToLower,
		},
		"ToUpper": data.Function{
			Declaration: &data.Declaration{
				Name: "ToUpper",
				Parameters: []data.Parameter{
					{
						Name: "text",
						Type: data.StringType,
					},
				},
				Returns: []*data.Type{data.StringType},
			},
			Value: ToUpper,
		},
		"Tokenize": data.Function{
			Declaration: &data.Declaration{
				Name: "Tokenize",
				Parameters: []data.Parameter{
					{
						Name: "text",
						Type: data.StringType,
					},
				},
				Returns: []*data.Type{data.ArrayType(data.StringType)},
			},
			Value: Tokenize,
		},
		"Truncate": data.Function{
			Declaration: &data.Declaration{
				Name: "Truncate",
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
			Value: Truncate,
		},
		"URLPattern": data.Function{
			Declaration: &data.Declaration{
				Name: "URLPattern",
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

	pkg, _ := bytecode.GetPackage(newpkg.Name())
	pkg.Merge(newpkg)
	s.Root().SetAlways(newpkg.Name(), newpkg)
}
