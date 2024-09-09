package io

import (
	"sync"

	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/runtime/time"
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
var initLock sync.Mutex

func Initialize(s *symbols.SymbolTable) {
	initLock.Lock()
	defer initLock.Unlock()

	if entryType == nil {
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
				Type: time.GetTimeType(s),
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
		}, closeFile)

		t.DefineFunction("ReadString", &data.Declaration{
			Name:    "ReadString",
			Type:    data.PointerType(t),
			Returns: []*data.Type{data.StringType, data.ErrorType},
		}, readString)

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
		}, writeString)

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
		}, write)

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
		}, writeAt)

		t.DefineFunction("String", &data.Declaration{
			Name:    "String",
			Type:    data.PointerType(t),
			Returns: []*data.Type{data.StringType},
		}, asString)

		fileType = t.SetPackage("io")
	}

	if _, found := s.Root().Get("io"); !found {
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
					Returns: []*data.Type{fileType, data.ErrorType},
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
					Returns: []*data.Type{data.ArrayType(entryType), data.ErrorType},
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

		pkg, _ := bytecode.GetPackage(newpkg.Name)
		pkg.Merge(newpkg)
		s.Root().SetAlways(newpkg.Name, newpkg)
	}
}
