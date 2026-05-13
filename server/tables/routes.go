package tables

import (
	"net/http"

	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/router"
	"github.com/tucats/ego/server/tables/scripting"
	"github.com/tucats/ego/util"
)

const (
	tableParameter = "{{table}}"
)

// AddStaticRoutes accepts an endpoint router, and adds to it the endpoint routes
// used by the Tables services.
func AddStaticRoutes(r *router.Router) {
	// Run a transaction script
	r.New(defs.TablesPath+"@transaction", scripting.Handler, http.MethodPost).
		Authentication(true, false).
		Permissions(defs.TableReadPermission, defs.TableUpdatePermission).
		Parameter(defs.FilterParameterName, defs.Any).
		AcceptMedia(defs.RowCountMediaType).
		Class(router.TableRequestCounter)

	// List all tables in a DSN
	r.New(defs.TablesPath, ListTablesHandler, http.MethodGet).
		Authentication(true, false).
		Permissions(defs.DSNAdminPermission).
		Parameter(defs.StartParameterName, "int").
		Parameter(defs.LimitParameterName, "int").
		Parameter(defs.UserParameterName, "string").
		Parameter(defs.RowCountParameterName, "bool").
		AcceptMedia(defs.TablesMediaType).
		Class(router.TableRequestCounter)

	// Start a transaction for a dsn
	r.New(defs.DSNBeginPath, BeginHandler, http.MethodGet).
		Authentication(true, false).
		Permissions(defs.TableReadPermission).
		Parameter(defs.ExpiresParameterName, util.DurationParameterType).
		AcceptMedia(defs.TransactionMediaType).
		Class(router.TableRequestCounter)

	// Rollback a transaction for a dsn
	r.New(defs.DSNRollbackPath, RollbackHandler, http.MethodGet).
		Authentication(true, false).
		Permissions(defs.TableReadPermission).
		Parameter(defs.TransactionIDParameterName, "string").
		AcceptMedia(defs.TransactionMediaType).
		Class(router.TableRequestCounter)

	// Commit a transaction for a dsn
	r.New(defs.DSNCommitPath, CommitHandler, http.MethodGet).
		Authentication(true, false).
		Permissions(defs.TableReadPermission).
		Parameter(defs.TransactionIDParameterName, "string").
		AcceptMedia(defs.TransactionMediaType).
		Class(router.TableRequestCounter)

	// Read rows from a table via a DSN
	r.New(defs.TablesRowsPath, ReadRows, http.MethodGet).
		Authentication(true, false).
		Permissions(defs.TableReadPermission).
		Parameter(defs.StartParameterName, util.IntParameterType).
		Parameter(defs.LimitParameterName, util.IntParameterType).
		Parameter(defs.ColumnParameterName, "list").
		Parameter(defs.SortParameterName, "list").
		Parameter(defs.AbstractParameterName, util.BoolParameterType).
		Parameter(defs.FilterParameterName, defs.Any).
		Parameter(defs.UserParameterName, util.StringParameterType).
		Parameter(defs.TransactionIDParameterName, "string").
		AcceptMedia(defs.RowSetMediaType, defs.AbstractRowSetMediaType).
		Class(router.TableRequestCounter)

	// Insert rows into a table via a DSN
	r.New(defs.TablesRowsPath, InsertRows, http.MethodPut).
		Authentication(true, false).
		Permissions(defs.TableWritePermission).
		Parameter(defs.AbstractParameterName, util.BoolParameterType).
		Parameter(defs.UserParameterName, util.StringParameterType).
		Parameter(defs.UpsertParameterName, util.StringOrFlagParameterType).
		Parameter(defs.TransactionIDParameterName, "string").
		AcceptMedia(defs.RowSetMediaType, defs.AbstractRowSetMediaType).
		AcceptMedia(defs.RowSetMediaType, defs.AbstractRowSetMediaType).
		Class(router.TableRequestCounter)

	// Delete rows from a table via a DSN
	r.New(defs.TablesRowsPath, DeleteRows, http.MethodDelete).
		Authentication(true, false).
		Permissions(defs.TableDeletePermission).
		Parameter(defs.FilterParameterName, defs.Any).
		Parameter(defs.UserParameterName, util.StringParameterType).
		Parameter(defs.TransactionIDParameterName, "string").
		AcceptMedia(defs.RowCountMediaType).
		Class(router.TableRequestCounter)

	// Update rows from a table via a DSN
	r.New(defs.TablesRowsPath, UpdateRows, http.MethodPatch).
		Authentication(true, false).
		Permissions(defs.TableUpdatePermission).
		Parameter(defs.FilterParameterName, defs.Any).
		Parameter(defs.UserParameterName, util.StringParameterType).
		Parameter(defs.ColumnParameterName, util.StringParameterType).
		Parameter(defs.AbstractParameterName, util.StringParameterType).
		Parameter(defs.TransactionIDParameterName, "string").
		AcceptMedia(defs.RowCountMediaType).
		Class(router.TableRequestCounter)

	// Read permissions for a table via a DSN
	r.New(defs.TablesPath+tableParameter+"/permissions", ReadPermissions, http.MethodGet).
		Authentication(true, false).
		Permissions(defs.TableAdminPermission).
		Parameter(defs.UserParameterName, util.StringParameterType).
		Class(router.TableRequestCounter)

	// Grant permissions for a table
	r.New(defs.TablesPath+"{{table}}/permissions", GrantPermissions, http.MethodPut).
		Authentication(true, false).
		Permissions(defs.TableAdminPermission).
		Parameter(defs.UserParameterName, util.StringParameterType).
		Class(router.TableRequestCounter)

	// Revoke permissions from a table
	r.New(defs.TablesPath+"{{table}}/permissions", DeletePermissions, http.MethodDelete).
		Authentication(true, false).
		Permissions(defs.TableAdminPermission).
		Parameter(defs.UserParameterName, util.StringParameterType).
		Class(router.TableRequestCounter)

	// Get metadata for a table via DSNS
	r.New(defs.TablesPath+tableParameter, ReadTable, http.MethodGet).
		Authentication(true, false).
		Parameter(defs.UserParameterName, util.StringParameterType).
		Parameter(defs.RowIDs, util.BoolParameterType).
		AcceptMedia(defs.TableMetadataMediaType).
		Class(router.TableRequestCounter)

	// Read all permissions data using the "@permissions" pseudo-table-name.
	r.New(defs.TablesPath+defs.PermissionsPseudoTable, ReadAllPermissions, http.MethodGet).
		Authentication(true, true).
		Parameter(defs.UserParameterName, util.StringParameterType).
		Class(router.TableRequestCounter)

	// Execute arbitrary SQL using the "@sql" pseudo-table-name.
	r.New(defs.TablesPath+sqlPseudoTable, SQLTransaction, http.MethodPut).
		Authentication(true, true).
		Class(router.TableRequestCounter)

	// Create a new table using a DSN
	r.New(defs.TablesPath+tableParameter, TableCreate, http.MethodPut).
		Authentication(true, false).
		Permissions(defs.TableUpdatePermission).
		AcceptMedia(defs.SQLStatementsMediaType, defs.RowSetMediaType, defs.RowCountMediaType).
		Class(router.TableRequestCounter)

	// Delete a table using a DSN
	r.New(defs.TablesPath+tableParameter, DeleteTable, http.MethodDelete).
		Authentication(true, false).
		Permissions(defs.TableDeletePermission).
		Class(router.TableRequestCounter)
}
