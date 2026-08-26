package tables

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/router"
	"github.com/tucats/ego/internal/server/tables/database"
)

// TestAuthorizeAndFormatStatements_PostgresQualifiesUnqualifiedTable is a
// regression test for the top-level @sql endpoint (SQLTransaction, sql.go)
// executing an unqualified table reference verbatim -- letting the database
// connection's own default schema resolution (e.g. PostgreSQL's
// search_path) decide which schema the statement actually touched, instead
// of the DSN's own resolved schema (db.User -- see database.Open) the way
// every structured /rows and /tables endpoint already does via
// parsing.FullName. authorizeAndFormatStatements must now rewrite the
// statement so it executes against db.User's schema.
func TestAuthorizeAndFormatStatements_PostgresQualifiesUnqualifiedTable(t *testing.T) {
	session := &router.Session{ID: 1, User: "admin", Admin: true}
	db := &database.Database{Provider: defs.PostgresProvider, User: "myschema", DSN: "d1", Session: session}

	rr := httptest.NewRecorder()

	formatted, _, status := authorizeAndFormatStatements(session, db, []string{"INSERT INTO names (id) VALUES (1)"}, rr)
	if status > 200 {
		t.Fatalf("unexpected status %d, body: %s", status, rr.Body.String())
	}

	if len(formatted) != 1 || !strings.Contains(formatted[0], `"myschema"."names"`) {
		t.Errorf("formatted = %v, want it to contain %q", formatted, `"myschema"."names"`)
	}
}

// TestAuthorizeAndFormatStatements_SQLiteIsUntouched confirms SQLite -- which
// has no schema concept -- never gets a schema injected, since db.User there
// is the caller's Ego identity, not a real schema name.
func TestAuthorizeAndFormatStatements_SQLiteIsUntouched(t *testing.T) {
	session := &router.Session{ID: 1, User: "admin", Admin: true}
	db := &database.Database{Provider: defs.SqliteProvider, User: "admin", DSN: "d1", Session: session}

	rr := httptest.NewRecorder()

	formatted, _, status := authorizeAndFormatStatements(session, db, []string{"SELECT * FROM names"}, rr)
	if status > 200 {
		t.Fatalf("unexpected status %d, body: %s", status, rr.Body.String())
	}

	if len(formatted) != 1 || strings.Contains(formatted[0], `"admin"`) {
		t.Errorf("formatted = %v, SQLite must not get a schema qualifier", formatted)
	}
}
