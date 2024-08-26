package strconv

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/symbols"
)

func Initialize(s *symbols.SymbolTable) {
	newpkg := data.NewPackageFromMap("strconv", map[string]interface{}{
		"Itor": data.Function{
			Declaration: &data.Declaration{
				Name: "Ator",
				Parameters: []data.Parameter{
					{
						Name: "i",
						Type: data.IntType,
					},
				},
				Returns: []*data.Type{data.StringType},
			},
			Value: doIntToRoman,
		},
		"Rtoi": data.Function{
			Declaration: &data.Declaration{
				Name: "Rtoi",
				Parameters: []data.Parameter{
					{
						Name: "r",
						Type: data.StringType,
					},
				},
				Returns: []*data.Type{data.IntType},
			},
			Value: doRomanToInt,
		},
		"Atoi": data.Function{
			Declaration: &data.Declaration{
				Name: "Atoi",
				Parameters: []data.Parameter{
					{
						Name: "text",
						Type: data.StringType,
					},
				},
				Returns: []*data.Type{data.IntType, data.ErrorType},
			},
			Value: doAtoi,
		},
		"Formatbool": data.Function{
			Declaration: &data.Declaration{
				Name: "Formatbool",
				Parameters: []data.Parameter{
					{
						Name: "b",
						Type: data.BoolType,
					},
				},
				Returns: []*data.Type{data.StringType},
			},
			Value: doFormatbool,
		},
		"Formatfloat": data.Function{
			Declaration: &data.Declaration{
				Name: "Formatfloat",
				Parameters: []data.Parameter{
					{
						Name: "f",
						Type: data.Float64Type,
					},
					{
						Name: "format",
						Type: data.ByteType,
					},
					{
						Name: "precision",
						Type: data.IntType,
					},
					{
						Name: "bitsize",
						Type: data.IntType,
					},
				},
				Returns: []*data.Type{data.StringType},
			},
			Value: doFormatfloat,
		}, "Formatint": data.Function{
			Declaration: &data.Declaration{
				Name: "Formatint",
				Parameters: []data.Parameter{
					{
						Name: "i",
						Type: data.IntType,
					},
					{
						Name: "base",
						Type: data.IntType,
					},
				},
				Returns: []*data.Type{data.StringType},
			},
			Value: doFormatint,
		},
		"Itoa": data.Function{
			Declaration: &data.Declaration{
				Name: "Itoa",
				Parameters: []data.Parameter{
					{
						Name: "i",
						Type: data.IntType,
					},
				},
				Returns: []*data.Type{data.StringType},
			},
			Value: doItoa,
		},
		"Quote": data.Function{
			Declaration: &data.Declaration{
				Name: "Quote",
				Parameters: []data.Parameter{
					{
						Name: "text",
						Type: data.StringType,
					},
				},
				Returns: []*data.Type{data.StringType},
			},
			Value: doQuote,
		},
		"Unquote": data.Function{
			Declaration: &data.Declaration{
				Name: "Unquote",
				Parameters: []data.Parameter{
					{
						Name: "text",
						Type: data.StringType,
					},
				},
				Returns: []*data.Type{data.StringType, data.ErrorType},
			},
			Value: doUnquote,
		},
	})

	pkg, _ := bytecode.GetPackage(newpkg.Name)
	pkg.Merge(newpkg)
	s.Root().SetAlways(newpkg.Name, newpkg)
}
