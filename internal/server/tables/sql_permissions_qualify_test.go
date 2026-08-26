package tables

import (
	"net/http"
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

// TestAuthorizeAndFormatStatements_RestrictSchemaRejectsOtherSchema confirms
// that a DSN with an explicitly configured schema (db.RestrictSchema) turns
// away raw SQL that names any other schema -- including a caller trying to
// reach a PostgreSQL system schema like pg_catalog by spelling it out --
// rather than letting the statement execute against a schema outside the
// DSN's own sandbox. This applies even to an admin caller, since the schema
// boundary belongs to the DSN, not to a per-user permission.
func TestAuthorizeAndFormatStatements_RestrictSchemaRejectsOtherSchema(t *testing.T) {
	session := &router.Session{ID: 1, User: "admin", Admin: true}
	db := &database.Database{Provider: defs.PostgresProvider, User: "myschema", RestrictSchema: true, DSN: "d1", Session: session}

	rr := httptest.NewRecorder()

	_, _, status := authorizeAndFormatStatements(session, db, []string{"SELECT * FROM pg_catalog.pg_tables"}, rr)
	if status != http.StatusForbidden {
		t.Fatalf("status = %d, want %d, body: %s", status, http.StatusForbidden, rr.Body.String())
	}
}

// TestAuthorizeAndFormatStatements_RestrictSchemaAllowsOwnSchema confirms
// that a DSN with an explicitly configured schema still executes SQL that
// either leaves the schema unqualified or names that same schema explicitly.
func TestAuthorizeAndFormatStatements_RestrictSchemaAllowsOwnSchema(t *testing.T) {
	session := &router.Session{ID: 1, User: "admin", Admin: true}
	db := &database.Database{Provider: defs.PostgresProvider, User: "myschema", RestrictSchema: true, DSN: "d1", Session: session}

	rr := httptest.NewRecorder()

	formatted, _, status := authorizeAndFormatStatements(session, db, []string{"SELECT * FROM myschema.names"}, rr)
	if status > 200 {
		t.Fatalf("unexpected status %d, body: %s", status, rr.Body.String())
	}

	if len(formatted) != 1 || !strings.Contains(formatted[0], `"myschema"."names"`) {
		t.Errorf("formatted = %v, want it to contain %q", formatted, `"myschema"."names"`)
	}
}

// TestAuthorizeAndFormatStatements_UnrestrictedSchemaAllowsOtherSchema
// confirms that a DSN with no schema of its own (db.RestrictSchema == false)
// keeps permitting an explicit reference to any schema, unchanged from
// before this restriction existed.
func TestAuthorizeAndFormatStatements_UnrestrictedSchemaAllowsOtherSchema(t *testing.T) {
	session := &router.Session{ID: 1, User: "public", Admin: true}
	db := &database.Database{Provider: defs.PostgresProvider, User: "public", DSN: "d1", Session: session}

	rr := httptest.NewRecorder()

	formatted, _, status := authorizeAndFormatStatements(session, db, []string{"SELECT * FROM other.names"}, rr)
	if status > 200 {
		t.Fatalf("unexpected status %d, body: %s", status, rr.Body.String())
	}

	if len(formatted) != 1 || !strings.Contains(formatted[0], `"other"."names"`) {
		t.Errorf("formatted = %v, want it to contain %q", formatted, `"other"."names"`)
	}
}
