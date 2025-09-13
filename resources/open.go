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

// Open opens a database handle to the named table. The object is used to
// determine the column names and types for the table, by using reflection
// to evaluate the object's structure type. It is an error if the object
// is not some kind of structure. The connection string can reference either
// a Postgres or SQLite3 database URL.
//
// The resulting handle can be used to read, write, update, or delete instances
// of the object's type from the database. If an error occurs accessing the
// database or evaluating the fields of the object structure, an error is
// returned and the resource handle will be nil.
func Open(object any, table, connection string) (*ResHandle, error) {
	var (
		err error
		url *url.URL
	)

	handle := &ResHandle{
		Table:   table,
		Name:    reflect.TypeOf(object).String(),
		Columns: describe(object),
		Type:    reflect.ValueOf(object).Type(),
	}

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

// Close closes the underlying database represented by this handle. If the
// handle is nil, no action is taken. When this is called on a valid handle,
// the database driver determines what resource(s) are free up versus cached.
func (r *ResHandle) Close() {
	if r != nil {
		r.Database.Close()
	}
}

func (r *ResHandle) DropAllResources() error {
	if r != nil {
		_, err := r.Database.Exec("DROP TABLE IF EXISTS " + r.Table)
		if err != nil {
			return err
		}

		return r.Database.Close()
	}

	return nil
}
