package reflect

import (
	"github.com/tucats/ego/data"
)

var MaxDeepCopyDepth int = 100

var ReflectParameterType = data.TypeDefinition("Parameter",
	data.StructureType().
		DefineField("Name", data.StringType).
		DefineField("Type", data.StringType),
).SetPackage("reflect")

var ReflectFunctionType = data.TypeDefinition("Function",
	data.StructureType().
		DefineField("Name", data.StringType).
		DefineField("Parameters", data.ArrayType(ReflectParameterType)).
		DefineField("Returns", data.ArrayType(data.StringType)).
		DefineField("Argcount", data.ArrayType(data.IntType)),
).SetPackage("reflect")

var ReflectReflectionType = data.TypeDefinition("Reflection",
	data.StructureType().
		DefineField("Name", data.StringType).
		DefineField("Type", data.TypeType).
		DefineField("Istype", data.BoolType).
		DefineField("Native", data.BoolType).
		DefineField("Imports", data.BoolType).
		DefineField("Builtins", data.BoolType).
		DefineField("Basetype", data.StringType).
		DefineField("Members", data.ArrayType(data.StringType)).
		DefineField("Size", data.IntType).
		DefineField("Error", data.ErrorType).
		DefineField("Text", data.StringType).
		DefineField("Context", data.StringType).
		DefineField("Declaration", ReflectFunctionType).
		DefineFunction("String", &data.Declaration{
			Name:    "String",
			Type:    data.OwnType,
			Returns: []*data.Type{data.StringType},
		}, getString),
).SetPackage("reflect").FixSelfreferences()

var ReflectPackage = data.NewPackageFromMap("reflect", map[string]interface{}{
	"arameter":   ReflectParameterType,
	"Function":   ReflectFunctionType,
	"Reflection": ReflectReflectionType,
	"DeepCopy": data.Function{
		Declaration: &data.Declaration{
			Name: "DeepCopy",
			Parameters: []data.Parameter{
				{
					Name: "any",
					Type: data.InterfaceType,
				},
				{
					Name: "depth",
					Type: data.IntType,
				},
			},
			ArgCount: data.Range{1, 2},
			Returns:  []*data.Type{data.InterfaceType},
		},
		Value: deepCopy,
	},
	"InstanceOf": data.Function{
		Declaration: &data.Declaration{
			Name: "InstanceOf",
			Parameters: []data.Parameter{
				{
					Name: "any",
					Type: data.InterfaceType,
				},
			},
			Returns: []*data.Type{data.InterfaceType},
		},
		Value: instanceOf,
	},
	"Members": data.Function{
		Declaration: &data.Declaration{
			Name: "Members",
			Parameters: []data.Parameter{
				{
					Name: "any",
					Type: data.InterfaceType,
				},
			},
			Returns: []*data.Type{data.ArrayType(data.StringType)},
		},
		Value: members,
	},
	"Reflect": data.Function{
		Declaration: &data.Declaration{
			Name: "Reflect",
			Parameters: []data.Parameter{
				{
					Name: "any",
					Type: data.InterfaceType,
				},
			},
			Returns: []*data.Type{data.StructType},
		},
		Value: describe,
	},
	"Type": data.Function{
		Extension: true,
		Declaration: &data.Declaration{
			Name: "Type",
			Parameters: []data.Parameter{
				{
					Name: "any",
					Type: data.InterfaceType,
				},
			},
			Returns: []*data.Type{data.TypeType},
		},
		Value: describeType,
	},
})
