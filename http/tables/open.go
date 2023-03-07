package tables

import (
	"database/sql"
	"fmt"
	"net/url"
	"strings"

	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/defs"
)

// OpenDB opens the database that hosts the /tables service. This can be
// a Postgres or sqlite3 database. The database URI is found in the config
//  data. Credentials for the databse connection can also be stored in the 
// configuration if needed and not part of the database URI.
func OpenDB() (db *sql.DB, err error) {
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
