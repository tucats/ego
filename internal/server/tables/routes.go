package tables

import (
	"net/http"

	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/router"
	"github.com/tucats/ego/internal/server/tables/scripting"
	"github.com/tucats/ego/internal/util"
)

const (
	tableParameter = "{{table}}"
)

// AddStaticRoutes accepts an endpoint router, and adds to it the endpoint routes
// used by the Tables services.
func AddStaticRoutes(r *router.Router) {
	// Return compact schema metadata (table names + column name/type) for every
	// table in the DSN. Paging is supported via ?start= and ?limit=.
	// This route is registered at the DSN level (/dsns/{dsn}/@metadata), not
	// inside the tables/ sub-path, so it cannot conflict with any real table name.
	//
	// Not gated by Permissions() here, same reasoning as the tables-list route
	// above: DSNMetadataHandler opens the DSN via GetDatabase (enforcing
	// dsns.AuthDSN for a Restricted DSN) and listTableNamesForMetadata filters
	// table-by-table via Authorized() for a Secured DSN, so a non-admin caller
	// sees only what they actually have access to instead of being blocked
	// outright without dsn.admin.
	r.New(defs.DSNMetadataPath, DSNMetadataHandler, http.MethodGet).
		Authentication(true, false).
		Parameter(defs.StartParameterName, util.IntParameterType).
		Parameter(defs.LimitParameterName, util.IntParameterType).
		AcceptMedia(defs.DSNMetadataMediaType).
		Class(router.TableRequestCounter)

	// Use a server-configured AI endpoint to generate a SQL query for the
	// named DSN from a natural-language request. The request body may be
	// JSON (an array of strings) or plain text; both are always-accepted
	// content types, so no explicit ContentMedia() restriction is needed.
	r.New(defs.DSNGeneratePath, GenerateHandler, http.MethodPost).
		Authentication(true, false).
		Permissions(defs.DSNAdminPermission).
		AcceptMedia(defs.DSNGenerateMediaType).
		Class(router.TableRequestCounter)

	// Run a transaction script
	r.New(defs.TablesPath+"@transaction", scripting.Handler, http.MethodPost).
		Authentication(true, false).
		Permissions(defs.TableReadPermission, defs.TableUpdatePermission).
		Parameter(defs.FilterParameterName, defs.Any).
		AcceptMedia(defs.RowCountMediaType).
		Class(router.TableRequestCounter)

	// List all tables in a DSN. Not gated by Permissions() here: unlike the
	// other DSN-scoped routes, this one doesn't require a single blanket
	// permission. Instead ListTablesHandler opens the DSN via GetDatabase
	// (which enforces dsns.AuthDSN for a Restricted DSN) and getTableNames
	// filters the result table-by-table via Authorized() for a Secured DSN,
	// so a non-admin caller only ever sees the DSNs/tables they actually
	// have access to rather than being all-or-nothing gated on dsn.admin.
	r.New(defs.TablesPath, ListTablesHandler, http.MethodGet).
		Authentication(true, false).
		Parameter(defs.StartParameterName, "int").
		Parameter(defs.LimitParameterName, "int").
		Parameter(defs.UserParameterName, util.StringParameterType).
		Parameter(defs.RowCountParameterName, util.BoolParameterType).
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
		Parameter(defs.TransactionIDParameterName, util.StringParameterType).
		AcceptMedia(defs.TransactionMediaType).
		Class(router.TableRequestCounter)

	// Commit a transaction for a dsn
	r.New(defs.DSNCommitPath, CommitHandler, http.MethodGet).
		Authentication(true, false).
		Permissions(defs.TableReadPermission).
		Parameter(defs.TransactionIDParameterName, util.StringParameterType).
		AcceptMedia(defs.TransactionMediaType).
		Class(router.TableRequestCounter)

	// Read rows from a table via a DSN
	r.New(defs.TablesRowsPath, ReadRows, http.MethodGet).
		Authentication(true, false).
		Permissions(defs.TableReadPermission).
		Parameter(defs.StartParameterName, util.IntParameterType).
		Parameter(defs.LimitParameterName, util.IntParameterType).
		Parameter(defs.ColumnParameterName, util.ListParameterType).
		Parameter(defs.SortParameterName, util.ListParameterType).
		Parameter(defs.AbstractParameterName, util.BoolParameterType).
		Parameter(defs.FilterParameterName, defs.Any).
		Parameter(defs.UserParameterName, util.StringParameterType).
		Parameter(defs.TransactionIDParameterName, util.StringParameterType).
		AcceptMedia(defs.RowSetMediaType, defs.AbstractRowSetMediaType).
		Class(router.TableRequestCounter)

	// Insert rows into a table via a DSN
	r.New(defs.TablesRowsPath, InsertRows, http.MethodPut).
		Authentication(true, false).
		Permissions(defs.TableWritePermission).
		Parameter(defs.AbstractParameterName, util.BoolParameterType).
		Parameter(defs.UserParameterName, util.StringParameterType).
		Parameter(defs.UpsertParameterName, util.StringOrFlagParameterType).
		Parameter(defs.TransactionIDParameterName, util.StringParameterType).
		AcceptMedia(defs.RowSetMediaType, defs.AbstractRowSetMediaType).
		AcceptMedia(defs.RowSetMediaType, defs.AbstractRowSetMediaType).
		Class(router.TableRequestCounter)

	// Delete rows from a table via a DSN
	r.New(defs.TablesRowsPath, DeleteRows, http.MethodDelete).
		Authentication(true, false).
		Permissions(defs.TableDeletePermission).
		Parameter(defs.FilterParameterName, defs.Any).
		Parameter(defs.UserParameterName, util.StringParameterType).
		Parameter(defs.TransactionIDParameterName, util.StringParameterType).
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
		Parameter(defs.TransactionIDParameterName, util.StringParameterType).
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

	// Execute arbitrary SQL using the "@sql" pseudo-table-name. Note that
	// this was previous a PUT operation which is incorrect since it is not
	// idempotent. It is now a POST operation as of 2024-06-05. The old PUT
	// operation is still supported for backward compatibility.
	//
	// This no longer requires administrator status. An admin caller still
	// has unrestricted access, as before; a non-admin caller instead needs
	// defs.SQLPermission ("ego.sql"), and SQLTransaction itself (see sql.go
	// and sql_permissions.go) authorizes each table the parsed SQL touches
	// against that caller's table_perms grants and, for schema-changing
	// statements, defs.DSNAdminPermission.
	r.New(defs.TablesPath+defs.SQLPseudoTable, SQLTransaction, http.MethodPost).
		Authentication(true, false).
		Permissions(defs.SQLPermission).
		Class(router.TableRequestCounter)

	// This is the deprecated old interface, which will be retired in
	// Ego 1.11.
	r.New(defs.TablesPath+defs.SQLPseudoTable, SQLTransaction, http.MethodPut).
		Authentication(true, false).
		Permissions(defs.SQLPermission).
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
