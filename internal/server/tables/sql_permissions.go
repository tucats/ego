package tables

import (
	"net/http"
	"strings"

	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/dsns"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/i18n"
	"github.com/tucats/ego/internal/router"
	"github.com/tucats/ego/internal/server/auth"
	"github.com/tucats/ego/internal/server/tables/database"
	"github.com/tucats/ego/internal/sqlparse"
	"github.com/tucats/ego/internal/util"
)

// This file authorizes and reformats the SQL text the @sql endpoint
// (SQLTransaction, in sql.go) receives from a client, using the sqlparse
// package (github.com/tucats/ego/internal/sqlparse) to get a structural view
// of each statement -- which tables it touches and how -- without having to
// hand the raw text to the underlying database driver first.
//
// Before this existed, @sql was restricted to administrators (routes.go used
// to register it with Authentication(true, true)) because there was no way
// to tell what tables a piece of client-supplied SQL would touch, or whether
// it read or wrote them, short of executing it. A non-admin caller can now
// reach @sql at all if granted the new defs.SQLPermission ("ego.sql"); what
// they are then allowed to do with it is governed the same way any other
// table access is: table_perms read/write grants (security.go's Authorized)
// for DML, and defs.DSNAdminPermission for DDL that changes the DSN's
// schema.

// sqlDialect maps a database.Database's Provider field (set by
// database.Open to e.g. defs.SqliteProvider or defs.PostgresProvider) to the
// sqlparse dialect constant used to parse and format @sql statements run
// against it.
func sqlDialect(provider string) int {
	if provider == defs.PostgresProvider {
		return sqlparse.PostgreSQL
	}

	return sqlparse.SQLite
}

// baseTableName strips a schema qualifier ("schema.table" -> "table") from a
// table name as reported by sqlparse's Tables(). table_perms records (see
// PermissionsObject above in security.go) are keyed by a bare table name
// within a DSN -- there is no separate schema column -- so a
// schema-qualified reference in the SQL text is checked against the same
// table_perms entry as an unqualified one would be.
func baseTableName(name string) string {
	if i := strings.LastIndex(name, "."); i >= 0 {
		return name[i+1:]
	}

	return name
}

// hasPermission reports whether session's user has been granted permission,
// either as the server administrator (session.Admin, which this checks
// first so a caller never needs to special-case it separately) or as an
// ordinary grant. It prefers session.Permissions when the router has
// already resolved it for this request (see the "hasResolvedPermissions"
// comment in router/serve.go) and falls back to a direct auth.GetPermission
// lookup otherwise, mirroring serve.go's own two-path check for the same
// reason: a federated (JWT/OAuth) session's permissions live only on
// session.Permissions, with no local user record for auth.GetPermission to
// find.
func hasPermission(session *router.Session, permission string) bool {
	if session.Admin {
		return true
	}

	if len(session.Permissions) > 0 {
		return session.HasAllPermissions(permission)
	}

	return auth.GetPermission(session.ID, session.User, permission)
}

