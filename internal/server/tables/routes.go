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
		Authentication(true).
		Parameter(defs.StartParameterName, util.IntParameterType).
		Parameter(defs.LimitParameterName, util.IntParameterType).
		AcceptMedia(defs.DSNMetadataMediaType).
		Class(router.TableRequestCounter)

	// Use a server-configured AI endpoint to generate a SQL query for the
	// named DSN from a natural-language request. The request body may be
	// JSON (an array of strings) or plain text; both are always-accepted
	// content types, so no explicit ContentMedia() restriction is needed.
	r.New(defs.DSNGeneratePath, GenerateHandler, http.MethodPost).
		Permissions(defs.SQLPermission).
		AcceptMedia(defs.DSNGenerateMediaType).
		Class(router.TableRequestCounter)

	// Run a transaction script
	r.New(defs.TablesPath+"@transaction", scripting.Handler, http.MethodPost).
		Authentication(true).
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
		Authentication(true).
		Parameter(defs.StartParameterName, "int").
		Parameter(defs.LimitParameterName, "int").
		Parameter(defs.UserParameterName, util.StringParameterType).
		Parameter(defs.RowCountParameterName, util.BoolParameterType).
		AcceptMedia(defs.TablesMediaType).
		Class(router.TableRequestCounter)

	// Start a transaction for a dsn
	r.New(defs.DSNBeginPath, BeginHandler, http.MethodGet).
		Authentication(true).
		Parameter(defs.ExpiresParameterName, util.DurationParameterType).
		AcceptMedia(defs.TransactionMediaType).
		Class(router.TableRequestCounter)

	// Rollback a transaction for a dsn
	r.New(defs.DSNRollbackPath, RollbackHandler, http.MethodGet).
		Authentication(true).
		Parameter(defs.TransactionIDParameterName, util.StringParameterType).
		AcceptMedia(defs.TransactionMediaType).
		Class(router.TableRequestCounter)

	// Keep a transaction for a dsn alive so it doesn't time out.
	r.New(defs.DSNKeepAlivePath, KeepaliveHandler, http.MethodGet).
		Authentication(true).
		Parameter(defs.TransactionIDParameterName, util.StringParameterType).
		AcceptMedia(defs.KeepaliveMediaType).
		Class(router.TableRequestCounter)

	// Commit a transaction for a dsn
	r.New(defs.DSNCommitPath, CommitHandler, http.MethodGet).
		Permissions(defs.TableReadPermission).
		Parameter(defs.TransactionIDParameterName, util.StringParameterType).
		AcceptMedia(defs.TransactionMediaType).
		Class(router.TableRequestCounter)

	// Read rows from a table via a DSN
	r.New(defs.TablesRowsPath, ReadRows, http.MethodGet).
		Authentication(true).
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
		Authentication(true).
		Parameter(defs.AbstractParameterName, util.BoolParameterType).
		Parameter(defs.UserParameterName, util.StringParameterType).
		Parameter(defs.UpsertParameterName, util.StringOrFlagParameterType).
		Parameter(defs.TransactionIDParameterName, util.StringParameterType).
		AcceptMedia(defs.RowSetMediaType, defs.AbstractRowSetMediaType).
		AcceptMedia(defs.RowSetMediaType, defs.AbstractRowSetMediaType).
		Class(router.TableRequestCounter)

	// Delete rows from a table via a DSN
	r.New(defs.TablesRowsPath, DeleteRows, http.MethodDelete).
		Authentication(true).
		Parameter(defs.FilterParameterName, defs.Any).
		Parameter(defs.UserParameterName, util.StringParameterType).
		Parameter(defs.TransactionIDParameterName, util.StringParameterType).
		AcceptMedia(defs.RowCountMediaType).
		Class(router.TableRequestCounter)

	// Update rows from a table via a DSN
	r.New(defs.TablesRowsPath, UpdateRows, http.MethodPatch).
		Authentication(true).
		Parameter(defs.FilterParameterName, defs.Any).
		Parameter(defs.UserParameterName, util.StringParameterType).
		Parameter(defs.ColumnParameterName, util.StringParameterType).
		Parameter(defs.AbstractParameterName, util.StringParameterType).
		Parameter(defs.TransactionIDParameterName, util.StringParameterType).
		AcceptMedia(defs.RowCountMediaType).
		Class(router.TableRequestCounter)

	// Read permissions for a table via a DSN. Not gated by Permissions()
	// here (DATA-SECURITY-2.md finding #4): as with TableCreate/DeleteTable
	// above, Permissions(defs.DSNAdminPermission) only ever checks the
	// caller's *identity-wide* permissions, which made ego.table.admin --
	// documented in docs/SERVER.md as existing precisely so a table's own
	// admin can manage its permissions -- useless for that purpose, since a
	// table.admin holder with no identity-wide ego.dsn.admin was rejected
	// before ReadPermissions' handler body ever ran. The four routes below
	// now all call security.go's authorizedForTablePermissions themselves,
	// which accepts session.Admin, identity-wide ego.dsn.admin, a
	// DSN-specific dsns_auth admin grant, OR a table-specific table_perms
	// admin grant -- see that function's doc comment for the full OR-chain.
	// Still requires plain authentication via Authentication(true).
	r.New(defs.TablesPath+tableParameter+"/permissions", ReadPermissions, http.MethodGet).
		Authentication(true).
		Parameter(defs.UserParameterName, util.StringParameterType).
		Class(router.TableRequestCounter)

	// List every user's permissions on a table via a DSN. Not gated by
	// Permissions() for the identical reason as ReadPermissions just above
	// (DATA-SECURITY-2.md finding #4).
	r.New(defs.TablesNameAllPermissionsPath, ReadTablePermissions, http.MethodGet).
		Authentication(true).
		Class(router.TableRequestCounter)

	// Grant permissions for a table. Not gated by Permissions() for the
	// identical reason as ReadPermissions above (DATA-SECURITY-2.md
	// finding #4).
	r.New(defs.TablesPath+"{{table}}/permissions", GrantPermissions, http.MethodPut).
		Authentication(true).
		Parameter(defs.UserParameterName, util.StringParameterType).
		Class(router.TableRequestCounter)

	// Revoke permissions from a table. Not gated by Permissions() for the
	// identical reason as ReadPermissions above (DATA-SECURITY-2.md
	// finding #4).
	r.New(defs.TablesPath+"{{table}}/permissions", DeletePermissions, http.MethodDelete).
		Authentication(true).
		Parameter(defs.UserParameterName, util.StringParameterType).
		Class(router.TableRequestCounter)

	// Get metadata for a table via DSNS
	r.New(defs.TablesPath+tableParameter, DescribeTable, http.MethodGet).
		Authentication(true).
		Parameter(defs.UserParameterName, util.StringParameterType).
		Parameter(defs.RowIDs, util.BoolParameterType).
		AcceptMedia(defs.TableMetadataMediaType).
		Class(router.TableRequestCounter)

	// Read all permissions data using the "@permissions" pseudo-table-name.
	// This dumps every permission for every user across every DSN/table in
	// one shot, with no per-resource scoping the way the other permission
	// routes have -- genuinely root-only by design, so the admin-only
	// intent formerly expressed via Authentication(true, true) is now
	// expressed as an explicit Permissions(defs.RootPermission).
	r.New(defs.TablesPath+defs.PermissionsPseudoTable, ReadAllPermissions, http.MethodGet).
		Permissions(defs.RootPermission).
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
		Permissions(defs.SQLPermission).
		Class(router.TableRequestCounter)

	// This is the deprecated old interface, which will be retired in
	// Ego 1.11.
	r.New(defs.TablesPath+defs.SQLPseudoTable, SQLTransaction, http.MethodPut).
		Permissions(defs.SQLPermission).
		Class(router.TableRequestCounter)

	// Format SQL text using the "@format" pseudo-table-name, without
	// executing it. Same permission as @sql (defs.SQLPermission) since it
	// parses client-supplied SQL the same way, but FormatSQL never opens a
	// transaction or touches a row, so there is no additional table_perms/
	// DSNAdminPermission check the way @sql itself enforces (see FormatSQL's
	// doc comment in format.go).
	r.New(defs.TablesPath+defs.SQLFormatPseudoTable, FormatSQL, http.MethodPost).
		Permissions(defs.SQLPermission).
		Class(router.TableRequestCounter)

	// Create a new table using a DSN. Not gated by Permissions() here
	// (DATA-SECURITY-2.md finding #3): Permissions(defs.DSNAdminPermission)
	// only ever checks a caller's *identity-wide* permissions (see
	// router/serve.go's requiredPermissions loop) -- it has no notion of "is
	// this caller an admin of just this one DSN". A caller who was granted
	// DSN-specific admin (for example, the DSN's own creator -- see
	// CreateDSNHandler's self-grant in internal/server/dsns/handler.go) but
	// holds no identity-wide ego.dsn.admin would be rejected by that route
	// gate before TableCreate's handler body ever ran, even though they are
	// exactly the kind of caller "whoever creates a table is automatically
	// granted all five actions on it" (docs/SERVER.md) is describing.
	//
	// TableCreate's own GetDatabase(session, dsnName, dsns.DSNAdminAction)
	// call, a few lines into the handler, already performs the correct
	// check: identity-wide ego.dsn.admin, OR a DSN-specific dsns_auth admin
	// record for this DSN, OR (if the DSN is unrestricted) no check at all
	// -- the same identity-OR-per-DSN pattern already used by
	// DeleteDSNHandler and friends. Relying on that one check, instead of
	// duplicating a second, different one at the route level, is what fixes
	// this gap. Still requires plain authentication via Authentication(true)
	// -- an anonymous caller is still rejected before reaching the handler.
	r.New(defs.TablesPath+tableParameter, TableCreate, http.MethodPut).
		Authentication(true).
		AcceptMedia(defs.SQLStatementsMediaType, defs.RowSetMediaType, defs.RowCountMediaType).
		Class(router.TableRequestCounter)

	// Delete a table using a DSN. Not gated by Permissions() here for the
	// identical reason as TableCreate just above (DATA-SECURITY-2.md
	// finding #3) -- DeleteTable's own GetDatabase(..., dsns.DSNAdminAction)
	// call already performs the correct identity-OR-per-DSN-admin check.
	r.New(defs.TablesPath+tableParameter, DeleteTable, http.MethodDelete).
		Authentication(true).
		Class(router.TableRequestCounter)
}
