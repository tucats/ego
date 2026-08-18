package scripting

import (
	"net/http"
	"strings"

	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/dsns"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/router"
	"github.com/tucats/ego/internal/server/auth"
	"github.com/tucats/ego/internal/server/tables/database"
	"github.com/tucats/ego/internal/server/tables/parsing"
	"github.com/tucats/ego/internal/sqlparse"
)

// This file enforces the same table_perms/DSN-admin permission model the
// top-level tables.Authorized and the @sql endpoint's sql_permissions.go
// already enforce, applied here to the @transaction endpoint's per-opcode
// handlers (select.go, rows.go, insert.go, update.go, delete.go, drop.go)
// and to its raw-SQL "sql" opcode (sql.go's doSQL).
//
// scripting cannot import the parent "tables" package directly: tables
// (routes.go) already imports scripting to register the @transaction route,
// so importing back would be a cycle. tables.Authorized is instead injected
// into AuthorizedFunc below by the tables package's own init()
// (tables/scripting_authz.go) -- every server binary imports tables, which
// runs that init before any HTTP request can be routed, so AuthorizedFunc
// is always populated in production. It is nil only for a scripting-package
// unit test that never imports tables; authorizedForTable/authorizedForDDL
// treat that the same as a hand-built database.Database with a nil Session
// (see their comments below) -- neither situation can occur when a real
// request reaches this package through Handler.

// AuthorizedFunc mirrors tables.Authorized's signature.
var AuthorizedFunc func(session *router.Session, user string, table string, operations ...string) bool

// hasPermission mirrors tables/sql_permissions.go's function of the same
// name: true for the server administrator, then identity-wide permissions
// (preferring the session's already-resolved list, falling back to a
// direct lookup for a federated session that reached here before the
// router populated it -- see that file's own doc comment for why both
// paths are checked). Kept as a small local copy, rather than exported
// from tables/sql_permissions.go and imported, for the same import-cycle
// reason AuthorizedFunc is injected instead of imported directly.
func hasPermission(session *router.Session, permission string) bool {
	if session == nil {
		return false
	}

	if session.Admin {
		return true
	}

	if len(session.Permissions) > 0 {
		return session.HasAllPermissions(permission)
	}

	return auth.GetPermission(session.ID, session.User, permission)
}

// authorizedForTable reports whether db.Session's caller may perform
// requiredPermission against the named (raw, unqualified) table in db's
// DSN.
//
// db.Session is nil only when a test constructs a database.Database by
// hand instead of via database.Open (see e.g. drop_test.go and
// status_test.go) -- that bypasses the DSN service entirely, so there is
// no real session or DSN record to check against; treating it as
// "authorization does not apply" mirrors how those tests already bypass
// every other permission check in the tables package, since they exist to
// exercise query-building and execution, not authorization. Every request
// that reaches this package through Handler always has a non-nil
// db.Session (database.Open always sets it from the router's own
// session), so this fallback is unreachable in production.
func authorizedForTable(db *database.Database, table string, requiredPermission string) bool {
	if db.Session == nil || db.Session.Admin {
		return true
	}

	if AuthorizedFunc == nil {
		// Defensive only: tables' init() always sets this before any
		// request can be routed. Fail closed rather than silently
		// granting access if it is somehow unset.
		return false
	}

	return AuthorizedFunc(db.Session, db.Session.User, db.DSN+"."+table, requiredPermission)
}

// authorizedForDDL reports whether db.Session's caller may perform a
// schema-altering (DDL) operation against db's DSN. Mirrors
// sql_permissions.go's authorizeStatement UsageAdmin branch: identity-wide
// ego.dsn.admin (or ego.root), OR a DSN-specific admin record for this DSN
// (DATA-SECURITY.md §3.8) -- altering a DSN's schema is a DSN-wide
// capability, not something a per-table table_perms grant covers.
func authorizedForDDL(db *database.Database) bool {
	if db.Session == nil || db.Session.Admin {
		return true
	}

	return hasPermission(db.Session, defs.DSNAdminPermission) ||
		dsns.DSNService.AuthDSN(db.Session.ID, db.Session.User, db.DSN, dsns.DSNAdminAction)
}

