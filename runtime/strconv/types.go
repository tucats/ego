package strconv

import (
	"strconv"

	"github.com/tucats/ego/data"
)

var StrconvPackage = data.NewPackageFromMap("strconv", map[string]any{
	"Itor": data.Function{
		Declaration: &data.Declaration{
			Name: "Itor",
			Parameters: []data.Parameter{
				{
					Name: "i",
					Type: data.IntType,
				},
			},
			Returns: []*data.Type{data.StringType, data.ErrorType},
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
			Returns: []*data.Type{data.IntType, data.ErrorType},
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
		Value:    strconv.Atoi,
		IsNative: true,
	},
	"CanBackquote": data.Function{
		Declaration: &data.Declaration{
			Name: "CanBackquote",
			Parameters: []data.Parameter{
				{
					Name: "s",
					Type: data.StringType,
				},
			},
			Returns: []*data.Type{data.BoolType},
		},
		Value:    strconv.CanBackquote,
		IsNative: true,
	},
	"FormatBool": data.Function{
		Declaration: &data.Declaration{
			Name: "FormatBool",
			Parameters: []data.Parameter{
				{
					Name: "b",
					Type: data.BoolType,
				},
			},
			Returns: []*data.Type{data.StringType},
		},
		Value:    strconv.FormatBool,
		IsNative: true,
	},
	"FormatFloat": data.Function{
		Declaration: &data.Declaration{
			Name: "FormatFloat",
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
		Value:    strconv.FormatFloat,
		IsNative: true,
	}, "FormatInt": data.Function{
		Declaration: &data.Declaration{
			Name: "FormatInt",
			Parameters: []data.Parameter{
				{
					Name: "i",
					Type: data.Int64Type,
				},
				{
					Name: "base",
					Type: data.IntType,
				},
			},
			Returns: []*data.Type{data.StringType},
		},
		Value:    strconv.FormatInt,
		IsNative: true,
	},
	"FormatUint": data.Function{
		Declaration: &data.Declaration{
			Name: "FormatUint",
			Parameters: []data.Parameter{
				{
					Name: "i",
					Type: data.UInt64Type,
				},
				{
					Name: "base",
					Type: data.IntType,
				},
			},
			Returns: []*data.Type{data.StringType},
		},
		Value:    strconv.FormatUint,
		IsNative: true,
	},
	"IsGraphic": data.Function{
		Declaration: &data.Declaration{
			Name: "IsGraphic",
			Parameters: []data.Parameter{
				{
					Name: "r",
					Type: data.Int32Type,
				},
			},
			Returns: []*data.Type{data.BoolType},
		},
		Value:    strconv.IsGraphic,
		IsNative: true,
	},
	"IsPrint": data.Function{
		Declaration: &data.Declaration{
			Name: "IsPrint",
			Parameters: []data.Parameter{
				{
					Name: "r",
					Type: data.Int32Type,
				},
			},
			Returns: []*data.Type{data.BoolType},
		},
		Value:    strconv.IsPrint,
		IsNative: true,
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
		Value:    strconv.Itoa,
		IsNative: true,
	},
	"ParseBool": data.Function{
		Declaration: &data.Declaration{
			Name: "ParseBool",
			Parameters: []data.Parameter{
				{
					Name: "s",
					Type: data.StringType,
				},
			},
			Returns: []*data.Type{data.BoolType, data.ErrorType},
		},
		Value:    strconv.ParseBool,
		IsNative: true,
	},
	"ParseFloat": data.Function{
		Declaration: &data.Declaration{
			Name: "ParseFloat",
			Parameters: []data.Parameter{
				{
					Name: "s",
					Type: data.StringType,
				},
				{
					Name: "bitSize",
					Type: data.IntType,
				},
			},
			Returns: []*data.Type{data.Float64Type, data.ErrorType},
		},
		Value:    strconv.ParseFloat,
		IsNative: true,
	},
	"ParseInt": data.Function{
		Declaration: &data.Declaration{
			Name: "ParseInt",
			Parameters: []data.Parameter{
				{
					Name: "s",
					Type: data.StringType,
				},
				{
					Name: "base",
					Type: data.IntType,
				},
				{
					Name: "bitSize",
					Type: data.IntType,
				},
			},
			Returns: []*data.Type{data.Int64Type, data.ErrorType},
		},
		Value:    strconv.ParseInt,
		IsNative: true,
	},
	"ParseUint": data.Function{
		Declaration: &data.Declaration{
			Name: "ParseUint",
			Parameters: []data.Parameter{
				{
					Name: "s",
					Type: data.StringType,
				},
				{
					Name: "base",
					Type: data.IntType,
				},
				{
					Name: "bitSize",
					Type: data.IntType,
				},
			},
			Returns: []*data.Type{data.UInt64Type, data.ErrorType},
		},
		Value:    strconv.ParseUint,
		IsNative: true,
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
		Value:    strconv.Quote,
		IsNative: true,
	},
	"QuoteRune": data.Function{
		Declaration: &data.Declaration{
			Name: "QuoteRune",
			Parameters: []data.Parameter{
				{
					Name: "r",
					Type: data.Int32Type,
				},
			},
			Returns: []*data.Type{data.StringType},
		},
		Value:    strconv.QuoteRune,
		IsNative: true,
	},
	"QuoteRuneToASCII": data.Function{
		Declaration: &data.Declaration{
			Name: "QuoteRuneToASCII",
			Parameters: []data.Parameter{
				{
					Name: "r",
					Type: data.Int32Type,
				},
			},
			Returns: []*data.Type{data.StringType},
		},
		Value:    strconv.QuoteRuneToASCII,
		IsNative: true,
	},
	"QuoteRuneToGraphic": data.Function{
		Declaration: &data.Declaration{
			Name: "QuoteRuneToGraphic",
			Parameters: []data.Parameter{
				{
					Name: "r",
					Type: data.Int32Type,
				},
			},
			Returns: []*data.Type{data.StringType},
		},
		Value:    strconv.QuoteRuneToGraphic,
		IsNative: true,
	},
	"QuoteToASCII": data.Function{
		Declaration: &data.Declaration{
			Name: "QuoteToASCII",
			Parameters: []data.Parameter{
				{
					Name: "s",
					Type: data.StringType,
				},
			},
			Returns: []*data.Type{data.StringType},
		},
		Value:    strconv.QuoteToASCII,
		IsNative: true,
	},
	"QuoteToGraphic": data.Function{
		Declaration: &data.Declaration{
			Name: "QuoteToGraphic",
			Parameters: []data.Parameter{
				{
					Name: "s",
					Type: data.StringType,
				},
			},
			Returns: []*data.Type{data.StringType},
		},
		Value:    strconv.QuoteToGraphic,
		IsNative: true,
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
		Value:    strconv.Unquote,
		IsNative: true,
	},
})
