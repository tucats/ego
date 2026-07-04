package scripting

import (
	"database/sql"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/server/tables/database"

	_ "modernc.org/sqlite"
)

// openDropTestDatabase opens a fresh temporary SQLite file, creates a table
// with the given (already-safely-quoted, by the caller) name, and registers
// cleanup of the backing file when the test completes. It returns a
// database.Database wired up to use that connection, matching the shape
// doDrop expects.
func openDropTestDatabase(t *testing.T, createTableSQL string) *database.Database {
	t.Helper()

	dbname := "testing-drop-" + uuid.New().String() + ".db"

	handle, err := sql.Open("sqlite", dbname)
	if err != nil {
		t.Fatalf("error opening database: %v", err)
	}

	if _, err := handle.Exec(createTableSQL); err != nil {
		t.Fatalf("error creating table: %v", err)
	}

	t.Cleanup(func() {
		_ = handle.Close()
		_ = os.Remove(dbname)
	})

	return &database.Database{
		Handle:   handle,
		Provider: defs.SqliteProvider,
	}
}

// tableExists reports whether a table with the given literal name (already
// quoted by the caller if needed) is present in sqlite_schema.
func tableExists(t *testing.T, db *database.Database, name string) bool {
	t.Helper()

	rows, err := db.Handle.Query("SELECT name FROM sqlite_schema WHERE type='table' AND name=$1", name)
	if err != nil {
		t.Fatalf("error querying sqlite_schema: %v", err)
	}
	defer rows.Close()

	return rows.Next()
}

// TestDoDrop_SQLiteTableNameIsQuoted is a regression test for a real SQL
// injection: doDrop's SQLite branch used to build the identifier by naively
// wrapping task.Table in literal quote characters ("\"" + task.Table + "\""),
// so a table name containing a '"' could break out of the intended
// identifier. The table name must now be escaped with egostrings.SQLIdentifier
// (which doubles embedded '"' characters), so a name containing a quote is
// treated as a single, safely-escaped identifier instead of breaking the
// statement.
func TestDoDrop_SQLiteTableNameIsQuoted(t *testing.T) {
	// The table's real name, exactly as SQLIdentifier would render it:
	// literal text `evil" table` with the embedded quote doubled and the
	// whole thing wrapped in one pair of quotes.
	const rawName = `evil" table`

	db := openDropTestDatabase(t, `CREATE TABLE "evil"" table" (id INTEGER)`)

	syms := &symbolTable{symbols: map[string]any{}}
	task := defs.TXOperation{Opcode: "drop", Table: rawName}

	status, err := doDrop(1, "admin", db, task, 0, syms)
	if err != nil {
		t.Fatalf("doDrop() unexpected error: %v (status %d)", err, status)
	}

	if tableExists(t, db, rawName) {
		t.Errorf("doDrop() did not drop the table named %q", rawName)
	}
}
