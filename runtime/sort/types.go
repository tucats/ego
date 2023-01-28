package sort

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/symbols"
)

func Initialize(s *symbols.SymbolTable) {
	var pkg *data.Package

	newpkg := data.NewPackageFromMap("sort", map[string]interface{}{
		"Bytes": data.Function{
			Declaration: &data.Declaration{
				Name: "Byte",
				Parameters: []data.Parameter{
					{
						Name: "data",
						Type: data.ArrayType(data.ByteType),
					},
				},
				Returns: []*data.Type{data.ArrayType(data.ByteType)},
			},
			Value: sortBytes,
		},
		"Float32s": data.Function{
			Declaration: &data.Declaration{
				Name: "Float32s",
				Parameters: []data.Parameter{
					{
						Name: "data",
						Type: data.ArrayType(data.Float32Type),
					},
				},
				Returns: []*data.Type{data.ArrayType(data.Float32Type)},
			},
			Value: sortFloat32s,
		},
		"Float64s": data.Function{
			Declaration: &data.Declaration{
				Name: "Float64s",
				Parameters: []data.Parameter{
					{
						Name: "data",
						Type: data.ArrayType(data.Float64Type),
					},
				},
				Returns: []*data.Type{data.ArrayType(data.Float64Type)},
			},
			Value: sortFloat64s,
		},
		"Int32s": data.Function{
			Declaration: &data.Declaration{
				Name: "Int32s",
				Parameters: []data.Parameter{
					{
						Name: "data",
						Type: data.ArrayType(data.Int32Type),
					},
				},
				Returns: []*data.Type{data.ArrayType(data.Int32Type)},
			},
			Value: sortInt32s,
		},
		"Int64s": data.Function{
			Declaration: &data.Declaration{
				Name: "Int64s",
				Parameters: []data.Parameter{
					{
						Name: "data",
						Type: data.ArrayType(data.Int64Type),
					},
				},
				Returns: []*data.Type{data.ArrayType(data.Int64Type)},
			},
			Value: sortInt64s,
		},
		"Ints": data.Function{
			Declaration: &data.Declaration{
				Name: "Ints",
				Parameters: []data.Parameter{
					{
						Name: "data",
						Type: data.ArrayType(data.IntType),
					},
				},
				Returns: []*data.Type{data.ArrayType(data.IntType)},
			},
			Value: sortInts,
		},
		"Slice": data.Function{
			Declaration: &data.Declaration{
				Name:  "Slice",
				Scope: true,
				Parameters: []data.Parameter{
					{
						Name: "data",
						Type: data.ArrayType(data.InterfaceType),
					},
					{
						Name: "lessThan",
						Type: data.FunctionType(&data.Function{
							Declaration: &data.Declaration{
								Name: "",
								Parameters: []data.Parameter{
									{
										Name: "data",
										Type: data.ArrayType(data.InterfaceType),
									},
									{
										Name: "i",
										Type: data.IntType,
									}, {
										Name: "j",
										Type: data.IntType,
									},
								},
								Returns: []*data.Type{data.BoolType},
							},
						}),
					},
				},
				Returns: []*data.Type{data.ArrayType(data.InterfaceType)},
			},
			Value: sortSlice,
		},
		"Sort": data.Function{
			Declaration: &data.Declaration{
				Name: "Sort",
				Parameters: []data.Parameter{
					{
						Name: "data",
						Type: data.ArrayType(data.InterfaceType),
					},
				},
				Returns: []*data.Type{data.ArrayType(data.InterfaceType)},
			},
			Value: genericSort,
		},
		"Strings": data.Function{
			Declaration: &data.Declaration{
				Name: "Strings",
				Parameters: []data.Parameter{
					{
						Name: "data",
						Type: data.ArrayType(data.StringType),
					},
				},
				Returns: []*data.Type{data.ArrayType(data.StringType)},
			},
			Value: sortStrings,
		},
	}).SetBuiltins(true)

	pkg, _ = bytecode.GetPackage(newpkg.Name())
	pkg.Merge(newpkg)
	s.Root().SetAlways(newpkg.Name(), newpkg)
}
