package http

import (
	"github.com/tucats/ego/data"
)

var URLType = data.TypeDefinition("URL",
	data.StructureType().
		DefineField("Path", data.StringType).
		DefineField("Parts", data.MapType(data.StringType, data.InterfaceType))).
	SetPackage("http")

var RequestType = data.TypeDefinition("Request",
	data.StructureType().
		DefineField("URL", URLType).
		DefineField("Endpoint", data.StringType).
		DefineField("Headers", data.MapType(data.StringType, data.ArrayType(data.StringType))).
		DefineField("Parameters", data.MapType(data.StringType, data.ArrayType(data.StringType))).
		DefineField("Method", data.StringType).
		DefineField("Body", data.StringType).
		DefineField("Username", data.StringType).
		DefineField("IsAdmin", data.BoolType).
		DefineField("IsJSON", data.BoolType).
		DefineField("IsText", data.BoolType).
		DefineField("Authenticated", data.BoolType).
		DefineField("SessionID", data.StringType).
		DefineField("Permissions", data.ArrayType(data.StringType)).
		DefineField("Authentication", data.StringType))

var HeaderType = data.TypeDefinition("Header", data.StructureType().
	DefineField("_headers", data.MapType(data.StringType, data.ArrayType(data.StringType)))).
	DefineFunctions(map[string]data.Function{
		"Add": {
			Declaration: &data.Declaration{
				Name: "Add",
				Type: data.OwnType,
				Parameters: []data.Parameter{
					{Name: "key", Type: data.StringType},
					{Name: "value", Type: data.StringType},
				},
			},
			Value: Add,
		},
		"Del": {
			Declaration: &data.Declaration{
				Name: "Del",
				Type: data.OwnType,
				Parameters: []data.Parameter{
					{Name: "key", Type: data.StringType},
				},
			},
			Value: Del,
		},
		"Set": {
			Declaration: &data.Declaration{
				Name: "Set",
				Type: data.OwnType,
				Parameters: []data.Parameter{
					{Name: "key", Type: data.StringType},
					{Name: "value", Type: data.StringType},
				},
			},
			Value: Add,
		},
	})

var ResponseWriterType = data.TypeDefinition("ResponseWriter",
	data.StructureType().
		DefineField("_writer", data.InterfaceType).
		DefineField("_status", data.IntType).
		DefineField("_headers", HeaderType).
		DefineField("_json", data.BoolType).
		DefineField("_text", data.BoolType).
		DefineField("Valid", data.BoolType).
		DefineField("_body", data.ArrayType(data.ByteType)).
		DefineField("_size", data.IntType)).
	SetPackage("http").
	DefineFunctions(map[string]data.Function{
		"Header": {
			Declaration: &data.Declaration{
				Name:    "Header",
				Type:    data.OwnType,
				Returns: []*data.Type{HeaderType},
			},
			Value: Header,
		},
		"Write": {
			Declaration: &data.Declaration{
				Name: "Write",
				Type: data.OwnType,
				Parameters: []data.Parameter{
					{
						Name: "data",
						Type: data.InterfaceType},
				},
				Returns: []*data.Type{data.IntType, data.ErrorType},
			},
			Value: Write,
		},
		"WriteHeader": {
			Declaration: &data.Declaration{
				Name: "WriteStatus",
				Type: data.OwnType,
				Parameters: []data.Parameter{
					{
						Name: "status",
						Type: data.IntType,
					},
				},
			},
			Value: WriteHeader,
		},
	})

var HttpPackage = data.NewPackageFromMap("http", map[string]interface{}{
	"Header":         HeaderType,
	"ResponseWriter": ResponseWriterType,
	"URLType":        URLType,
	"Request":        RequestType,
})
