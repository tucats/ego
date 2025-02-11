package exec

import (
	"github.com/tucats/ego/data"
)

var ExecCmdType = data.TypeDefinition("Cmd", data.StructType).
	DefineField("cmd", data.InterfaceType).
	DefineField("Dir", data.StringType).
	DefineField("Path", data.StringType).
	DefineField("Args", data.ArrayType(data.StringType)).
	DefineField("Env", data.ArrayType(data.StringType)).
	DefineField("Stdout", data.ArrayType(data.StringType)).
	DefineField("Stdin", data.ArrayType(data.StringType)).
	DefineFunctions(map[string]data.Function{
		"Output": {
			Declaration: &data.Declaration{
				Name:    "Output",
				Type:    data.OwnType,
				Returns: []*data.Type{data.ArrayType(data.StringType), data.ErrorType},
			},
			Value: output},
		"Run": {
			Declaration: &data.Declaration{
				Name:    "Run",
				Type:    data.OwnType,
				Returns: []*data.Type{data.ErrorType},
			},
			Value: run},
	}).
	SetPackage("exec")

var ExecPackage = data.NewPackageFromMap("exec", map[string]interface{}{
	"Command": data.Function{
		Declaration: &data.Declaration{
			Name: "Command",
			Parameters: []data.Parameter{
				{
					Name: "commandText",
					Type: data.StringType,
				},
				{
					Name: "argument",
					Type: data.StringType,
				},
			},
			Returns:  []*data.Type{ExecCmdType},
			Variadic: true,
		},
		Value: newCommand,
	},
	"LookPath": data.Function{
		Declaration: &data.Declaration{
			Name: "LookPath",
			Parameters: []data.Parameter{
				{
					Name: "file",
					Type: data.StringType,
				},
			},
			Returns: []*data.Type{data.StringType, data.ErrorType},
		},
		Value: lookPath,
	},
	"Cmd": ExecCmdType,
})
