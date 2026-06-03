package database

import (
	"database/sql"
	"net/url"
	"strings"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/dsns"
	"github.com/tucats/ego/egostrings"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/router"
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
}

// Open the database that is associated with the named DSN.
func Open(session *router.Session, name string, action dsns.DSNAction) (db *Database, err error) {
	var (
		user string
	)

	// You must specify a DSN name to open a database.
	if name == "" || name == defs.NilTypeString {
		return nil, errors.ErrMissingTableName
	}

	if session != nil {
		user = session.User
	}

	dsnName, err := dsns.DSNService.ReadDSN(session.ID, user, name, false)
	if err != nil {
		ui.Log(ui.DBLogger, "db.dsn.error", ui.A{
			"session": session.ID,
			"user":    user,
			"name":    name,
			"error":   err})

		return nil, err
	}

	savedUser := user

	if !session.Admin {
		if !dsns.DSNService.AuthDSN(session.ID, user, name, action) {
			ui.Log(ui.DBLogger, "db.dsn.no.auth", ui.A{
				"session": session.ID,
				"user":    user,
				"dsn":     dsnName.Name,
				"action":  dsns.ActionString(action)})

			return nil, errors.ErrNoPrivilegeForOperation
		}
	}

	ui.Log(ui.DBLogger, "db.dsn.auth", ui.A{
		"session": session.ID,
		"user":    user,
		"dsn":     dsnName.Name,
		"action":  dsns.ActionString(action)})

	// If there is an explicit schema in this DSN, make that the
	// "user" identity for this operation.
	if dsnName.Schema != "" {
		savedUser = dsnName.Schema
	}

	conStr, err := dsns.Connection(&dsnName)
	if err != nil {
		ui.Log(ui.DBLogger, "db.error", ui.A{
			"session": session.ID,
			"error":   err})

		return nil, err
	}

	ui.Log(ui.DBLogger, "db.dsn.constr", ui.A{
		"session": session.ID,
		"constr":  redactURLString(conStr)})

	db = &Database{
		User:     savedUser,
		DSN:      name,
		Schema:   dsnName.Schema,
		HasRowID: dsnName.RowId,
		Session:  session,
		Name:     dsnName.Name,
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

		db.Handle, err = sql.Open(scheme, conStr)
		db.Provider = scheme

		// Apply post-open setup that is specific to each provider.
		if err == nil {
			switch scheme {
			case defs.SqliteProvider:
				// Enable Write-Ahead Logging for better concurrent read performance,
				// and set a busy timeout so writers do not fail immediately when the
				// database is locked by another writer.
				db.Handle.Exec("PRAGMA journal_mode=WAL;")
				db.Handle.Exec("PRAGMA busy_timeout=5000;")

				// PostgreSQL: no post-open PRAGMA-style configuration needed.
			case defs.PostgresProvider:
			}
		}
	}

	return db, err
}

// Close is a shim to pass through to the underlying database handle.
// This does nothing if there are active/pending transactions for this
// handle.
func (d *Database) Close() error {
	if d.Transaction != nil {
		ui.Log(ui.TableLogger, "table.tx.rest.not.closed", ui.A{
			"seq": d.TransID,
			"id":  d.TransUUID,
		})

		return nil
	}

	return d.Handle.Close()
}

// CloseTX is a shim to pass through to the underlying database handle.
// It will close the database, and dismiss active transactions. Brute
// force reclamation.
func (d *Database) CloseTX(session int) error {
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

	return d.Handle.Close()
}

func redactURLString(s string) string {
	url, err := url.Parse(s)
	if err != nil {
		return s
	}

	return url.Redacted()
}
