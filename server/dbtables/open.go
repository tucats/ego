package dbtables

import (
	"database/sql"
	"net/url"
	"strings"

	"github.com/tucats/ego/app-cli/persistence"
)

func OpenDB(sessionID int32, user, table string) (db *sql.DB, err error) {
	conStr := persistence.Get("ego.tables.database")
	if conStr == "" {
		// @tomcole remove this before finalizing
		conStr = "postgres://tom:secret@localhost/tom?sslmode=disable"
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
