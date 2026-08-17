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
// that way: UsageRead requires defs.TableReadPermission, UsageWrite
// requires defs.TableWritePermission (this collapses INSERT/UPDATE/DELETE
// into one check; table_perms' separate update/delete flags are not
// consulted here -- see the doc comment on Tables in sqlparse/analyze.go for
// the same read/write/admin split this mirrors), and UsageAdmin
// (CREATE/ALTER/DROP TABLE, CREATE/DROP INDEX, CREATE/DROP VIEW) requires
// defs.DSNAdminPermission instead of a table_perms grant, since altering the
// DSN's schema is a DSN-wide capability rather than something a per-table
// grant is meant to cover.
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

// authorizeStatement checks every table p's parsed statement references
// against session's permissions, as described in authorizeAndFormatStatements
// above. It returns http.StatusOK if every reference is permitted; otherwise
// it writes an error response to w and returns the status that was written.
func authorizeStatement(session *router.Session, w http.ResponseWriter, dsn string, p *sqlparse.Sqlparse) int {
	for _, t := range p.Tables() {
		table := baseTableName(t.Name)

		switch t.Usage {
		case sqlparse.UsageRead:
			if !Authorized(session, session.User, dsn+"."+table, defs.TableReadPermission) {
				return denyTable(session, w, table, defs.TableReadPermission)
			}
		case sqlparse.UsageWrite:
			if !Authorized(session, session.User, dsn+"."+table, defs.TableWritePermission) {
				return denyTable(session, w, table, defs.TableWritePermission)
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