// authorizeAndFormatStatements parses each of the raw SQL statements the
// client sent to @sql, and returns them rewritten by sqlparse's Format() --
// which upper-cases keywords and, importantly, quotes identifiers the way
// the target dialect requires to preserve the case the client wrote them in
// (see Format's doc comment in sqlparse's format.go) -- alongside each
// statement's primary verb (its sqlparse.StatementKind), which the caller
// uses to enforce the "a SELECT must be the last statement" rule (see
// SQLTransaction in sql.go) without re-deriving it from the text.
//
// For an admin caller (session.Admin), that is all this function does:
// admins retain the unrestricted access @sql always gave them, so a
// statement that fails to parse -- this parser is necessarily a subset of
// whatever the backend actually accepts, see the "Syntax only" design goal
// in sqlparse/ast/node.go -- is passed through as-is, unformatted, with an
// unknown StatementKind, rather than rejected, and no table permissions are
// checked.
//
// For a non-admin caller (only reachable at all with defs.SQLPermission;
// see routes.go), every statement must parse, and every table it
// references (sqlparse's Tables()) must be one the caller is allowed to use
// that way: UsageRead requires defs.TableReadPermission, UsageWrite requires
// whichever of defs.TableWritePermission/TableUpdatePermission/
// TableDeletePermission matches the statement's own kind (see
// writePermissionForKind below -- sqlparse's own UsageMode only has three
// values, read/write/admin, but the statement's separate StatementKind
// already distinguishes INSERT from UPDATE from DELETE, which is all this
// function needs to preserve table_perms' full five-way granularity here
// too), and UsageAdmin (CREATE/ALTER/DROP TABLE, CREATE/DROP INDEX,
// CREATE/DROP VIEW) requires defs.DSNAdminPermission instead of a
// table_perms grant, since altering the DSN's schema is a DSN-wide
// capability rather than something a per-table grant is meant to cover.
//
// On any failure, an error response has already been written to w, and the
// returned status is greater than http.StatusOK; the caller must stop and
// return that status without executing any statement.
func authorizeAndFormatStatements(session *router.Session, db *database.Database, statements []string, w http.ResponseWriter) ([]string, []sqlparse.StatementKind, int) {
	dialect := sqlDialect(db.Provider)
	formatted := make([]string, len(statements))
	kinds := make([]sqlparse.StatementKind, len(statements))

	for i, stmt := range statements {
		p, err := sqlparse.New(stmt, dialect)
		if err != nil {
			if session.Admin {
				formatted[i] = stmt

				continue
			}

			return nil, nil, util.ErrorResponse(w, session.ID, errors.Localize(err, session.Language), http.StatusBadRequest)
		}

		kinds[i] = p.StatementKind()

		// PostgreSQL: pin every table/view/index reference the client left
		// unqualified to the DSN's own resolved schema (db.User -- see
		// database.Open's doc comment) rather than letting the server
		// resolve it through whatever default schema the connection
		// happens to have (e.g. search_path). A reference the client
		// qualified itself (schema.table) is left untouched. This mirrors
		// what parsing.FullName already does for every structured /rows
		// and /tables endpoint; raw @sql text previously had no equivalent
		// and so could depend on database-side schema resolution. SQLite
		// has no schema concept, so this is skipped there.
		if db.Provider == defs.PostgresProvider {
			p.QualifyTables(db.User)
		}

		formatted[i] = p.Format()

		if session.Admin {
			continue
		}

		if status := authorizeStatement(session, w, db.DSN, p); status > http.StatusOK {
			return nil, nil, status
		}
	}

	return formatted, kinds, http.StatusOK
}

// isSchemaAlteringKind reports whether kind is a DDL statement that can
// invalidate cached table schema metadata (internal/caches, caches.SchemaCache)
// -- CREATE/ALTER/DROP TABLE, CREATE/DROP INDEX, or CREATE/DROP VIEW. Mirrors
// scripting/authz.go's function of the same name (see that copy's doc
// comment for why it is a local copy rather than exported and imported: the
// two packages cannot import each other without a cycle, tables already
// importing scripting to register the @transaction route). Replaces this
// file's own former use of parsing.IsSchemaAlteringStatement, a text-prefix
// heuristic that only ever recognized ALTER TABLE and DROP TABLE -- it
// missed CREATE INDEX/DROP INDEX (and CREATE/DROP VIEW) entirely, which
// matters now that the schema cache also holds sql_pkey.go's unique-index
// lookups.
func isSchemaAlteringKind(kind sqlparse.StatementKind) bool {
	switch kind {
	case sqlparse.StmtCreateTable, sqlparse.StmtDropTable, sqlparse.StmtAlterTable,
		sqlparse.StmtCreateIndex, sqlparse.StmtDropIndex,
		sqlparse.StmtCreateView, sqlparse.StmtDropView:
		return true
	default:
		return false
	}
}

