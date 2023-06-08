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
	"github.com/tucats/ego/http/dsns"
)

type Database struct {
	Handle   *sql.DB
	Provider string
}

// openDefault opens the database that hosts the /tables service. This can be
// a Postgres or sqlite3 database. The database URI is found in the config
//
//	data. Credentials for the databse connection can also be stored in the
//
// configuration if needed and not part of the database URI.
func openDefault() (*Database, error) {
	// Is a full database access URL provided?  If so, use that. Otherwise,
	// we assume it's a postgres server on the local system, and fill in the
	// info with the database credentials, name, etc.
	conStr := settings.Get(defs.TablesServerDatabase)
	dbname := settings.Get(defs.TablesServerDatabaseName)

	// Sanity check; if there is no database, and no dbname, then we have no
	// configured database.
	if conStr == "" && dbname == "" {
		return nil, errors.ErrNoDatabase
	}

	// If we didn't have a connection string, construct one from parts...
	if conStr == "" {
		credentials := settings.Get(defs.TablesServerDatabaseCredentials)
		if credentials != "" {
			credentials = credentials + "@"
		}

		if dbname == "" {
			dbname = "ego_tables"
		}

		sslMode := "?sslmode=disable"
		if settings.GetBool(defs.TablesServerDatabaseSSLMode) {
			sslMode = ""
		}

		conStr = fmt.Sprintf("postgres://%slocalhost/%s%s", credentials, dbname, sslMode)
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

	return &Database{Handle: handle, Provider: "postgres"}, err
}

// OpenDSN opens the database that is associated with the named DSN.
func Open(user *string, name string) (db *Database, err error) {
	if name == "" || name == "<nil>" {
		return openDefault()
	}

	ui.Log(ui.DBLogger, "accessing using DSN %s", name)

	dsname, err := dsns.DSNService.ReadDSN(*user, name, false)
	if err != nil {
		return nil, err
	}

	// If there is an explicit schema in this DSN, make that the
	// "user" identity for this operation.
	if dsname.Schema != "" {
		*user = dsname.Schema
	}

	conStr, err := dsns.Connection(&dsname)
	if err != nil {
		return nil, err
	}

	var url *url.URL

	db = &Database{}

	url, err = url.Parse(conStr)
	if err == nil {
		scheme := url.Scheme
		if scheme == "sqlite3" {
			conStr = strings.TrimPrefix(conStr, scheme+"://")
		}

		ui.Log(ui.DBLogger, "using connection string: %s", conStr)

		db.Handle, err = sql.Open(scheme, conStr)
		db.Provider = scheme
	}

	return db, err
}

// Query is a shim to pass through to the underlying database handle.
func (d *Database) Query(sqltext string, parameters ...interface{}) (*sql.Rows, error) {
	return d.Handle.Query(sqltext, parameters...)
}

// Exec is a shim to pass through to the underlying database handle.
func (d *Database) Exec(sqltext string, parameters ...interface{}) (sql.Result, error) {
	return d.Handle.Exec(sqltext, parameters...)
}

// Close is a shim to pass through to the underlying database handle.
func (d *Database) Close() {
	d.Handle.Close()
}

// Begin is a shim to pass through to the underlying database handle.
func (d *Database) Begin() (*sql.Tx, error) {
	return d.Handle.Begin()
}
