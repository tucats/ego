package tables

import (
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/server/tables/database"
	"github.com/tucats/ego/internal/server/tables/scripting"
	"github.com/tucats/ego/internal/sqlparse"
)

// Wire this package's Authorized (security.go), uniqueKeyLookup
// (sql_pkey.go), and ensureSchemaExists (tables.go) into the scripting
// package at startup. scripting cannot import tables directly -- tables
// already imports scripting (routes.go registers scripting.Handler for the
// @transaction endpoint) -- so the dependency is injected in this direction
// instead. This init runs whenever tables is imported, which every server
// binary does unconditionally, so scripting.AuthorizedFunc,
// scripting.UniqueKeyLookupFunc, and scripting.EnsureSchemaFunc are always
// populated before any request can be routed. See scripting's authz.go for
// the consumer side.
func init() {
	scripting.AuthorizedFunc = Authorized
	scripting.UniqueKeyLookupFunc = func(db *database.Database) sqlparse.UniqueKeyLookup {
		return uniqueKeyLookup(db.Session, db)
	}
	scripting.EnsureSchemaFunc = func(db *database.Database) error {
		if db.Provider != defs.PostgresProvider {
			return nil
		}

		return ensureSchemaExists(db, db.User)
	}
}
