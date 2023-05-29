package database

import (
	"database/sql"
	"fmt"
	"net/url"
	"strings"

	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/http/dsns"
)

// openDefault opens the database that hosts the /tables service. This can be
// a Postgres or sqlite3 database. The database URI is found in the config
//
//	data. Credentials for the databse connection can also be stored in the
//
// configuration if needed and not part of the database URI.
func openDefault() (db *sql.DB, err error) {
	// Is a full database access URL provided?  If so, use that. Otherwise,
	// we assume it's a postgres server on the local system, and fill in the
	// info with the database credentials, name, etc.
	conStr := settings.Get(defs.TablesServerDatabase)
	if conStr == "" {
		credentials := settings.Get(defs.TablesServerDatabaseCredentials)
		if credentials != "" {
			credentials = credentials + "@"
		}

		dbname := settings.Get(defs.TablesServerDatabaseName)
		if dbname == "" {
			dbname = "ego_tables"
		}

		sslMode := "?sslmode=disable"
		if settings.GetBool(defs.TablesServerDatabaseSSLMode) {
			sslMode = ""
		}

		conStr = fmt.Sprintf("postgres://%slocalhost/%s%s", credentials, dbname, sslMode)
	}

	var url *url.URL

	url, err = url.Parse(conStr)
	if err == nil {
		scheme := url.Scheme
		if scheme == "sqlite3" {
			conStr = strings.TrimPrefix(conStr, scheme+"://")
		}

		db, err = sql.Open(scheme, conStr)
	}

	return db, err
}

// OpenDSN opens the database that is associated with the named DSN.
func Open(user *string, name string) (db *sql.DB, err error) {
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

	url, err = url.Parse(conStr)
	if err == nil {
		scheme := url.Scheme
		if scheme == "sqlite3" {
			conStr = strings.TrimPrefix(conStr, scheme+"://")
		}

		ui.Log(ui.DBLogger, "using connection string: %s", conStr)

		db, err = sql.Open(scheme, conStr)
	}

	return db, err
}
