package strings

import (
	"fmt"
	"strings"

	"github.com/tucats/ego/data"
)

var StringsReaderType = data.TypeDefinition("Reader", data.StructureType()).
	SetNativeName("*strings.Reader").
	SetPackage("strings").
	DefineNativeFunction("Read", &data.Declaration{
		Name: "Read",
		Type: data.OwnType,
		Parameters: []data.Parameter{
			{
				Name: "buff",
				Type: data.ArrayType(data.ByteType),
			},
		},
		Returns: []*data.Type{data.IntType, data.ErrorType},
	}, nil)

var StringsTokenArrayType = data.TypeOf(data.NewStructFromMap(map[string]interface{}{
	"kind":     "",
	"spelling": "",
}))

var StringsBuilderType = data.TypeDefinition("Builder",
	data.StructureType()).
	SetNativeName("strings.Builder").
	SetFormatFunc(func(v interface{}) string {
		b := v.(*strings.Builder)

		return fmt.Sprintf(`strings.Builder{String: "%s", Len: %d, Cap: %d}`, b.String(), b.Len(), b.Cap())
	}).
	SetPackage("strings").
	SetNew(func() interface{} {
		return &strings.Builder{}
	}).
	DefineNativeFunction("Cap", &data.Declaration{
		Name: "Cap",
		Type: data.OwnType,
		Parameters: []data.Parameter{
			{
				Name: "s",
				Type: data.StringType,
			},
		},
		Returns: []*data.Type{data.IntType},
	}, nil).
	DefineNativeFunction("Grow", &data.Declaration{
		Name: "Grow",
		Type: data.OwnType,
		Parameters: []data.Parameter{
			{
				Name: "n",
				Type: data.IntType,
			},
		},
		Returns: []*data.Type{data.IntType},
	}, nil).
	DefineNativeFunction("Len", &data.Declaration{
		Name:    "Len",
		Type:    data.OwnType,
		Returns: []*data.Type{data.IntType},
	}, nil).
	DefineNativeFunction("Reset", &data.Declaration{
		Name: "Reset",
		Type: data.OwnType,
	}, nil).
	DefineNativeFunction("String", &data.Declaration{
		Name:    "String",
		Type:    data.OwnType,
		Returns: []*data.Type{data.StringType},
	}, nil).
	DefineNativeFunction("Write", &data.Declaration{
		Name: "Write",
		Type: data.OwnType,
		Parameters: []data.Parameter{
			{
				Name: "data",
				Type: data.ArrayType(data.ByteType),
			},
		},
		Returns: []*data.Type{data.IntType, data.ErrorType},
	}, nil).
	DefineNativeFunction("WriteString", &data.Declaration{
		Name: "WriteString",
		Type: data.OwnType,
		Parameters: []data.Parameter{
			{
				Name: "text",
				Type: data.StringType,
			},
		},
		Returns: []*data.Type{data.IntType, data.ErrorType},
	}, nil).
	DefineNativeFunction("WriteByte", &data.Declaration{
		Name: "WriteByte",
		Type: data.OwnType,
		Parameters: []data.Parameter{
			{
				Name: "c",
				Type: data.ByteType,
			},
		},
		Returns: []*data.Type{data.IntType, data.ErrorType},
	}, nil).
	DefineNativeFunction("WriteRune", &data.Declaration{
		Name: "WriteRune",
		Type: data.OwnType,
		Parameters: []data.Parameter{
			{
				Name: "r",
				Type: data.Int32Type,
			},
		},
		Returns: []*data.Type{data.IntType, data.ErrorType},
	}, nil)

