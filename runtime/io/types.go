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

	t.DefineFunction("Close", &data.Declaration{
		Name:    "Close",
		Type:    data.PointerType(t),
		Returns: []*data.Type{data.ErrorType},
	}, Close)

	t.DefineFunction("ReadString", &data.Declaration{
		Name:    "ReadString",
		Type:    data.PointerType(t),
		Returns: []*data.Type{data.StringType, data.ErrorType},
	}, ReadString)

	t.DefineFunction("WriteString", &data.Declaration{
		Name: "WriteString",
		Type: data.PointerType(t),
		Parameters: []data.Parameter{
			{
				Name: "data",
				Type: data.StringType,
			},
		},
		Returns: []*data.Type{data.IntType, data.ErrorType},
	}, WriteString)

	t.DefineFunction("Write", &data.Declaration{
		Name: "Write",
		Type: data.PointerType(t),
		Parameters: []data.Parameter{
			{
				Name: "data",
				Type: data.ArrayType(data.ByteType),
			},
		},
		Returns: []*data.Type{data.IntType, data.ErrorType},
	}, Write)

	t.DefineFunction("WriteAt", &data.Declaration{
		Name: "WriteAt",
		Type: data.PointerType(t),
		Parameters: []data.Parameter{
			{
				Name: "data",
				Type: data.ArrayType(data.ByteType),
			},
			{
				Name: "offset",
				Type: data.IntType,
			},
		},
		Returns: []*data.Type{data.IntType, data.ErrorType},
	}, Write)

	t.DefineFunction("String", &data.Declaration{
		Name:    "String",
		Type:    data.PointerType(t),
		Returns: []*data.Type{data.StringType},
	}, AsString)

	fileType = t.SetPackage("io")
	newpkg := data.NewPackageFromMap("io", map[string]interface{}{
		"File":  fileType,
		"Entry": entryType,
		"Expand": data.Function{
			Declaration: &data.Declaration{
				Name: "Expand",
				Parameters: []data.Parameter{
					{
						Name: "path",
						Type: data.StringType,
					},
					{
						Name: "filter",
						Type: data.StringType,
					},
				},
				Returns:  []*data.Type{data.ArrayType(data.StringType)},
				ArgCount: data.Range{1, 2},
			},
			Value: Expand,
		},
		"Open": data.Function{
			Declaration: &data.Declaration{
				Name: "Open",
				Parameters: []data.Parameter{
					{
						Name: "filename",
						Type: data.StringType,
					},
				},
				Returns: []*data.Type{fileType, data.ErrorType},
			},
			Value: Open,
		},
		"ReadDir": data.Function{
			Declaration: &data.Declaration{
				Name: "ReadDir",
				Parameters: []data.Parameter{
					{
						Name: "path",
						Type: data.StringType,
					},
				},
				Returns: []*data.Type{data.ArrayType(entryType)},
			},
			Value: ReadDir,
		},
		"Prompt": data.Function{
			Declaration: &data.Declaration{
				Name: "Prompt",
				Parameters: []data.Parameter{
					{
						Name: "text",
						Type: data.StringType,
					},
				},
				Returns: []*data.Type{data.StringType},
			},
			Value: Prompt,
		},
	}).SetBuiltins(true)

	pkg, _ := bytecode.GetPackage(newpkg.Name())
	pkg.Merge(newpkg)
	s.Root().SetAlways(newpkg.Name(), newpkg)
}
