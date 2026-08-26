package database

import (
	"database/sql"
	"net/url"
	"strings"

	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/dbpool"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/dsns"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/router"
	"github.com/tucats/ego/internal/server/auth"
	egostrings "github.com/tucats/ego/internal/util/strings"
)

type Database struct {
	Name        string
	Handle      *sql.DB
	Transaction *sql.Tx
	TransID     uint64
	TransUUID   string
	Session     *router.Session
	User        string
	DSN         string
	Provider    string
	Schema      string
	HasRowID    bool
	Restricted  bool
	Pooled      bool

	// RestrictSchema is true when this DSN's own definition named an
	// explicit Postgres schema (as opposed to Schema having been defaulted
	// to defs.DefaultSchema below because the DSN left it blank). It tells
	// sqlparse.RestrictToSchema's callers (sql_permissions.go and
	// scripting/authz.go) whether raw @sql/@transaction text may reference
	// a schema other than Schema at all, or -- for a DSN with no schema of
	// its own -- may name any schema, matching prior behavior.
	RestrictSchema bool
}

// Open the database that is associated with the named DSN.
//
// NILPTR-3: this function used to test "session != nil" before reading
// session.User, and then dereference session.ID on the very next line and
// session.Admin a few lines later, with no guard at all. Either the nil test was
// unnecessary or the eight dereferences that followed it were a crash waiting to
// happen -- the two cannot both be right.
//
// Rather than delete the nil test (which would silently commit to "session is
// never nil" for a function reachable from every table endpoint), the values
// actually needed are pulled out once, here, under a single nil check. The rest
// of the function then uses plain local variables that are always safe to read.
// A nil session is treated as an unauthenticated, non-admin caller, which is the
// conservative interpretation: it can only ever deny access, never grant it.
func Open(session *router.Session, name string, action dsns.DSNAction) (db *Database, err error) {
	var (
		user string
	)

	// You must specify a DSN name to open a database.
	if name == "" || name == defs.NilTypeString {
		return nil, errors.ErrMissingTableName
	}

	// sessionID is only used for log correlation, so zero is a fine stand-in
	// when there is no session. isAdmin must default to false so that a missing
	// session cannot bypass the DSN authorization check below.
	sessionID := 0
	isAdmin := false

	var permissions []string

	if session != nil {
		user = session.User
		sessionID = session.ID
		isAdmin = session.Admin
		permissions = session.Permissions
	}

	dsnName, err := dsns.DSNService.ReadDSN(sessionID, user, name, false)
	if err != nil {
		ui.Log(ui.DBLogger, "db.dsn.error", ui.A{
			"session": sessionID,
			"user":    user,
			"name":    name,
			"error":   err})

		return nil, err
	}

	savedUser := user

	if !isAdmin {
		// DATA-SECURITY.md §3.4: identity-level ego.dsn.admin/read/write --
		// a permission attached to the user's own identity, granting that
		// access for every DSN -- is checked first, ahead of the per-DSN
		// dsns_auth grant AuthDSN looks up. Before this, only a per-DSN
		// grant ever worked here: a user holding identity-level
		// ego.dsn.admin (e.g. because they created this very DSN, per
		// CreateDSNHandler's route-level Permissions() gate) still needed
		// a separate, explicit per-DSN record to open it at all.
		//
		// permissions falls back to a direct lookup when session.Permissions
		// was empty (e.g. a native Ego-token session reaching here before
		// the router's own lazy population runs) -- mirroring the same
		// two-path check sql_permissions.go's hasPermission uses, and for
		// the same reason: a federated (JWT/OAuth) session's permissions
		// live only on session.Permissions, with no local user record for
		// auth.GetPermissions to find, so this must prefer an already-
		// resolved non-empty list rather than re-deriving it.
		if len(permissions) == 0 && user != "" {
			permissions = auth.GetPermissions(sessionID, user)
		}

		if !dsns.IdentityAuthorizesAction(permissions, action) &&
			!dsns.DSNService.AuthDSN(sessionID, user, name, action) {
			ui.Log(ui.DBLogger, "db.dsn.no.auth", ui.A{
				"session": sessionID,
				"user":    user,
				"dsn":     dsnName.Name,
				"action":  dsns.ActionString(action)})

			return nil, errors.ErrNoPrivilegeForOperation
		}
	}

	ui.Log(ui.DBLogger, "db.dsn.auth", ui.A{
		"session": sessionID,
		"user":    user,
		"dsn":     dsnName.Name,
		"action":  dsns.ActionString(action)})

	// The actual table names sent to a PostgreSQL server are determined
	// solely by the DSN's own configured schema -- defaulting to "public"
	// when unset -- never by the caller's Ego identity. The Ego identity
	// checked above only gates whether this DSN/action is authorized at
	// all; once that passes, savedUser (used as the Postgres schema by every
	// query-composition call site in this package, via db.User below) must
	// reflect the DSN's schema, so that e.g. "admin" and "tom" identities
	// sharing the same restricted DSN reach the identical Postgres schema.
	// SQLite has no schema concept, so savedUser there stays the Ego
	// identity, though it goes unused by SQLite's query composition.
	var restrictSchema bool

	if dsnName.Provider == defs.PostgresProvider {
		if dsnName.Schema == "" {
			dsnName.Schema = defs.DefaultSchema
		} else {
			// The DSN named its own schema rather than falling back to
			// the default above -- lock raw SQL to that schema too. See
			// RestrictSchema's doc comment for why this must be decided
			// here, before the defaulting above makes an explicitly
			// configured schema indistinguishable from an unconfigured
			// one that happens to also be "public".
			restrictSchema = true
		}

		savedUser = dsnName.Schema
	}

	conStr, err := dsns.Connection(&dsnName)
	if err != nil {
		ui.Log(ui.DBLogger, "db.error", ui.A{
			"session": sessionID,
			"error":   err})

		return nil, err
	}

	ui.Log(ui.DBLogger, "db.dsn.constr", ui.A{
		"session": sessionID,
		"constr":  redactURLString(conStr)})

	db = &Database{
		User:           savedUser,
		DSN:            name,
		Schema:         dsnName.Schema,
		RestrictSchema: restrictSchema,
		HasRowID:       dsnName.RowId,
		Session:        session,
		Name:           dsnName.Name,
		Restricted:     dsnName.Restricted,
	}

	scheme, err := egostrings.FindScheme(conStr)
	if err == nil {
		// normalize provider aliases and apply any provider-specific connection setup.
		// To add a new provider: add a case for its scheme(s) and any required setup
		// (connection string rewriting, driver registration, post-open PRAGMAs, etc.).
		switch scheme {
		case defs.DeprecatedSqliteProvider, defs.SqliteProvider:
			// modernc.org/sqlite registers as "sqlite"; strip the scheme prefix to
			// obtain a bare filesystem path, then normalize the alias.
			conStr = strings.TrimPrefix(conStr, scheme+"://")
			scheme = defs.SqliteProvider

		case defs.PostgresProvider:
			// lib/pq uses the connection string as-is; no rewriting needed.

		default:
			// The scheme from the DSN connection string does not correspond to any
			// provider known to this server.  Fail here rather than passing an
			// unrecognized driver name to sql.Open.
			return db, errors.ErrUnsupportedDatabase.Context(scheme)
		}

		// dbpool.Get returns a shared, cached *sql.DB reused across requests
		// for this DSN name (see that package's doc comment), applying the
		// admin-tunable pool limits and any provider-specific post-open setup
		// (SQLite PRAGMAs, etc.) once, when the pool is first created, rather
		// than on every request. The returned bool records whether the handle
		// is shared -- Close/CloseTX below must not close a shared handle out
		// from under every other request currently using it.
		db.Provider = scheme
		db.Handle, db.Pooled, err = dbpool.Get(dsnName.Name, scheme, conStr)
	}

	return db, err
}

