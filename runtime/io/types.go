package io

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/symbols"
)

const (
	fileFieldName    = "File"
	nameFieldName    = "Name"
	validFieldName   = "Valid"
	scannerFieldName = "Scanner"
	modeFieldName    = "Mode"
)

var fileType *data.Type
var entryType *data.Type

func Initialize(s *symbols.SymbolTable) {
	entryType = data.StructureType(
		data.Field{
			Name: "Name",
			Type: data.StringType,
		},
		data.Field{
			Name: "IsDirectory",
			Type: data.BoolType,
		},
		data.Field{
			Name: "Mode",
			Type: data.StringType,
		},
		data.Field{
			Name: "Size",
			Type: data.IntType,
		},
		data.Field{
			Name: "Modified",
			Type: data.StringType,
		},
	).SetPackage("io").SetName("Entry")

	structType := data.StructureType()

	structType.DefineField(fileFieldName, data.InterfaceType).
		DefineField(validFieldName, data.BoolType).
		DefineField(nameFieldName, data.StringType).
		DefineField(modeFieldName, data.StringType)

	t := data.TypeDefinition("File", structType)

	t.DefineFunction("Close", &data.FunctionDeclaration{
		Name:         "Close",
		ReceiverType: data.PointerType(t),
		ReturnTypes:  []*data.Type{data.ErrorType},
	}, Close)

	t.DefineFunction("ReadString", &data.FunctionDeclaration{
		Name:         "ReadString",
		ReceiverType: data.PointerType(t),
		ReturnTypes:  []*data.Type{data.StringType, data.ErrorType},
	}, ReadString)

	t.DefineFunction("WriteString", &data.FunctionDeclaration{
		Name:         "WriteString",
		ReceiverType: data.PointerType(t),
		Parameters: []data.FunctionParameter{
			{
				Name:     "data",
				ParmType: data.StringType,
			},
		},
		ReturnTypes: []*data.Type{data.IntType, data.ErrorType},
	}, WriteString)

	t.DefineFunction("Write", &data.FunctionDeclaration{
		Name:         "Write",
		ReceiverType: data.PointerType(t),
		Parameters: []data.FunctionParameter{
			{
				Name:     "data",
				ParmType: data.ArrayType(data.ByteType),
			},
		},
		ReturnTypes: []*data.Type{data.IntType, data.ErrorType},
	}, Write)

	t.DefineFunction("WriteAt", &data.FunctionDeclaration{
		Name:         "WriteAt",
		ReceiverType: data.PointerType(t),
		Parameters: []data.FunctionParameter{
			{
				Name:     "data",
				ParmType: data.ArrayType(data.ByteType),
			},
			{
				Name:     "offset",
				ParmType: data.IntType,
			},
		},
		ReturnTypes: []*data.Type{data.IntType, data.ErrorType},
	}, Write)

	t.DefineFunction("String", &data.FunctionDeclaration{
		Name:         "String",
		ReceiverType: data.PointerType(t),
		ReturnTypes:  []*data.Type{data.StringType},
	}, AsString)

	fileType = t.SetPackage("io")
	newpkg := data.NewPackageFromMap("io", map[string]interface{}{
		"File":             fileType,
		"Entry":            entryType,
		"Expand":           Expand,
		"Open":             Open,
		"ReadDir":          ReadDir,
		"Prompt":           Prompt,
		data.TypeMDKey:     data.PackageType("io"),
		data.ReadonlyMDKey: true,
	}).SetBuiltins(true)

	pkg, _ := bytecode.GetPackage(newpkg.Name())
	pkg.Merge(newpkg)
	s.Root().SetAlways(newpkg.Name(), newpkg)
}
