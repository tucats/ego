package json

import (
	"github.com/tucats/ego/data"
)

var JsonPackage = data.NewPackageFromMap("json", map[string]interface{}{
	"WriteFile": data.Function{
		Declaration: &data.Declaration{
			Name: "WriteFile",
			Parameters: []data.Parameter{
				{
					Name: "filename",
					Type: data.StringType,
				},
				{
					Name: "data",
					Type: data.InterfaceType,
				},
			},
			Returns: []*data.Type{data.ErrorType},
		},
		Value: writeFile,
	},
	"ReadFile": data.Function{
		Declaration: &data.Declaration{
			Name: "ReadFile",
			Parameters: []data.Parameter{
				{
					Name: "filename",
					Type: data.StringType,
				},
			},
			Returns: []*data.Type{data.InterfaceType, data.ErrorType},
		},
		Value: readFile,
	},
	"Marshal": data.Function{
		Declaration: &data.Declaration{
			Name: "Marshal",
			Parameters: []data.Parameter{
				{
					Name: "any",
					Type: data.InterfaceType,
				},
			},
			Returns: []*data.Type{data.ArrayType(data.ByteType)},
		},
		Value: marshal,
	},
	"MarshalIndent": data.Function{
		Declaration: &data.Declaration{
			Name: "MarshalIndent",
			Parameters: []data.Parameter{
				{
					Name: "any",
					Type: data.InterfaceType,
				},
				{
					Name: "prefix",
					Type: data.StringType,
				},
				{
					Name: "indent",
					Type: data.StringType,
				},
			},
			Returns: []*data.Type{data.ArrayType(data.ByteType)},
		},
		Value: marshalIndent,
	},
	"Unmarshal": data.Function{
		Declaration: &data.Declaration{
			Name: "Unmarshal",
			Parameters: []data.Parameter{
				{
					Name: "data",
					Type: data.ArrayType(data.ByteType),
				},
				{
					Name: "value",
					Type: data.PointerType(data.InterfaceType),
				},
			},
			Returns: []*data.Type{data.ErrorType},
		},
		Value: unmarshal,
	},
})
