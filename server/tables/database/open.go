package database

import (
	"database/sql"
	"fmt"
	"net/url"
	"strings"

	"github.com/tucats/ego/app-cli/settings"
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

// IsDefaultDatabaseConfigured returns true if a default database is configured. This
// is determined by checking the configuration data for a default connection
// string or default database name.
func IsDefaultDatabaseConfigured() bool {
	conStr := settings.Get(defs.TablesServerDatabase)
	databaseName := settings.Get(defs.TablesServerDatabaseName)

	return conStr != "" || databaseName != ""
}

// openDefault opens the database that hosts the /tables service. This can be
// a Postgres or sqlite3 database. The database URI is found in the config
//
//	data. Credentials for the database connection can also be stored in the
//
// configuration if needed and not part of the database URI.
func openDefault(session *server.Session) (*Database, error) {
	// Sanity check; if there is no database, and no dsn, then we have no
	// configured database.
	if !IsDefaultDatabaseConfigured() {
		return nil, errors.ErrNoDatabase
	}

	// Is a full database access URL provided?  If so, use that. Otherwise,
	// we assume it's a postgres server on the local system, and fill in the
	// info with the database credentials, name, etc.
	conStr := settings.Get(defs.TablesServerDatabase)
	databaseName := settings.Get(defs.TablesServerDatabaseName)

	// If we didn't have a connection string, construct one from parts...
	if conStr == "" {
		credentials := settings.Get(defs.TablesServerDatabaseCredentials)
		if credentials != "" {
			credentials = credentials + "@"
		}

		if databaseName == "" {
			databaseName = "ego_tables"
		}

		sslMode := "?sslmode=disable"
		if settings.GetBool(defs.TablesServerDatabaseSSLMode) {
			sslMode = ""
		}

		conStr = fmt.Sprintf("postgres://%slocalhost/%s%s", credentials, databaseName, sslMode)
	}

	var (
		url    *url.URL
		handle *sql.DB
		err    error
	)

	url, err = url.Parse(conStr)
	if err == nil {
		scheme := url.Scheme
		if scheme == "sqlite3" {
			conStr = strings.TrimPrefix(conStr, scheme+"://")
		}

		handle, err = sql.Open(scheme, conStr)
	}

	return &Database{Handle: handle, Provider: url.Scheme, HasRowID: true, Session: session, Name: databaseName}, err
}

// Open the database that is associated with the named DSN.
func Open(session *server.Session, name string, action dsns.DSNAction) (db *Database, err error) {
	var (
		url  *url.URL
		user string
	)

	if name == "" || name == defs.NilTypeString {
		return openDefault(session)
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
