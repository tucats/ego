// Package sql manages the Ego database interfaces, mirroring the standard
// "database/sql" package in conventional Go. It provides:
//
//   - sql.Open(driver, connStr) — open a connection and return a sql.Database struct
//   - sql.Database              — struct with methods Execute, Query, QueryResult,
//     Begin, Commit, Rollback, Close, AsStruct
//   - sql.Rows                  — cursor struct with methods Next, Scan, Close, Headings
//
// All exported symbols are registered in SqlPackage (types.go) so that Ego
// code can access them via `import "sql"`.
//
// Supported drivers are "sqlite" and "postgres" (via driver imports).
package sql

import (
	goSQL "database/sql"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/tucats/ego/internal/cli/settings"
	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/dsns"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/symbols"
	"github.com/tucats/ego/internal/util"

	// Blank imports to make sure we link in the database drivers.
	_ "github.com/lib/pq"
	_ "modernc.org/sqlite"
)

// supportedDrivers lists the driver names that Ego code may specify.
// "sqlite3" is accepted as a user-friendly alias and is normalized to "sqlite"
// (the name under which modernc.org/sqlite registers itself) before the actual
// goSQL.Open call. The "dsn" pseudo-driver is resolved to a real driver at
// open time by looking up the named data source.
var supportedDrivers = []string{defs.DeprecatedSqliteProvider, defs.SqliteProvider, "postgres", "dsn"}

// openDatabase implements goSQL.Open(driver, connStr string) and is the entry point for
// opening a database connection from Ego code. The driver must be either "sqlite"
// or "postgresql". The second parameter is the connection string for the database.
// goSQL.Open() opens the underlying *goSQL.DB, and
// returns a fully initialized goSQL.DB *data.Struct.
//
// Security: when the scheme is "sqlite", the requested file base name is
// compared (case-insensitively) to the server's credentials database file
// (see ego.server.userdata setting). If they match, the call is rejected with
// ErrNoPrivilegeForOperation to prevent sandboxed Ego code from reading or
// modifying server authentication data.
//
// Any password embedded in the URL is redacted before being stored in the
// Constr field and before it is written to the diagnostic log.
func openDatabase(s *symbols.SymbolTable, args data.List) (any, error) {
	// Get the driver string, which must be one of the supported drivers.
	driverType := strings.ToLower(data.String(args.Get(0)))
	if !util.InList(driverType, supportedDrivers...) {
		err := errors.ErrUnsupportedDatabase.Context(driverType)

		return data.NewList(nil, err), err
	}

	// Get the connection string
	connStr := data.String(args.Get(1))

	// If the driver type is "dsn", now is the time to fetch the "real" info
	// from the dsn database and use that instead.
	if driverType == "dsn" {
		var (
			user    string
			name    string
			session int
		)

		// The connection string for a dsn can have several parts, separated
		// by commas. Parse them to see what's here...
		name, user, session, err := dsns.ParseDSN(connStr)
		if err != nil {
			return data.NewList(nil, err), err
		}

		// Extract some info from the symbol table placed there by the REST
		// handler (if we are running as a service). If we're not running in
		// a service, these will be empty.
		if u, found := s.Get(defs.UserVariable); found {
			user = data.String(u)
		}

		if sv, found := s.Get(defs.SessionVariable); found {
			session, _ = data.Int(sv)
		}

		dsn, err := dsns.Lookup(session, user, name)
		if err != nil {
			return data.NewList(nil, err), err
		}

		connStr, err = dsns.Connection(dsn)
		if err != nil {
			return data.NewList(nil, err), err
		}

		// If this is a sqlite3/sqlite database, strip off the URL scheme from
		// the connection string and normalize the driver name to "sqlite".
		driverType = strings.ToLower(dsn.Provider)
		if driverType == defs.DeprecatedSqliteProvider || driverType == defs.SqliteProvider {
			connStr = strings.TrimPrefix(connStr, driverType+"://")
			driverType = defs.SqliteProvider
		}
	}

	// Normalize the user-facing "sqlite3" alias to the modernc.org/sqlite
	// driver name "sqlite" for all code paths below.
	if driverType == defs.DeprecatedSqliteProvider {
		driverType = defs.SqliteProvider
	}

	if driverType == defs.SqliteProvider || driverType == defs.DeprecatedSqliteProvider {
		// Make sure we are not talking to the credentials database. Code running in a
		// user-supplied service (or via the dashboard code tab) runs in the context of
		// the server. We don't want to allow such code to talk to the credentials database.
		// The only time we care about this is when it's a sqlite3 database since file
		// system protections won't suffice in this scenario. For postgres, the database
		// credential authorization protects us.
		requestedBaseName := filepath.Base(connStr)

		configPath := settings.Get("ego.server.userdata")
		if configPath == "" {
			configPath = defs.DefaultUserdataFileName
		} else if strings.HasPrefix(strings.ToLower(configPath), "sqlite3://") {
			configPath = strings.TrimPrefix(configPath, "sqlite3://")
		} else if strings.HasPrefix(strings.ToLower(configPath), "sqlite://") {
			configPath = strings.TrimPrefix(configPath, "sqlite://")
		}

		if strings.EqualFold(requestedBaseName, filepath.Base(configPath)) {
			err := errors.ErrNoPrivilegeForOperation.Context(connStr)

			return data.NewList(nil, err), err
		}
	}

	db, err := goSQL.Open(driverType, connStr)
	if err != nil {
		return data.NewList(nil, errors.New(err)), errors.New(err)
	}

	if driverType == defs.SqliteProvider {
		db.Exec("PRAGMA journal_mode=WAL;")
		db.Exec("PRAGMA busy_timeout=5000;")
	}

	// If the connection string had a password in URL format, blank it out now before we log it.
	if url, err2 := url.Parse(connStr); err2 == nil {
		if _, found := url.User.Password(); found {
			connStr = url.Redacted()
		}
	}

	if ui.IsActive(ui.DBLogger) {
		ui.Log(ui.DBLogger, "db.connect", ui.A{
			"constr": redactURLString(connStr)})
	}

	_ = s.Set(Database.Name(), Database)

	result := data.NewStruct(Database).
		FromBuiltinPackage().
		SetAlways(clientFieldName, db).
		SetAlways(constrFieldName, connStr).
		SetAlways(asStructFieldName, false).
		SetAlways(rowCountFieldName, 0).
		SetReadonly(true)

	return data.NewList(result, nil), nil
}

