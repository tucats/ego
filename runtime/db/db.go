// The db package manages the Ego data base interfaces, similar to the
// sql package in conventional Go. There is basic functionality
// for creating a new connection, and then using that connection
// object (a db.Client) to perform queries, etc. A db.Rows type
// is also defined for row sets.
package db

import (
	"database/sql"
	"net/url"
	"strings"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"

	// Blank imports to make sure we link in the database drivers.
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
)

// New implements the New() db function. This allocated a new structure that
// contains all the info needed to call the database, including the function pointers
// for the functions available to a specific handle.
func New(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) != 1 {
		return nil, errors.ErrArgumentCount
	}


	// Get the connection string, which MUST be in URL format.
	connStr := data.String(args[0])

	url, err := url.Parse(connStr)
	if err != nil {
		return nil, errors.NewError(err)
	}

	if scheme := url.Scheme; scheme == "sqlite3" {
		connStr = strings.TrimPrefix(connStr, scheme+"://")
	}

	db, err := sql.Open(url.Scheme, connStr)
	if err != nil {
		return nil, errors.NewError(err)
	}

	// If there was a password specified in the URL, blank it out now before we log it.
	if secretString, found := url.User.Password(); found {
		connStr = strings.ReplaceAll(connStr, ":"+secretString+"@", ":"+strings.Repeat("*", len(secretString))+"@")
	}

	ui.Log(ui.DBLogger, "Connecting to %s", connStr)

	_ = s.Set(clientType.Name(), clientType)

	result := data.NewStruct(clientType).FromBuiltinPackage()
	result.SetAlways(clientFieldName, db)
	result.SetAlways(constrFieldName, connStr)
	result.SetAlways(asStructFieldName, false)
	result.SetAlways(rowCountFieldName, 0)
	result.SetReadonly(true)

	return result, nil
}

// AsStructures sets the asStruct flag. When true, result sets from queries are an array
// of structs, where the struct members are the same as the result set column names. When
// not true, the result set is an array of arrays, where the inner array contains the
// column data in the order of the result set, but with no labels, etc.
func AsStructures(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) != 1 {
		return nil, errors.ErrArgumentCount
	}

	_, _, err := client(s)
	if err != nil {
		return nil, err
	}

	this := getThisStruct(s)
	this.SetAlways(asStructFieldName, data.Bool(args[0]))

	return this, nil
}

// Close closes the database connection, frees up any resources held, and resets the
// handle contents to prevent re-using the connection.
func Close(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) > 0 {
		return nil, errors.ErrArgumentCount
	}

	_, tx, err := client(s)
	if err != nil {
		return nil, err
	}

	if tx != nil {
		err = tx.Rollback()
	}

	this := getThisStruct(s)
	this.SetAlways(clientFieldName, nil)
	this.SetAlways(constrFieldName, "")
	this.SetAlways(transactionFieldName, nil)
	this.SetAlways(asStructFieldName, false)
	this.SetAlways(rowCountFieldName, -1)

	if err != nil {
		err = errors.NewError(err)
	}

	return true, err
}

// getClient searches the symbol table for the client receiver (defs.ThisVariable)
// variable, validates that it contains a database client object, and returns
// the native client object.
func client(symbols *symbols.SymbolTable) (*sql.DB, *sql.Tx, error) {
	if g, ok := symbols.Get(defs.ThisVariable); ok {
		if gc, ok := g.(*data.Struct); ok {
			if client := gc.GetAlways(clientFieldName); client != nil {
				if cp, ok := client.(*sql.DB); ok {
					if cp == nil {
						return nil, nil, errors.ErrDatabaseClientClosed
					}

					tx := gc.GetAlways(transactionFieldName)
					if tx == nil {
						return cp, nil, nil
					}

					return cp, tx.(*sql.Tx), nil
				}
			}
		}
	}

	return nil, nil, errors.ErrNoFunctionReceiver
}

// getThis returns a map for the "this" object in the current
// symbol table.
func getThisStruct(s *symbols.SymbolTable) *data.Struct {
	t, ok := s.Get(defs.ThisVariable)
	if !ok {
		return nil
	}

	this, ok := t.(*data.Struct)
	if !ok {
		return nil
	}

	return this
}
