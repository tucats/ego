package strconv

import (
	"strconv"

	"github.com/tucats/ego/data"
)

var StrconvPackage = data.NewPackageFromMap("strconv", map[string]interface{}{
	"Itor": data.Function{
		Declaration: &data.Declaration{
			Name: "Itor",
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
		Value:    strconv.Atoi,
		IsNative: true,
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
		Value:    strconv.FormatBool,
		IsNative: true,
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
		Value:    strconv.FormatFloat,
		IsNative: true,
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
		Value:    strconv.FormatInt,
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
