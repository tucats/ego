package tables

import (
	"net/http"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/http/server"
	"github.com/tucats/ego/http/tables/scripting"
)

// AddStaticRoutes accepts an endpoint router, and adds to it the endpoint routes
// used by the Tables services.
func AddStaticRoutes(router *server.Router) {
	// Run a transaction script
	router.New(defs.TablesPath+"@transaction", scripting.Handler, http.MethodPost).
		Authentication(true, false).
		Permissions("table_read", "table_modify").
		Parameter(defs.FilterParameterName, defs.Any).
		AcceptMedia(defs.RowCountMediaType).
		Class(server.TableRequestCounter)

	// List all tables.
	router.New(defs.TablesPath, ListTablesHandler, http.MethodGet).
		Authentication(true, false).
		Permissions("table_read").
		Parameter(defs.StartParameterName, "int").
		Parameter(defs.LimitParameterName, "int").
		Parameter(defs.UserParameterName, "string").
		Parameter(defs.RowCountParameterName, "bool").
		AcceptMedia(defs.TablesMediaType).
		Class(server.TableRequestCounter)

	// Read rows from a table.
	router.New(defs.TablesRowsPath, ReadRows, http.MethodGet).
		Authentication(true, false).
		Permissions("table_read").
		Parameter(defs.StartParameterName, data.IntTypeName).
		Parameter(defs.LimitParameterName, data.IntTypeName).
		Parameter(defs.ColumnParameterName, "list").
		Parameter(defs.SortParameterName, "list").
		Parameter(defs.AbstractParameterName, data.BoolTypeName).
		Parameter(defs.FilterParameterName, defs.Any).
		Parameter(defs.UserParameterName, data.StringTypeName).
		AcceptMedia(defs.RowSetMediaType, defs.AbstractRowSetMediaType).
		Class(server.TableRequestCounter)

	// Insert rows into a table.
	router.New(defs.TablesRowsPath, InsertRows, http.MethodPut).
		Authentication(true, false).
		Permissions("table_modify").
		Parameter(defs.AbstractParameterName, data.BoolTypeName).
		Parameter(defs.UserParameterName, data.StringTypeName).
		AcceptMedia(defs.RowSetMediaType, defs.AbstractRowSetMediaType).
		Class(server.TableRequestCounter)

	// Delete rows from a table.
	router.New(defs.TablesRowsPath, DeleteRows, http.MethodDelete).
		Authentication(true, false).
		Permissions("table_modify").
		Parameter(defs.FilterParameterName, defs.Any).
		Parameter(defs.UserParameterName, data.StringTypeName).
		AcceptMedia(defs.RowCountMediaType).
		Class(server.TableRequestCounter)

	// Update rows from a table.
	router.New(defs.TablesRowsPath, UpdateRows, http.MethodPatch).
		Authentication(true, false).
		Permissions("table_modify").
		Parameter(defs.FilterParameterName, defs.Any).
		Parameter(defs.UserParameterName, data.StringTypeName).
		Parameter(defs.ColumnParameterName, data.StringTypeName).
		Parameter(defs.AbstractParameterName, data.BoolTypeName).
		AcceptMedia(defs.RowCountMediaType).
		Class(server.TableRequestCounter)

	// Read permissions for a table
	router.New(defs.TablesPath+"{{table}}/permissions", ReadPermissions, http.MethodGet).
		Authentication(true, false).
		Permissions("table_admin").
		Parameter(defs.UserParameterName, data.StringTypeName).
		Class(server.TableRequestCounter)

	// Grant permissions for a table
	router.New(defs.TablesPath+"{{table}}/permissions", ReadPermissions, http.MethodPut).
		Authentication(true, false).
		Permissions("table_admin").
		Parameter(defs.UserParameterName, data.StringTypeName).
		Class(server.TableRequestCounter)

	// Revoke permissions from a table
	router.New(defs.TablesPath+"{{table}}/permissions", DeletePermissions, http.MethodDelete).
		Authentication(true, false).
		Permissions("table_admin").
		Parameter(defs.UserParameterName, data.StringTypeName).
		Class(server.TableRequestCounter)

	// Get metadata for a table
	router.New(defs.TablesPath+"{{table}}", ReadTable, http.MethodGet).
		Authentication(true, false).
		Parameter(defs.UserParameterName, data.StringTypeName).
		AcceptMedia(defs.TableMetadataMediaType).
		Class(server.TableRequestCounter)

	// Read all permissions data using the "@permissions" pseudo-table-name.
	router.New(defs.TablesPath+"@permissions", ReadAllPermissions, http.MethodGet).
		Authentication(true, true).
		Parameter(defs.UserParameterName, data.StringTypeName).
		Class(server.TableRequestCounter)

	// Execute arbitrary SQL using the "@sql" pseudo-table-name.
	router.New(defs.TablesPath+sqlPseudoTable, SQLTransaction, http.MethodPut).
		Authentication(true, true).
		Class(server.TableRequestCounter)

	// Create a new table
	router.New(defs.TablesPath+"{{table}}", TableCreate, http.MethodPut).
		Authentication(true, false).
		Permissions("table_update").
		AcceptMedia(defs.SQLStatementsMediaType, defs.RowSetMediaType, defs.RowCountMediaType).
		Class(server.TableRequestCounter)

	// Delete a table
	router.New(defs.TablesPath+"{{table}}", DeleteTable, http.MethodDelete).
		Authentication(true, false).
		Permissions("table_update").
		Class(server.TableRequestCounter)
}
