package tables

import (
	"database/sql"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/router"
	"github.com/tucats/ego/internal/server/tables/database"

	_ "modernc.org/sqlite"
)

// TestGetTableNames_SQLiteTableNameIsQuoted is a regression test for a
// real bug/vulnerability: getTableNames used to build the zero-row 
// "count the columns" query by concatenating the table name between
// literal quote characters ("SELECT * FROM \"" + name + "\" WHERE 1=0"). 
// The valeue name comes from the database's own catalog, but a table
// created via the admin-only "@sql" raw-SQL endpoint can legally have
// a '"' embedded in its name (e.g. CREATE TABLE "a""b" (...)), so a
// crafted table name could break the intended identifier quoting. name must
// now be run through egostrings.SQLIdentifier instead, which safely
// doubles the embedded '"'.
//
// This is verified end to end: a real table is created with a '"' in its
// name, and getTableNames is fed a single-row *sql.Rows containing that
// exact name (standing in for a row scanned from sqlite_schema). Before the
// fix, the resulting "SELECT * FROM ..." query was malformed SQL and failed,
// causing the table to be silently skipped (count == 0, an empty result).
// After the fix, the table's columns are counted correctly.
func TestGetTableNames_SQLiteTableNameIsQuoted(t *testing.T) {
	const rawName = `evil" table`

	dbname := "testing-list-" + uuid.New().String() + ".db"

	handle, err := sql.Open("sqlite", dbname)
	if err != nil {
		t.Fatalf("error opening database: %v", err)
	}

	t.Cleanup(func() {
		_ = handle.Close()
		_ = os.Remove(dbname)
	})

	if _, err := handle.Exec(`CREATE TABLE "evil"" table" (id INTEGER, name TEXT)`); err != nil {
		t.Fatalf("error creating table: %v", err)
	}

	db := &database.Database{
		Handle:   handle,
		Provider: defs.SqliteProvider,
		// Admin: true short-circuits the Authorized() permission check, which
		// needs a fully configured DSN/permissions subsystem this test has no
		// need to stand up -- it is irrelevant to the quoting fix being tested.
		Session: &router.Session{Admin: true, User: "admin", ID: 1},
	}

	// Stand in for a single row scanned from sqlite_schema: a query that
	// returns exactly one row containing the crafted table name.
	rows, err := handle.Query("SELECT $1 AS name", rawName)
	if err != nil {
		t.Fatalf("error building stand-in rows: %v", err)
	}

	var name string

	tables, count, err, status := getTableNames(rows, name, db, "", false, nil)
	if err != nil {
		t.Fatalf("getTableNames() unexpected error: %v (status %d)", err, status)
	}

	if count != 1 {
		t.Fatalf("getTableNames() count = %d, want 1 (table was likely skipped due to a malformed query)", count)
	}

	if len(tables) != 1 {
		t.Fatalf("getTableNames() returned %d tables, want 1", len(tables))
	}

	if tables[0].Name != rawName {
		t.Errorf("getTableNames() table name = %q, want %q", tables[0].Name, rawName)
	}

	if tables[0].Columns != 2 {
		t.Errorf("getTableNames() column count = %d, want 2 (id, name)", tables[0].Columns)
	}
}
