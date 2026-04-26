// Package tables implements the Ego "tables" built-in package.  It bridges
// the Ego scripting language to the app-cli/tables Go package, which handles
// the actual table construction, formatting, and output.
//
// # Architecture
//
// Every Table object visible to Ego code is a *data.Struct whose type is
// TablesTableType.  That struct has two fields:
//
//   - "Headings" – a read-only *data.Array of column-name strings exposed to
//     Ego code so scripts can inspect the schema without mutating it.
//   - "table"   – an opaque interface{} slot that holds the underlying
//     *app-cli/tables.Table pointer; Ego code never reads this field directly.
//
// All method implementations in this package follow the same calling
// convention required by the Ego runtime:
//
//	func name(s *symbols.SymbolTable, args data.List) (any, error)
//
// The receiver (the Table object the method was called on) is retrieved from
// the symbol table via the special variable defs.ThisVariable ("__this").
// Helper functions getTable and getThisStruct encapsulate that lookup so
// the individual method implementations stay concise.
//
// # Error handling convention
//
// Functions that are declared to return a single error value (e.g. AddRow,
// Sort) return the same error as both the first and second return values:
//
//	return err, err
//
// Functions declared to return a data value plus an error (e.g. Get, GetRow)
// wrap the pair in a data.List:
//
//	return data.NewList(value, err), err
//
// This duplication is intentional: the Ego virtual machine uses the second
// return value to detect and propagate errors, while the first value is what
// gets assigned to the Ego variable on the left-hand side of a statement.
package tables

import (
	"github.com/tucats/ego/data"
)

// headingsFieldName is the name of the exported struct field that holds the
// column-heading array.  Ego scripts can read this field to inspect the schema
// of a table (e.g. t.Headings).
const headingsFieldName = "Headings"

// tableFieldName is the name of the unexported struct field that holds the
// native *app-cli/tables.Table pointer.  Ego scripts cannot access this field
// directly; all interaction goes through the methods defined below.
const tableFieldName = "table"

// TablesTableType is the Ego type descriptor for the Table object.  It
// declares the struct layout (two fields: Headings and table) and registers
// all public methods so the Ego compiler knows their signatures.  The
// FixSelfReferences call at the end resolves any recursive type references
// that arise because some method parameters reference OwnType (i.e. the
// Table type itself).
var TablesTableType = data.TypeDefinition("Table",
	data.StructureType().
		DefineField(headingsFieldName, data.ArrayType(data.StringType)).
		DefineField(tableFieldName, data.InterfaceType).
		DefineFunctions(map[string]data.Function{
			"AddColumn": {
				Declaration: &data.Declaration{
					Name: "AddColumn",
					Type: data.OwnType,
					Parameters: []data.Parameter{
						{
							Name: "heading",
							Type: data.StringType,
						},
					},
					Returns: []*data.Type{
						data.ErrorType,
					},
				},
				Value: addColumn,
			},
			"AddColumns": {
				Declaration: &data.Declaration{
					Name:     "AddColumns",
					Type:     data.OwnType,
					Variadic: true,
					Parameters: []data.Parameter{
						{
							Name: "headings",
							Type: data.StringType,
						},
					},
					Returns: []*data.Type{
						data.ErrorType,
					},
				},
				Value: addColumns,
			},
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
							Name: "height",
							Type: data.IntType,
						},
						{
							Name: "width",
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
).SetPackage("tables").FixSelfReferences()

// TablesPackage is the Ego package descriptor for "tables".  It exposes:
//   - tables.New(column …string) Table  — factory function for new tables
//   - tables.Table                      — the Table type itself (for type assertions)
//
// The package is registered with the Ego runtime during startup so that
// Ego scripts can write:
//
//	import "tables"
//	t := tables.New("Name", "Age")
var TablesPackage = data.NewPackageFromMap("tables", map[string]any{
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
