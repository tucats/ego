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
)

type Database struct {
	Handle   *sql.DB
	User     string
	DSN      string
	Provider string
	Schema   string
	HasRowID bool
}

// openDefault opens the database that hosts the /tables service. This can be
// a Postgres or sqlite3 database. The database URI is found in the config
//
//	data. Credentials for the database connection can also be stored in the
//
// configuration if needed and not part of the database URI.
func openDefault() (*Database, error) {
	// Is a full database access URL provided?  If so, use that. Otherwise,
	// we assume it's a postgres server on the local system, and fill in the
	// info with the database credentials, name, etc.
	conStr := settings.Get(defs.TablesServerDatabase)
	databaseName := settings.Get(defs.TablesServerDatabaseName)

	// Sanity check; if there is no database, and no dsn, then we have no
	// configured database.
	if conStr == "" && databaseName == "" {
		return nil, errors.ErrNoDatabase
	}

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

	return &Database{Handle: handle, Provider: url.Scheme, HasRowID: true}, err
}

// OpenDSN opens the database that is associated with the named DSN.
func Open(user *string, name string, action dsns.DSNAction) (db *Database, err error) {
	var url *url.URL

	if name == "" || name == defs.NilTypeString {
		return openDefault()
	}

	ui.Log(ui.DBLogger, "db.dsn", ui.A{
		"name": name})

	dsnName, err := dsns.DSNService.ReadDSN(*user, name, false)
	if err != nil {
		ui.Log(ui.DBLogger, "db.dsn.error", ui.A{
			"user":  *user,
			"name":  name,
			"error": err})

		return nil, err
	}

	ui.Log(ui.DBLogger, "db.dsn.found", ui.A{
		"name": dsnName.Name})

	savedUser := *user

	if !dsns.DSNService.AuthDSN(*user, name, action) {
		ui.Log(ui.DBLogger, "db.dsn.no.auth", ui.A{
			"user":   name,
			"dsn":    dsnName.Name,
			"action": action})

		return nil, errors.ErrNoPrivilegeForOperation
	}

	ui.Log(ui.DBLogger, "db.dsn.auth", ui.A{
		"user":   name,
		"dsn":    dsnName.Name,
		"action": action})

	// If there is an explicit schema in this DSN, make that the
	// "user" identity for this operation.
	if dsnName.Schema != "" {
		*user = dsnName.Schema
	}

	conStr, err := dsns.Connection(&dsnName)
	if err != nil {
		ui.Log(ui.DBLogger, "db.error", ui.A{
			"error": err})

		return nil, err
	}

	ui.Log(ui.DBLogger, "db.dsn.constr", ui.A{
		"constr": redactURLString(conStr)})

	db = &Database{
		User:     savedUser,
		DSN:      name,
		Schema:   dsnName.Schema,
		HasRowID: dsnName.RowId,
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

// Query is a shim to pass through to the underlying database handle.
func (d *Database) Query(sqlText string, parameters ...interface{}) (*sql.Rows, error) {
	return d.Handle.Query(sqlText, parameters...)
}

// Exec is a shim to pass through to the underlying database handle.
func (d *Database) Exec(sqlText string, parameters ...interface{}) (sql.Result, error) {
	return d.Handle.Exec(sqlText, parameters...)
}

// Close is a shim to pass through to the underlying database handle.
func (d *Database) Close() {
	d.Handle.Close()
}

// Begin is a shim to pass through to the underlying database handle.
func (d *Database) Begin() (*sql.Tx, error) {
	return d.Handle.Begin()
}

func redactURLString(s string) string {
	url, err := url.Parse(s)
	if err != nil {
		return s
	}

	return url.Redacted()
}
