package util

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/compiler"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/symbols"
)

var symbolTableTypeDef *data.Type
var memoryTypeDef *data.Type

func Initialize(s *symbols.SymbolTable) {
	var pkg *data.Package

	// Compile the type definition for the structure we're going to return.
	symbolTableTypeDef, _ = compiler.CompileTypeSpec(`
	type SymbolTable struct{
		depth int
		name string
		id string
		root bool
		shared bool
		size int
		}`)

	memoryTypeDef, _ = compiler.CompileTypeSpec(`
		type MemoryStatus struct {
			Time string
			Current float64
			Total float64
			System float64
			GC int
		}`)

	memoryTypeDef.SetPackage("util")
	symbolTableTypeDef.SetPackage("util")

	newpkg := data.NewPackageFromMap("util", map[string]interface{}{
		"Eval": data.Function{
			Declaration: &data.Declaration{
				Name: "Eval",
				Parameters: []data.Parameter{
					{
						Name: "expression",
						Type: data.StringType,
					},
				},
				Returns: []*data.Type{data.InterfaceType},
			},
			Value: Eval,
		},
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
				Returns: []*data.Type{memoryTypeDef},
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
				Returns: []*data.Type{symbolTableTypeDef},
			},
			Value: formatTables,
		},
	}).SetBuiltins(true)

	pkg, _ = bytecode.GetPackage("util")
	pkg.Merge(newpkg)
	s.Root().SetAlways(newpkg.Name(), newpkg)
}