var StringsPackage = data.NewPackageFromMap("strings", map[string]interface{}{
	"Reader":  StringsReaderType,
	"Builder": StringsBuilderType,
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
		Value: chars,
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
		Value:    strings.Compare,
		IsNative: true,
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
		Value:    strings.Contains,
		IsNative: true,
	},
	"ContainsAny": data.Function{
		Declaration: &data.Declaration{
			Name: "ContainsAny",
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
		Value:    strings.ContainsAny,
		IsNative: true,
	},
	"Count": data.Function{
		Declaration: &data.Declaration{
			Name: "Count",
			Parameters: []data.Parameter{
				{
					Name: "text",
					Type: data.StringType,
				},
				{
					Name: "substring",
					Type: data.StringType,
				},
			},
			Returns: []*data.Type{data.IntType},
		},
		Value:    strings.Count,
		IsNative: true,
	},
	"Cut": data.Function{
		Declaration: &data.Declaration{
			Name: "Cut",
			Parameters: []data.Parameter{
				{
					Name: "text",
					Type: data.StringType,
				},
				{
					Name: "sep",
					Type: data.StringType,
				},
			},
			Returns: []*data.Type{data.StringType, data.StringType, data.BoolType},
		},
		Value:    strings.Cut,
		IsNative: true,
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
		Value:    strings.EqualFold,
		IsNative: true,
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
		Value:    strings.Fields,
		IsNative: true,
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
			Variadic: true,
			Returns:  []*data.Type{data.StringType},
		},
		Value: format,
	},
	"Index": data.Function{
		Declaration: &data.Declaration{
			Name: "Index",
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
		Value:    strings.Index,
		IsNative: true,
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
		Value:    strings.Join,
		IsNative: true,
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
					Name: "count",
					Type: data.IntType,
				},
			},
			Returns: []*data.Type{data.StringType},
		},
		Value: leftSubstring,
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
		Value: length,
	},
	"NewReader": data.Function{
		Declaration: &data.Declaration{
			Name: "NewReader",
			Parameters: []data.Parameter{
				{
					Name: "text",
					Type: data.StringType,
				},
			},
			Returns: []*data.Type{StringsReaderType},
		},
		Value:    strings.NewReader,
		IsNative: true,
	},
	"Replace": data.Function{
		Declaration: &data.Declaration{
			Name: "Replace",
			Parameters: []data.Parameter{
				{
					Name: "text",
					Type: data.StringType,
				},
				{
					Name: "old",
					Type: data.StringType,
				},
				{
					Name: "new",
					Type: data.StringType,
				},
				{
					Name: "count",
					Type: data.IntType,
				},
			},
			Returns: []*data.Type{data.StringType},
		},
		Value:    strings.Replace,
		IsNative: true,
	},
	"ReplaceAll": data.Function{
		Declaration: &data.Declaration{
			Name: "ReplaceAll",
			Parameters: []data.Parameter{
				{
					Name: "text",
					Type: data.StringType,
				},
				{
					Name: "old",
					Type: data.StringType,
				},
				{
					Name: "new",
					Type: data.StringType,
				},
			},
			Returns: []*data.Type{data.StringType},
		},
		Value:    strings.ReplaceAll,
		IsNative: true,
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
					Name: "count",
					Type: data.IntType,
				},
			},
			Returns: []*data.Type{data.StringType},
		},
		Value: rightSubstring,
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
		Value:    strings.Split,
		IsNative: true,
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
		Value: toString,
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
			Name: "ToLower",
			Parameters: []data.Parameter{
				{
					Name: "text",
					Type: data.StringType,
				},
			},
			Returns: []*data.Type{data.StringType},
		},
		Value:    strings.ToLower,
		IsNative: true,
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
		Value:    strings.ToUpper,
		IsNative: true,
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
			Returns: []*data.Type{data.ArrayType(StringsTokenArrayType)},
		},
		Value: tokenize,
	},
	"TrimPrefix": data.Function{
		Declaration: &data.Declaration{
			Name: "TrimPrefix",
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
		Value:    strings.TrimPrefix,
		IsNative: true,
	},
	"TrimSuffix": data.Function{
		Declaration: &data.Declaration{
			Name: "TrimSuffix",
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
		Value:    strings.TrimSuffix,
		IsNative: true,
	},
	"TrimSpace": data.Function{
		Declaration: &data.Declaration{
			Name: "TrimSpace",
			Parameters: []data.Parameter{
				{
					Name: "text",
					Type: data.StringType,
				},
			},
			Returns: []*data.Type{data.StringType},
		},
		Value:    strings.TrimSpace,
		IsNative: true,
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
		Value: truncate,
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
