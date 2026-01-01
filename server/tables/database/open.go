package database

import (
	"database/sql"
	"net/url"
	"strings"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/server/dsns"
	"github.com/tucats/ego/server/server"
)

type Database struct {
	Name        string
	Handle      *sql.DB
	Transaction *sql.Tx
	TransID     uint64
	Session     *server.Session
	User        string
	DSN         string
	Provider    string
	Schema      string
	HasRowID    bool
}

// Open the database that is associated with the named DSN.
func Open(session *server.Session, name string, action dsns.DSNAction) (db *Database, err error) {
	var (
		url  *url.URL
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

	if !dsns.DSNService.AuthDSN(session.ID, user, name, action) {
		ui.Log(ui.DBLogger, "db.dsn.no.auth", ui.A{
			"session": session.ID,
			"user":    user,
			"dsn":     dsnName.Name,
			"action":  dsns.ActionString(action)})

		return nil, errors.ErrNoPrivilegeForOperation
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

	url, err = url.Parse(conStr)
	if err == nil {
		scheme := url.Scheme
		if scheme == "sqlite3" {
			conStr = strings.TrimPrefix(conStr, scheme+"://")
		}

		db.Handle, err = sql.Open(scheme, conStr)
		db.Provider = scheme
	}

	return db, err
}

// Close is a shim to pass through to the underlying database handle.
func (d *Database) Close() error {
	if d.Transaction != nil {
		err := d.Transaction.Commit()
		if err != nil {
			d.Transaction.Rollback()
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
