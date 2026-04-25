// Package db manages the Ego database interfaces, mirroring the standard
// "database/sql" package in conventional Go. It provides:
//
//   - db.New(connStr) — open a connection and return a db.Client struct
//   - db.Client       — struct with methods Execute, Query, QueryResult,
//     Begin, Commit, Rollback, Close, AsStruct
//   - db.Rows         — cursor struct with methods Next, Scan, Close, Headings
//
// All exported symbols are registered in DBPackage (types.go) so that Ego
// code can access them via `import "db"`.
//
// Supported URL schemes: "sqlite3://" and "postgres://" (via driver imports).
// The connection string must be a valid URL; e.g., "sqlite3:///tmp/test.db"
// or "postgres://user:pass@host/dbname".
package db

import (
	"database/sql"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"

	// Blank imports to make sure we link in the database drivers.
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
)

// newConnection implements db.New(connStr string) and is the entry point for
// opening a database connection from Ego code. It accepts a single string
// argument in URL format (e.g., "sqlite3:///tmp/payroll.db" or
// "postgres://user:pass@host/dbname"), opens the underlying *sql.DB, and
// returns a fully initialized db.Client *data.Struct.
//
// Security: when the scheme is "sqlite3", the requested file base name is
// compared (case-insensitively) to the server's credentials database file
// (see ego.server.userdata setting). If they match, the call is rejected with
// ErrNoPrivilegeForOperation to prevent sandboxed Ego code from reading or
// modifying server authentication data.
//
// Any password embedded in the URL is redacted before being stored in the
// Constr field and before it is written to the diagnostic log.
func newConnection(s *symbols.SymbolTable, args data.List) (any, error) {
	// Get the connection string, which MUST be in URL format.
	connStr := data.String(args.Get(0))

	url, err := url.Parse(connStr)
	if err != nil {
		return nil, errors.New(err)
	}

	if scheme := url.Scheme; scheme == "sqlite3" {
		connStr = strings.TrimPrefix(connStr, scheme+"://")
		// While we're here, make sure we are not talking to the credentials database.
		// Code running in a user-supplied service (or via the dashboard code tab) run
		// in the context of the server. We don't want to allow such code to talk to the
		// credentials database. The only time we care about this is when it's a sqlite3
		// database since file system protections won't suffice.
		requestedBaseName := filepath.Base(connStr)

		configPath := settings.Get("ego.server.userdata")
		if configPath == "" {
			configPath = defs.DefaultUserdataFileName
		} else if strings.HasPrefix(strings.ToLower(configPath), "sqlite3://") {
			configPath = strings.TrimPrefix(configPath, "sqlite3://")
		}

		if strings.EqualFold(requestedBaseName, filepath.Base(configPath)) {
			return nil, errors.ErrNoPrivilegeForOperation.Context(connStr)
		}
	}

	db, err := sql.Open(url.Scheme, connStr)
	if err != nil {
		return nil, errors.New(err)
	}

	// If there was a password specified in the URL, blank it out now before we log it.
	if secretString, found := url.User.Password(); found {
		connStr = strings.ReplaceAll(connStr, ":"+secretString+"@", ":"+strings.Repeat("*", len(secretString))+"@")
	}

	ui.Log(ui.DBLogger, "db.connect", ui.A{
		"constr": redactURLString(connStr)})

	_ = s.Set(DBClientType.Name(), DBClientType)

	result := data.NewStruct(DBClientType).
		FromBuiltinPackage().
		SetAlways(clientFieldName, db).
		SetAlways(constrFieldName, connStr).
		SetAlways(asStructFieldName, false).
		SetAlways(rowCountFieldName, 0).
		SetReadonly(true)

	return result, nil
}

// asStructures implements the db.Client.AsStruct(flag bool) method. It sets
// the asStruct field on the Client struct which controls how subsequent
// Query() and QueryResult() calls format their results:
//
//   - false (default) — each row is a *data.Array of column values in the
//     same order as the SELECT list; callers use integer indices
//   - true            — each row is a *data.Struct whose field names match
//     the column names; callers use field-name access
//
// The method returns the Client struct itself so Ego code can chain calls.
func asStructures(s *symbols.SymbolTable, args data.List) (any, error) {
	if _, _, err := client(s); err != nil {
		return nil, err
	}

	this := getThis(s)
	this.SetAlways(asStructFieldName, data.BoolOrFalse(args.Get(0)))

	return this, nil
}

// closeConnection implements the db.Client.Close() method. It rolls back any
// active transaction, closes the underlying *sql.DB, and then zeroes all
// fields on the Client struct. Zeroing the fields achieves two goals:
//
//  1. It prevents accidental re-use of the connection — subsequent calls
//     to client() will find clientFieldName == nil and return an error.
//  2. It releases the references to native objects so the garbage collector
//     can reclaim them.
//
// Returns (true, nil) on success; (true, error) if the rollback failed.
func closeConnection(s *symbols.SymbolTable, args data.List) (any, error) {
	db, tx, err := client(s)
	if err != nil {
		return nil, err
	}

	if tx != nil {
		err = tx.Rollback()
	}

	db.Close()

	this := getThis(s)
	this.SetAlways(clientFieldName, nil)
	this.SetAlways(constrFieldName, "")
	this.SetAlways(transactionFieldName, nil)
	this.SetAlways(asStructFieldName, false)
	this.SetAlways(rowCountFieldName, -1)

	if err != nil {
		err = errors.New(err)
	}

	return true, err
}

// client is an internal helper that extracts the *sql.DB and optional *sql.Tx
// from the symbol table's receiver (__this). The lookup chain is:
//
//  1. Read defs.ThisVariable from the symbol table — must be *data.Struct
//  2. Read clientFieldName from that struct — must be a non-nil *sql.DB
//  3. Optionally read transactionFieldName — if non-nil, unwrap and cast to
//     *sql.Tx so the caller can use tx.Exec / tx.Query when inside a
//     transaction
//
// Errors returned:
//   - ErrDatabaseClientClosed — client field is a typed nil *sql.DB
//   - ErrNoFunctionReceiver   — __this is missing, wrong type, or
//     clientFieldName is nil/not a *sql.DB
func client(symbols *symbols.SymbolTable) (*sql.DB, *sql.Tx, error) {
	if g, ok := symbols.Get(defs.ThisVariable); ok {
		if gc, ok := g.(*data.Struct); ok {
			if client := gc.GetAlways(clientFieldName); client != nil {
				if cp, ok := client.(*sql.DB); ok {
					if cp == nil {
						return nil, nil, errors.ErrDatabaseClientClosed
					}

					tx, _ := data.UnWrap(gc.GetAlways(transactionFieldName))
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

// getThis retrieves the __this *data.Struct from the symbol table.
// It returns nil (rather than an error) when the symbol is missing or has the
// wrong type, so callers that need a hard error should use client() instead.
func getThis(s *symbols.SymbolTable) *data.Struct {
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

// redactURLString parses s as a URL and returns url.URL.Redacted(), which
// replaces any password component with "xxxxx". If parsing fails the original
// string is returned unchanged. This is used to sanitize connection strings
// before they are written to diagnostic logs.
func redactURLString(s string) string {
	url, err := url.Parse(s)
	if err != nil {
		return s
	}

	return url.Redacted()
}
