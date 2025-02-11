package util

import (
	"github.com/tucats/ego/data"
)

var UtilSymbolTableType = data.TypeDefinition("SymbolTable", data.StructureType()).
	SetPackage("util").
	DefineField("depth", data.IntType).
	DefineField("name", data.StringType).
	DefineField("id", data.StringType).
	DefineField("root", data.BoolType).
	DefineField("shared", data.BoolType).
	DefineField("size", data.IntType)

var UtilMemoryType = data.TypeDefinition("MemoryStatus", data.StructureType()).
	SetPackage("util").
	DefineField("Time", data.StringType).
	DefineField("Current", data.Float64Type).
	DefineField("Total", data.Float64Type).
	DefineField("System", data.Float64Type).
	DefineField("GC", data.IntType)

var UtilPackage = data.NewPackageFromMap("util", map[string]interface{}{
	"Log": data.Function{
		Declaration: &data.Declaration{
			Name: "Log",
			Parameters: []data.Parameter{
				{
					Name: "count",
					Type: data.IntType,
				},
				{
					Name: "session",
					Type: data.IntType,
				},
			},
			ArgCount: data.Range{1, 2},
			Returns:  []*data.Type{data.ArrayType(data.StringType)},
		},
		Value: getLogContents,
	},
	"Memory": data.Function{
		Declaration: &data.Declaration{
			Name:    "Memory",
			Returns: []*data.Type{UtilMemoryType},
		},
		Value: getMemoryStats,
	},
	"Mode": data.Function{
		Declaration: &data.Declaration{
			Name:    "Mode",
			Returns: []*data.Type{data.StringType},
		},
		Value: getMode,
	},
	"Packages": data.Function{
		Declaration: &data.Declaration{
			Name:    "Packages",
			Returns: []*data.Type{data.ArrayType(data.StringType)},
		},
		Value: getPackages,
	},
	"SetLogger": data.Function{
		Declaration: &data.Declaration{
			Name: "SetLogger",
			Parameters: []data.Parameter{
				{
					Name: "name",
					Type: data.StringType,
				},
				{
					Name: "active",
					Type: data.BoolType,
				},
			},
			Returns: []*data.Type{data.BoolType},
		},
		Value: setLogger,
	},
	"Symbols": data.Function{
		Declaration: &data.Declaration{
			Name: "Symbols",
			Parameters: []data.Parameter{
				{
					Name: "scope",
					Type: data.IntType,
				},
				{
					Name: "format",
					Type: data.StringType,
				},
				{
					Name: "allSymbols",
					Type: data.BoolType,
				},
			},
			ArgCount: data.Range{0, 3},
		},
		Value: formatSymbols,
	},
	"SymbolTables": data.Function{
		Declaration: &data.Declaration{
			Name:    "SymbolTables",
			Returns: []*data.Type{UtilSymbolTableType},
		},
		Value: formatTables,
	},
})
