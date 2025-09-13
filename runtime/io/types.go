package io

import (
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/runtime/time"
)

const (
	fileFieldName    = "File"
	nameFieldName    = "Name"
	validFieldName   = "Valid"
	scannerFieldName = "Scanner"
	modeFieldName    = "Mode"
)

var IoFileType = data.TypeDefinition("File",
	data.StructureType().
		DefineField(fileFieldName, data.InterfaceType).
		DefineField(validFieldName, data.BoolType).
		DefineField(nameFieldName, data.StringType).
		DefineField(modeFieldName, data.StringType).
		DefineFunction("Close", &data.Declaration{
			Name:    "Close",
			Type:    data.OwnType,
			Returns: []*data.Type{data.ErrorType},
		}, closeFile).
		DefineFunction("ReadString", &data.Declaration{
			Name:    "ReadString",
			Type:    data.OwnType,
			Returns: []*data.Type{data.StringType, data.ErrorType},
		}, readString).
		DefineFunction("WriteString", &data.Declaration{
			Name: "WriteString",
			Type: data.OwnType,
			Parameters: []data.Parameter{
				{
					Name: "data",
					Type: data.StringType,
				},
			},
			Returns: []*data.Type{data.IntType, data.ErrorType},
		}, writeString).
		DefineFunction("Write", &data.Declaration{
			Name: "Write",
			Type: data.OwnType,
			Parameters: []data.Parameter{
				{
					Name: "data",
					Type: data.ArrayType(data.ByteType),
				},
			},
			Returns: []*data.Type{data.IntType, data.ErrorType},
		}, write).
		DefineFunction("WriteAt", &data.Declaration{
			Name: "WriteAt",
			Type: data.OwnType,
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
		}, writeAt).
		DefineFunction("String", &data.Declaration{
			Name:    "String",
			Type:    data.OwnType,
			Returns: []*data.Type{data.StringType},
		}, asString),
).SetPackage("io").FixSelfReferences()

var IoEntryType = data.TypeDefinition("Entry",
	data.StructureType().
		DefineField("Name", data.StringType).
		DefineField("IsDirectory", data.BoolType).
		DefineField("Mode", data.StringType).
		DefineField("Size", data.IntType).
		DefineField("Modified", time.TimeType),
).SetPackage("io")

var IoPackage = data.NewPackageFromMap("io", map[string]any{
	"File":  IoFileType,
	"Entry": IoEntryType,
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
		Value: expand,
	},
	"Open": data.Function{
		Declaration: &data.Declaration{
			Name:     "Open",
			ArgCount: data.Range{1, 2},
			Parameters: []data.Parameter{
				{
					Name: "filename",
					Type: data.StringType,
				},
				{
					Name: "mode",
					Type: data.StringType,
				},
			},
			Returns: []*data.Type{IoFileType, data.ErrorType},
		},
		Value: openFile,
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
			Returns: []*data.Type{data.ArrayType(IoEntryType), data.ErrorType},
		},
		Value: readDirectory,
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
		Value: prompt,
	},
})