// asStructures implements the sql.Database.AsStruct(flag bool) method. It sets
// the asStruct field on the Database struct which controls how subsequent
// Query() and QueryResult() calls format their results:
//
//   - false (default) — each row is a *data.Array of column values in the
//     same order as the SELECT list; callers use integer indices
//   - true            — each row is a *data.Struct whose field names match
//     the column names; callers use field-name access
//
// The method returns the Database struct itself so Ego code can chain calls.
//
// Returns an error (e.g. ErrNoFunctionReceiver, ErrDatabaseClientClosed) if
// called on a closed or invalid connection.
func asStructures(s *symbols.SymbolTable, args data.List) (any, error) {
	if _, _, err := client(s); err != nil {
		return data.NewList(nil, err), err
	}

	this := getThis(s)
	this.SetAlways(asStructFieldName, data.BoolOrFalse(args.Get(0)))

	return data.NewList(this, nil), nil
}

// closeConnection implements the sql.Database.Close() method. It rolls back any
// active transaction, closes the underlying *goSQL.DB, and then zeroes all
// fields on the Database struct. Zeroing the fields achieves two goals:
//
//  1. It prevents accidental re-use of the connection — subsequent calls
//     to client() will find clientFieldName == nil and return an error.
//  2. It releases the references to native objects so the garbage collector
//     can reclaim them.
//
// Returns (nil error) on success; (error) if the rollback or close failed.
func closeConnection(s *symbols.SymbolTable, args data.List) (any, error) {
	db, tx, err := client(s)
	if err != nil {
		return data.NewList(err), err
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

	return data.NewList(err), err
}

// client is an internal helper that extracts the *goSQL.DB and optional *goSQL.Tx
// from the symbol table's receiver (__this). The lookup chain is:
//
//  1. Read defs.ThisVariable from the symbol table — must be *data.Struct
//  2. Read clientFieldName from that struct — must be a non-nil *goSQL.DB
//  3. Optionally read transactionFieldName — if non-nil, unwrap and cast to
//     *goSQL.Tx so the caller can use tx.Exec / tx.Query when inside a
//     transaction
//
// Errors returned:
//   - ErrDatabaseClientClosed — client field is a typed nil *goSQL.DB
//   - ErrNoFunctionReceiver   — __this is missing, wrong type, or
//     clientFieldName is nil/not a *goSQL.DB
func client(symbols *symbols.SymbolTable) (*goSQL.DB, *goSQL.Tx, error) {
	if g, ok := symbols.Get(defs.ThisVariable); ok {
		if gc, ok := g.(*data.Struct); ok {
			if client := gc.GetAlways(clientFieldName); client != nil {
				if cp, ok := client.(*goSQL.DB); ok {
					if cp == nil {
						return nil, nil, errors.ErrDatabaseClientClosed
					}

					tx, _ := data.UnWrap(gc.GetAlways(transactionFieldName))
					if tx == nil {
						return cp, nil, nil
					}

					return cp, tx.(*goSQL.Tx), nil
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
