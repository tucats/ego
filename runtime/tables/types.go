package tables

import (
	"github.com/tucats/ego/data"
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

var TablesTableType = data.TypeDefinition("Table",
	data.StructureType().
		DefineField(headingsFieldName, data.ArrayType(data.StringType)).
		DefineField(tableFieldName, data.InterfaceType).
		DefineFunctions(map[string]data.Function{
			"AddRow": {
				Declaration: &data.Declaration{
					Name:     "AddRow",
					Type:     data.OwnType,
					Variadic: true,
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
					Type: data.OwnType,
					Returns: []*data.Type{
						data.ErrorType,
					},
				},
				Value: closeTable,
			},

			"Sort": {
				Declaration: &data.Declaration{
					Name:     "Sort",
					Type:     data.OwnType,
					Variadic: true,
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
					Type: data.OwnType,
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
					Type: data.OwnType,
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
					Type: data.OwnType,
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
					Type: data.OwnType,
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
					Type: data.OwnType,
					Returns: []*data.Type{
						data.IntType,
					},
				},
				Value: lenTable,
			},
			"Width": {
				Declaration: &data.Declaration{
					Name: "Width",
					Type: data.OwnType,
					Returns: []*data.Type{
						data.IntType,
					},
				},
				Value: widthTable,
			},
			"Find": {
				Declaration: &data.Declaration{
					Name: "Find",
					Type: data.OwnType,
					Parameters: []data.Parameter{
						{
							Name: "eval",
							Type: data.FunctionType(&data.Function{
								Declaration: &data.Declaration{
									Name: "",
									Parameters: []data.Parameter{
										{
											Name: "column",
											Type: data.ArrayType(data.StringType),
										},
									},
									ArgCount: data.Range{1, -1},
									Variadic: true,
									Returns:  []*data.Type{data.BoolType},
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
					Type: data.OwnType,
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
					Type: data.OwnType,
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
					Type: data.OwnType,
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
		}),
).SetPackage("tables")

var TablesPackage = data.NewPackageFromMap("tables", map[string]interface{}{
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
			Returns:  []*data.Type{TablesTableType},
		},
		Value: newTable,
	},
	"Table": TablesTableType,
})
