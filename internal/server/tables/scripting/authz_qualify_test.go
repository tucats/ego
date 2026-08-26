package scripting

import (
	"strings"
	"testing"

	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/server/tables/database"
	"github.com/tucats/ego/internal/sqlparse"
)

// TestAuthorizeAndClassifySQL_PostgresQualifiesUnqualifiedTable is a
// regression test for the @transaction endpoint's "sql"/"readrows" raw-SQL
// opcodes (doSQL, doRows) previously executing the client's SQL text
// verbatim -- unlike every structured opcode and unlike the top-level @sql
// endpoint (tables/sql_permissions.go), which always schema-qualifies via
// parsing.FullName. authorizeAndClassifySQL must now return SQL rewritten so
// an unqualified table reference is pinned to db.User (the DSN's resolved
// schema -- see database.Open), for a PostgreSQL-provider db, so the caller
// executes that returned text instead of the original.
//
// db.Session is left nil deliberately: authorizeAndClassifySQL's own doc
// comment documents this as the supported way to bypass authorization in a
// test, exercising only the qualify-and-format behavior.
func TestAuthorizeAndClassifySQL_PostgresQualifiesUnqualifiedTable(t *testing.T) {
	db := &database.Database{Provider: defs.PostgresProvider, User: "myschema"}

	formatted, kind, _, status, err := authorizeAndClassifySQL(db, "INSERT INTO names (id) VALUES (1)")
	if err != nil {
		t.Fatalf("unexpected error: %v (status %d)", err, status)
	}

	if kind != sqlparse.StmtInsert {
		t.Errorf("kind = %v, want StmtInsert", kind)
	}

	if !strings.Contains(formatted, `"myschema"."names"`) {
		t.Errorf("formatted SQL = %q, want it to contain %q", formatted, `"myschema"."names"`)
	}
}

// TestAuthorizeAndClassifySQL_AlreadyQualifiedIsUntouched confirms a table
// reference the client already schema-qualified is left alone rather than
// being overwritten with db.User.
func TestAuthorizeAndClassifySQL_AlreadyQualifiedIsUntouched(t *testing.T) {
	db := &database.Database{Provider: defs.PostgresProvider, User: "myschema"}

	formatted, _, _, status, err := authorizeAndClassifySQL(db, "SELECT * FROM other.names")
	if err != nil {
		t.Fatalf("unexpected error: %v (status %d)", err, status)
	}

	if !strings.Contains(formatted, `"other"."names"`) {
		t.Errorf("formatted SQL = %q, want it to contain %q", formatted, `"other"."names"`)
	}

	if strings.Contains(formatted, "myschema") {
		t.Errorf("formatted SQL = %q, an explicitly-qualified reference must not be rewritten", formatted)
	}
}

// TestAuthorizeAndClassifySQL_SQLiteIsUntouched confirms SQLite -- which has
// no schema concept -- never gets a schema injected, since db.User there is
// the caller's Ego identity, not a real schema name.
func TestAuthorizeAndClassifySQL_SQLiteIsUntouched(t *testing.T) {
	db := &database.Database{Provider: defs.SqliteProvider, User: "admin"}

	formatted, _, _, status, err := authorizeAndClassifySQL(db, "SELECT * FROM names")
	if err != nil {
		t.Fatalf("unexpected error: %v (status %d)", err, status)
	}

	if strings.Contains(formatted, "admin") {
		t.Errorf("formatted SQL = %q, SQLite must not get a schema qualifier", formatted)
	}
}
