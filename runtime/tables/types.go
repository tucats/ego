package tables

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/compiler"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/symbols"
)

// tables.Table type specification.
const tableTypeSpec = `
	type Table struct {
		table 	 interface{},
		Headings []string,
	}`

const (
	headingsFieldName = "Headings"
	tableFieldName    = "table"
)

var tableTypeDef *data.Type

func Initialize(s *symbols.SymbolTable) {
	t, _ := compiler.CompileTypeSpec(tableTypeSpec, nil)

	t.DefineFunctions(map[string]data.Function{
		"AddRow": {
			Declaration: &data.Declaration{
				Name:     "AddRow",
				Variadic: true,
				Type:     data.PointerType(t),
				Parameters: []data.Parameter{
					{
						Name: "value",
						Type: data.InterfaceType,
					},
				},
				Returns: []*data.Type{
					data.ErrorType,
				},
			},
			Value: addRow,
		},

		"Close": {
			Declaration: &data.Declaration{
				Name: "Close",
				Type: data.PointerType(t),
				Returns: []*data.Type{
					data.ErrorType,
				},
			},
			Value: closeTable,
		},

		"Sort": {
			Declaration: &data.Declaration{
				Name:     "Sort",
				Variadic: true,
				Type:     data.PointerType(t),
				Parameters: []data.Parameter{
					{
						Name: "columnName",
						Type: data.StringType,
					},
				},
				Returns: []*data.Type{
					data.ErrorType,
				},
			},
			Value: sortTable,
		},

		"Print": {
			Declaration: &data.Declaration{
				Name: "Print",
				Type: data.PointerType(t),
				Parameters: []data.Parameter{
					{
						Name: "format",
						Type: data.StringType,
					},
				},
				Returns: []*data.Type{
					data.ErrorType,
				},
				ArgCount: data.Range{0, 1},
			},
			Value: printTable,
		},

		"Format": {
			Declaration: &data.Declaration{
				Name: "Format",
				Type: data.PointerType(t),
				Parameters: []data.Parameter{
					{
						Name: "headings",
						Type: data.BoolType,
					},
					{
						Name: "underlines",
						Type: data.BoolType,
					},
				},
				Returns: []*data.Type{
					data.ErrorType,
				},
			},
			Value: setFormat,
		},

		"Align": {
			Declaration: &data.Declaration{
				Name: "Align",
				Type: data.PointerType(t),
				Parameters: []data.Parameter{
					{
						Name: "columnName",
						Type: data.StringType,
					},
					{
						Name: "alignment",
						Type: data.StringType,
					},
				},
				Returns: []*data.Type{
					data.ErrorType,
				},
			},
			Value: setAlignment,
		},

		"String": {
			Declaration: &data.Declaration{
				Name: "String",
				Type: data.PointerType(t),
				Parameters: []data.Parameter{
					{
						Name: "format",
						Type: data.StringType,
					},
				},
				Returns: []*data.Type{
					data.StringType,
					data.ErrorType,
				},
				ArgCount: data.Range{0, 1},
			},
			Value: toString,
		},
		"Len": {
			Declaration: &data.Declaration{
				Name: "Len",
				Type: data.PointerType(t),
				Returns: []*data.Type{
					data.IntType,
				},
			},
			Value: lenTable,
		},
		"Width": {
			Declaration: &data.Declaration{
				Name: "Width",
				Type: data.PointerType(t),
				Returns: []*data.Type{
					data.IntType,
				},
			},
			Value: widthTable,
		},
		"Find": {
			Declaration: &data.Declaration{
				Name: "Find",
				Type: data.PointerType(t),
				Parameters: []data.Parameter{
					{
						Name: "eval",
						Type: data.FunctionType(&data.Function{
							Declaration: &data.Declaration{
								Name: "",
								Parameters: []data.Parameter{
									{
										Name: "rowData",
										Type: data.ArrayType(data.StringType),
									},
								},
								Returns: []*data.Type{data.BoolType},
							},
						}),
					},
				},
				Returns: []*data.Type{
					data.ArrayType(data.IntType),
					data.ErrorType,
				},
			},
			Value: findRows,
		},
		"Get": {
			Declaration: &data.Declaration{
				Name: "Get",
				Type: data.PointerType(t),
				Parameters: []data.Parameter{
					{
						Name: "rowIndex",
						Type: data.IntType,
					},
					{
						Name: "columnName",
						Type: data.StringType,
					},
				},
				Returns: []*data.Type{
					data.StringType,
					data.ErrorType,
				},
			},
			Value: getTableElement,
		},
		"GetRow": {
			Declaration: &data.Declaration{
				Name: "GetRow",
				Type: data.PointerType(t),
				Parameters: []data.Parameter{
					{
						Name: "rowIndex",
						Type: data.IntType,
					},
				},
				Returns: []*data.Type{
					data.ArrayType(data.StringType),
					data.ErrorType,
				},
			},
			Value: getRow,
		},
		"Pagination": {
			Declaration: &data.Declaration{
				Name: "Pagination",
				Type: data.PointerType(t),
				Parameters: []data.Parameter{
					{
						Name: "width",
						Type: data.IntType,
					},
					{
						Name: "height",
						Type: data.IntType,
					},
				},
				Returns: []*data.Type{
					data.ErrorType,
				},
			},
			Value: setPagination,
		},
	})

	tableTypeDef = t.SetPackage("tables")

	newpkg := data.NewPackageFromMap("tables", map[string]interface{}{
		"New": data.Function{
			Declaration: &data.Declaration{
				Name: "New",
				Parameters: []data.Parameter{
					{
						Name: "column",
						Type: data.StringType,
					},
				},
				Variadic: true,
				Returns:  []*data.Type{tableTypeDef},
			},
			Value: newTable,
		},
		"Table": tableTypeDef,
	})

	pkg, _ := bytecode.GetPackage(newpkg.Name)
	pkg.Merge(newpkg)
	s.Root().SetAlways(newpkg.Name, newpkg)
}