// Close is a shim to pass through to the underlying database handle.
// This does nothing if there are active/pending transactions for this
// handle.
//
// NILPTR-4: Open can return a non-nil *Database whose Handle is still nil --
// see the ErrUnsupportedDatabase return above, and the case where FindScheme
// fails and the sql.Open block is skipped entirely. Every current caller checks
// the error before deferring Close, so this is not reachable today, but Close is
// an exported method on an exported type and "defer db.Close()" placed before
// the error check is a very easy mistake to make. In Go, calling a method on a
// nil pointer is legal as long as the method does not dereference the receiver,
// so checking here costs nothing and makes the method safe to call in any state.
func (d *Database) Close() error {
	if d == nil {
		return nil
	}

	if d.Transaction != nil {
		ui.Log(ui.TableLogger, "table.tx.rest.not.closed", ui.A{
			"seq": d.TransID,
			"id":  d.TransUUID,
		})

		return nil
	}

	// A database that never finished opening has nothing to close.
	if d.Handle == nil {
		return nil
	}

	// A pooled handle is shared with every other request currently using
	// this DSN -- closing it here would tear down the pool out from under
	// them. Its lifecycle belongs to dbpool (idle eviction, DSN-change
	// eviction, or server shutdown), not to any single request.
	if d.Pooled {
		return nil
	}

	return d.Handle.Close()
}

// CloseTX is a shim to pass through to the underlying database handle.
// It will close the database, and dismiss active transactions. Brute
// force reclamation.
//
// NILPTR-4: guarded the same way as Close, for the same reason.
func (d *Database) CloseTX(session int) error {
	if d == nil {
		return nil
	}

	if d.Transaction != nil {
		err := d.Transaction.Commit()
		if err != nil {
			ui.Log(ui.TableLogger, "table.tx.rest.commit.error", ui.A{
				"session": session,
				"seq":     d.TransID,
				"id":      d.TransUUID,
				"error":   err.Error(),
			})

			d.Transaction.Rollback()
		} else {
			ui.Log(ui.TableLogger, "table.tx.rest.commit", ui.A{
				"session": session,
				"seq":     d.TransID,
				"id":      d.TransUUID,
			})
		}
	}

	if d.Handle == nil {
		return nil
	}

	// See Close's identical guard above -- a pooled handle must outlive this
	// one request/transaction.
	if d.Pooled {
		return nil
	}

	return d.Handle.Close()
}

func redactURLString(s string) string {
	url, err := url.Parse(s)
	if err != nil {
		return s
	}

	return url.Redacted()
}