// writePermissionForKind reports which specific ego.table.* permission a
// sqlparse.UsageWrite table reference actually requires, given the kind of
// the statement it came from.
//
// DATA-SECURITY-2.md finding #5: sqlparse.Tables() (sqlparse/analyze.go)
// only ever tags a table UsageWrite for an INSERT, UPDATE, or DELETE
// statement -- see that function's own doc comment, and its "write" helper,
// which is called from exactly those three switch cases and nowhere else.
// Within any one statement there is only one StatementKind (sqlparse's own
// doc comment on that type: "There is exactly one StatementKind per
// concrete ast.Statement type"), so every UsageWrite reference in a given
// statement maps to the same one of the three permissions below -- this
// function does not need to look at anything about the specific table
// reference, only the statement kind the caller already has in hand.
//
// Go note for readers new to the language: a "switch" with no expression
// after the word "switch" (as authorizeStatement's outer switch below has,
// switching on t.Usage) picks whichever "case" matches first, top to
// bottom; this one switches on an explicit value (kind) instead, which
// works the same way -- try each case in order, run the first one that
// matches. "default" catches anything not explicitly listed.
func writePermissionForKind(kind sqlparse.StatementKind) string {
	switch kind {
	case sqlparse.StmtUpdate:
		return defs.TableUpdatePermission
	case sqlparse.StmtDelete:
		return defs.TableDeletePermission
	default:
		// StmtInsert is the only other kind Tables() ever pairs with
		// UsageWrite today, so this default only exists to keep the
		// function total (every possible StatementKind must return
		// *something*) rather than because any other kind is expected
		// to reach here in practice.
		return defs.TableWritePermission
	}
}

// authorizeStatement checks every table p's parsed statement references
// against session's permissions, as described in authorizeAndFormatStatements
// above. It returns http.StatusOK if every reference is permitted; otherwise
// it writes an error response to w and returns the status that was written.
func authorizeStatement(session *router.Session, w http.ResponseWriter, dsn string, p *sqlparse.Sqlparse) int {
	// Computed once per statement, outside the loop below, because -- per
	// writePermissionForKind's doc comment -- every UsageWrite reference in
	// this one statement needs the identical answer, so there is no reason
	// to ask p.StatementKind() more than once.
	kind := p.StatementKind()

	for _, t := range p.Tables() {
		table := baseTableName(t.Name)

		switch t.Usage {
		case sqlparse.UsageRead:
			if !Authorized(session, session.User, dsn+"."+table, defs.TableReadPermission) {
				return denyTable(session, w, table, defs.TableReadPermission)
			}
		case sqlparse.UsageWrite:
			// DATA-SECURITY-2.md finding #5: this used to always require
			// defs.TableWritePermission here, regardless of whether the
			// statement was an INSERT, UPDATE, or DELETE -- so a caller
			// granted only ego.table.write (meant to authorize inserting
			// new rows) could also UPDATE or DELETE existing ones via
			// @sql, bypassing the finer read/write/update/delete boundary
			// the plain REST row endpoints (rows.go) already enforce for
			// the exact same table. writePermissionForKind picks the
			// permission that actually matches what this statement does.
			permission := writePermissionForKind(kind)
			if !Authorized(session, session.User, dsn+"."+table, permission) {
				return denyTable(session, w, table, permission)
			}
		case sqlparse.UsageAdmin:
			// DATA-SECURITY.md §3.8: identity-level ego.dsn.admin
			// (hasPermission) is not the only way to be an admin of this
			// DSN -- a caller with a DSN-specific dsns_auth admin record
			// for it (e.g. its own creator, per §3.3's self-grant) can run
			// DDL against it too, same as DeleteDSNHandler/
			// DSNPermissionsHandler now accept for DSN administration
			// itself (§3.6).
			if !hasPermission(session, defs.DSNAdminPermission) &&
				!dsns.DSNService.AuthDSN(session.ID, session.User, dsn, dsns.DSNAdminAction) {
				return denyTable(session, w, table, defs.DSNAdminPermission)
			}
		}
	}

	return http.StatusOK
}

// denyTable logs and writes the 403 response for a single missing
// permission found by authorizeStatement.
func denyTable(session *router.Session, w http.ResponseWriter, table, permission string) int {
	ui.Log(ui.AuthLogger, "route.perm.auth", ui.A{
		"session":    session.ID,
		"permission": permission,
		"user":       session.User,
	})

	return util.ErrorResponse(w, session.ID, i18n.Text(session.Language, "error.sql.perm.table", ui.A{"table": table, "permission": permission}), http.StatusForbidden)
}
