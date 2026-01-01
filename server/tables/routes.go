package tables

import (
	"net/http"

	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/server/server"
	"github.com/tucats/ego/server/tables/scripting"
	"github.com/tucats/ego/util"
)

const (
	tableParameter = "{{table}}"
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

	// List all tables in a DSN
	router.New(defs.TablesPath, ListTablesHandler, http.MethodGet).
		Authentication(true, false).
		Permissions("dsn_read").
		Parameter(defs.StartParameterName, "int").
		Parameter(defs.LimitParameterName, "int").
		Parameter(defs.UserParameterName, "string").
		Parameter(defs.RowCountParameterName, "bool").
		AcceptMedia(defs.TablesMediaType).
		Class(server.TableRequestCounter)

	// Read rows from a table via a DSN
	router.New(defs.TablesRowsPath, ReadRows, http.MethodGet).
		Authentication(true, false).
		Permissions("table_read").
		Parameter(defs.StartParameterName, util.IntParameterType).
		Parameter(defs.LimitParameterName, util.IntParameterType).
		Parameter(defs.ColumnParameterName, "list").
		Parameter(defs.SortParameterName, "list").
		Parameter(defs.AbstractParameterName, util.BoolParameterType).
		Parameter(defs.FilterParameterName, defs.Any).
		Parameter(defs.UserParameterName, util.StringParameterType).
		AcceptMedia(defs.RowSetMediaType, defs.AbstractRowSetMediaType).
		Class(server.TableRequestCounter)

	// Insert rows into a table via a DSN
	router.New(defs.TablesRowsPath, InsertRows, http.MethodPut).
		Authentication(true, false).
		Permissions("table_modify").
		Parameter(defs.AbstractParameterName, util.BoolParameterType).
		Parameter(defs.UserParameterName, util.StringParameterType).
		Parameter(defs.UpsertParameterName, util.StringOrFlagParameterType).
		AcceptMedia(defs.RowSetMediaType, defs.AbstractRowSetMediaType).
		AcceptMedia(defs.RowSetMediaType, defs.AbstractRowSetMediaType).
		Class(server.TableRequestCounter)

	// Delete rows from a table via a DSN
	router.New(defs.TablesRowsPath, DeleteRows, http.MethodDelete).
		Authentication(true, false).
		Permissions("table_modify").
		Parameter(defs.FilterParameterName, defs.Any).
		Parameter(defs.UserParameterName, util.StringParameterType).
		AcceptMedia(defs.RowCountMediaType).
		Class(server.TableRequestCounter)

	// Update rows from a table via a DSN
	router.New(defs.TablesRowsPath, UpdateRows, http.MethodPatch).
		Authentication(true, false).
		Permissions("table_modify").
		Parameter(defs.FilterParameterName, defs.Any).
		Parameter(defs.UserParameterName, util.StringParameterType).
		Parameter(defs.ColumnParameterName, util.StringParameterType).
		Parameter(defs.AbstractParameterName, util.StringParameterType).
		AcceptMedia(defs.RowCountMediaType).
		Class(server.TableRequestCounter)

	// Read permissions for a table via a DSN
	router.New(defs.TablesPath+tableParameter+"/permissions", ReadPermissions, http.MethodGet).
		Authentication(true, false).
		Permissions("table_admin").
		Parameter(defs.UserParameterName, util.StringParameterType).
		Class(server.TableRequestCounter)

	// Grant permissions for a table
	router.New(defs.TablesPath+"{{table}}/permissions", GrantPermissions, http.MethodPut).
		Authentication(true, false).
		Permissions("table_admin").
		Parameter(defs.UserParameterName, util.StringParameterType).
		Class(server.TableRequestCounter)

	// Revoke permissions from a table
	router.New(defs.TablesPath+"{{table}}/permissions", DeletePermissions, http.MethodDelete).
		Authentication(true, false).
		Permissions("table_admin").
		Parameter(defs.UserParameterName, util.StringParameterType).
		Class(server.TableRequestCounter)

	// Get metadata for a table via DSNS
	router.New(defs.TablesPath+tableParameter, ReadTable, http.MethodGet).
		Authentication(true, false).
		Parameter(defs.UserParameterName, util.StringParameterType).
		AcceptMedia(defs.TableMetadataMediaType).
		Class(server.TableRequestCounter)

	// Read all permissions data using the "@permissions" pseudo-table-name.
	router.New(defs.TablesPath+defs.PermissionsPseudoTable, ReadAllPermissions, http.MethodGet).
		Authentication(true, true).
		Parameter(defs.UserParameterName, util.StringParameterType).
		Class(server.TableRequestCounter)

	// Execute arbitrary SQL using the "@sql" pseudo-table-name.
	router.New(defs.TablesPath+sqlPseudoTable, SQLTransaction, http.MethodPut).
		Authentication(true, true).
		Class(server.TableRequestCounter)

	// Create a new table using a DSN
	router.New(defs.TablesPath+tableParameter, TableCreate, http.MethodPut).
		Authentication(true, false).
		Permissions("table_modify").
		AcceptMedia(defs.SQLStatementsMediaType, defs.RowSetMediaType, defs.RowCountMediaType).
		Class(server.TableRequestCounter)

	// Delete a table using a DSN
	router.New(defs.TablesPath+tableParameter, DeleteTable, http.MethodDelete).
		Authentication(true, false).
		Permissions("table_modify").
		Class(server.TableRequestCounter)
}
