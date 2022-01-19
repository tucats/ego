package dbtables

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
)

func OpenDB(sessionID int32, user, table string) (db *sql.DB, err error) {
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

func ErrorResponse(w http.ResponseWriter, sessionID int32, msg string, status int) {
	response := defs.RestResponse{
		Message: msg,
		Status:  status,
	}

	b, _ := json.MarshalIndent(response, "", "  ")

	ui.Debug(ui.ServerLogger, "[%d] %s; %d", sessionID, msg, status)
	w.WriteHeader(status)
	_, _ = w.Write(b)
}
