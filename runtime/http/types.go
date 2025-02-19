package http

import "github.com/tucats/ego/data"

var HeaderType = data.TypeDefinition("Header",
	data.StructureType().
		DefineField("_header", data.IntType)).SetPackage("http").
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
	})

var ResponseWriterType = data.TypeDefinition("ResponseWriter",
	data.StructureType().
		DefineField("_writer", data.InterfaceType).
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
					{Name: "b", Type: data.ArrayType(data.ByteType)},
				},
				Returns: []*data.Type{data.IntType, data.ErrorType},
			},
			Value: Write,
		},
		"WriteStatus": {
			Declaration: &data.Declaration{
				Name: "WriteStatus",
				Type: data.OwnType,
				Parameters: []data.Parameter{
					{Name: "status", Type: data.IntType},
				},
			},
			Value: WriteStatus,
		},
	})

var HttpPackage = data.NewPackageFromMap("http", map[string]interface{}{
	"Header":         HeaderType,
	"ResponseWriter": ResponseWriterType,
})
