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
			Declaration: &data.FunctionDeclaration{
				Name: "Byte",
				Parameters: []data.FunctionParameter{
					{
						Name:     "data",
						ParmType: data.ArrayType(data.ByteType),
					},
				},
				ReturnTypes: []*data.Type{data.ArrayType(data.ByteType)},
			},
			Value: Bytes,
		},
		"Float32s": data.Function{
			Declaration: &data.FunctionDeclaration{
				Name: "Float32s",
				Parameters: []data.FunctionParameter{
					{
						Name:     "data",
						ParmType: data.ArrayType(data.Float32Type),
					},
				},
				ReturnTypes: []*data.Type{data.ArrayType(data.Float32Type)},
			},
			Value: Float32s,
		},
		"Float64s": data.Function{
			Declaration: &data.FunctionDeclaration{
				Name: "Float64s",
				Parameters: []data.FunctionParameter{
					{
						Name:     "data",
						ParmType: data.ArrayType(data.Float64Type),
					},
				},
				ReturnTypes: []*data.Type{data.ArrayType(data.Float64Type)},
			},
			Value: Float64s,
		},
		"Int32s": data.Function{
			Declaration: &data.FunctionDeclaration{
				Name: "Int32s",
				Parameters: []data.FunctionParameter{
					{
						Name:     "data",
						ParmType: data.ArrayType(data.Int32Type),
					},
				},
				ReturnTypes: []*data.Type{data.ArrayType(data.Int32Type)},
			},
			Value: Int32s,
		},
		"Int64s": data.Function{
			Declaration: &data.FunctionDeclaration{
				Name: "Int64s",
				Parameters: []data.FunctionParameter{
					{
						Name:     "data",
						ParmType: data.ArrayType(data.Int64Type),
					},
				},
				ReturnTypes: []*data.Type{data.ArrayType(data.Int64Type)},
			},
			Value: Int64s,
		},
		"Ints": data.Function{
			Declaration: &data.FunctionDeclaration{
				Name: "Ints",
				Parameters: []data.FunctionParameter{
					{
						Name:     "data",
						ParmType: data.ArrayType(data.IntType),
					},
				},
				ReturnTypes: []*data.Type{data.ArrayType(data.IntType)},
			},
			Value: Ints,
		},
		"Slice": data.Function{
			Declaration: &data.FunctionDeclaration{
				Name: "Slice",
				Parameters: []data.FunctionParameter{
					{
						Name:     "data",
						ParmType: data.ArrayType(data.InterfaceType),
					},
					{
						Name: "lessThan",
						ParmType: data.FunctionType(&data.Function{
							Declaration: &data.FunctionDeclaration{
								Name: "",
								Parameters: []data.FunctionParameter{
									{
										Name:     "data",
										ParmType: data.ArrayType(data.InterfaceType),
									},
									{
										Name:     "i",
										ParmType: data.IntType,
									}, {
										Name:     "j",
										ParmType: data.IntType,
									},
								},
								ReturnTypes: []*data.Type{data.BoolType},
							},
						}),
					},
				},
				ReturnTypes: []*data.Type{data.ArrayType(data.InterfaceType)},
			},
			Value: Slice,
		},
		"Sort": data.Function{
			Declaration: &data.FunctionDeclaration{
				Name: "Sort",
				Parameters: []data.FunctionParameter{
					{
						Name:     "data",
						ParmType: data.ArrayType(data.InterfaceType),
					},
				},
				ReturnTypes: []*data.Type{data.ArrayType(data.InterfaceType)},
			},
			Value: Sort,
		},
		"Strings": data.Function{
			Declaration: &data.FunctionDeclaration{
				Name: "Strings",
				Parameters: []data.FunctionParameter{
					{
						Name:     "data",
						ParmType: data.ArrayType(data.StringType),
					},
				},
				ReturnTypes: []*data.Type{data.ArrayType(data.StringType)},
			},
			Value: Strings,
		},
	}).SetBuiltins(true)

	pkg, _ = bytecode.GetPackage(newpkg.Name())
	pkg.Merge(newpkg)
	s.Root().SetAlways(newpkg.Name(), newpkg)
}
