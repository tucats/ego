package resources

import (
	"database/sql"
	"net/url"
	"reflect"
	"strings"

	// Blank imports to make sure we link in the database drivers.
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
)

// Open opens a database handle to the named table.
func Open(object interface{}, table, connection string) (*ResHandle, error) {
	var err error

	handle := &ResHandle{
		Table:   table,
		Columns: describe(object),
		Type:    reflect.ValueOf(object).Type(),
	}

	var url *url.URL

	url, err = url.Parse(connection)
	if err == nil {
		scheme := url.Scheme
		if scheme == "sqlite3" {
			connection = strings.TrimPrefix(connection, scheme+"://")
		}

		handle.Database, err = sql.Open(scheme, connection)
	}

	// Pack it up and go home. If we had a problem, the handle must be nil.
	if err != nil {
		return nil, err
	}

	return handle, err
}

func (r *ResHandle) Close() {
	r.Database.Close()
}