// baseTableName strips a schema qualifier ("schema.table" -> "table") from
// a table name as reported by sqlparse's Tables(). table_perms records are
// keyed by a bare table name within a DSN -- there is no separate schema
// column -- so a schema-qualified reference in the SQL text is checked
// against the same table_perms entry as an unqualified one would be. Same
// helper as tables/sql_permissions.go's baseTableName.
func baseTableName(name string) string {
	if i := strings.LastIndex(name, "."); i >= 0 {
		return name[i+1:]
	}

	return name
}

// isSchemaAlteringKind reports whether kind is a DDL statement that can
// invalidate cached table schema metadata -- CREATE/ALTER/DROP TABLE,
// CREATE/DROP INDEX, or CREATE/DROP VIEW. Replaces the older
// parsing.IsSchemaAlteringStatement text-prefix heuristic (which only ever
// recognized ALTER TABLE and DROP TABLE) for any statement that parses.
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

// authorizeAndClassifySQL parses sqlText (expected to already be
// symbol-expanded -- see applySymbolsToTask) against db's provider
// dialect, authorizes every table it references against db.Session's
// permissions, and reports the statement's kind so the caller can decide
// whether to run it as a query (a SELECT) or an exec, and whether to flush
// the schema cache afterward. Mirrors tables/sql_permissions.go's
// authorizeAndFormatStatements/authorizeStatement, which do the same job
// for the top-level @sql endpoint.
//
// If sqlText fails to parse: a caller with no session to check against
// (db.Session == nil, e.g. a hand-built test database.Database -- see
// authorizedForTable) or an admin caller is let through exactly as this
// package always has, with kind == sqlparse.StmtUnknown so the caller
// falls back to a text-based heuristic; any other caller is rejected with
// 400, since there is then no way to know what the statement touches.
//
// On any authorization failure the returned status is http.StatusForbidden
// and err is non-nil; the caller must stop and not execute the statement.
func authorizeAndClassifySQL(db *database.Database, sqlText string) (sqlparse.StatementKind, bool, int, error) {
	dialect := sqlparse.SQLite
	if db.Provider == defs.PostgresProvider {
		dialect = sqlparse.PostgreSQL
	}

	noAuthCheck := db.Session == nil || db.Session.Admin

	p, parseErr := sqlparse.New(sqlText, dialect)
	if parseErr != nil {
		if noAuthCheck {
			return sqlparse.StmtUnknown, parsing.IsSchemaAlteringStatement(sqlText), http.StatusOK, nil
		}

		return sqlparse.StmtUnknown, false, http.StatusBadRequest, errors.New(parseErr)
	}

	kind := p.StatementKind()
	cacheFlush := isSchemaAlteringKind(kind)

	if noAuthCheck {
		return kind, cacheFlush, http.StatusOK, nil
	}

	for _, t := range p.Tables() {
		table := baseTableName(t.Name)

		switch t.Usage {
		case sqlparse.UsageRead:
			if !authorizedForTable(db, table, defs.TableReadPermission) {
				return kind, cacheFlush, http.StatusForbidden, errors.ErrNoPrivilegeForOperation.Context(table)
			}
		case sqlparse.UsageWrite:
			if !authorizedForTable(db, table, defs.TableWritePermission) {
				return kind, cacheFlush, http.StatusForbidden, errors.ErrNoPrivilegeForOperation.Context(table)
			}
		case sqlparse.UsageAdmin:
			if !authorizedForDDL(db) {
				return kind, cacheFlush, http.StatusForbidden, errors.ErrNoPrivilegeForOperation.Context(table)
			}
		}
	}

	return kind, cacheFlush, http.StatusOK, nil
}
