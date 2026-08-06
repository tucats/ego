package scripting

// Tests for the REST-3 audit's tables residual-gaps section
// (docs/issues/REST-3.md, section 7): sibling @transaction opcodes that
// REST-1 fixed in some files but not others now agree with each other on
// how a missing table is classified.

import (
	"database/sql"
	"net/http"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/server/tables/database"

	_ "modernc.org/sqlite"
)

// openStatusTestDatabase opens a fresh temporary SQLite database with no
// tables in it -- exactly what's needed to provoke a "missing table"
// failure from any opcode without constructing driver-specific errors by
// hand (mirroring the technique already used in drop_test.go).
func openStatusTestDatabase(t *testing.T) *database.Database {
	t.Helper()

	dbname := "testing-status-" + uuid.New().String() + ".db"

	handle, err := sql.Open("sqlite", dbname)
	if err != nil {
		t.Fatalf("error opening database: %v", err)
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

// TestDoDelete_MissingTable_ReturnsNotFound guards 7.1: the delete opcode's
// db.Exec failure was hardcoded to 400, unlike doDrop in this same package,
// which already routed through dberrors.ExecStatus and so correctly
// answered 404 for the identical condition.
func TestDoDelete_MissingTable_ReturnsNotFound(t *testing.T) {
	db := openStatusTestDatabase(t)
	syms := &symbolTable{symbols: map[string]any{}}
	task := defs.TXOperation{Opcode: "delete", Table: "nosuchtable", Filters: []string{"id", "eq", "1"}}

	_, status, err := doDelete(1, "admin", db, task, 0, syms)
	if err == nil {
		t.Fatal("expected an error for a delete against a missing table, got nil")
	}

	if status != http.StatusNotFound {
		t.Errorf("status = %d, want 404 -- err: %v", status, err)
	}
}

// TestDoSQL_MissingTable_ReturnsNotFound guards 7.2: the @transaction "sql"
// opcode's db.Exec failure was hardcoded to 400, distinct from (and with the
// same bug as) the already-fixed top-level tables/sql.go @sql handler.
func TestDoSQL_MissingTable_ReturnsNotFound(t *testing.T) {
	db := openStatusTestDatabase(t)
	syms := &symbolTable{symbols: map[string]any{}}
	task := defs.TXOperation{Opcode: "sql", SQL: "DELETE FROM nosuchtable WHERE id = 1"}

	_, status, _, err := doSQL(1, db, task, 0, syms)
	if err == nil {
		t.Fatal("expected an error for SQL against a missing table, got nil")
	}

	if status != http.StatusNotFound {
		t.Errorf("status = %d, want 404 -- err: %v", status, err)
	}
}

// TestDoRows_MissingTable_ReturnsNotFound guards 7.3: the "readrows"
// opcode's db.Query failure set status = 400 directly, where the
// nearly-identical "select" opcode (doSelect) already called
// dberrors.ExecStatus for the same failure.
func TestDoRows_MissingTable_ReturnsNotFound(t *testing.T) {
	db := openStatusTestDatabase(t)
	syms := &symbolTable{symbols: map[string]any{}}
	task := defs.TXOperation{Opcode: "readrows", SQL: "SELECT * FROM nosuchtable"}

	_, status, err := doRows(1, "admin", db, task, 0, syms)
	if err == nil {
		t.Fatal("expected an error for readrows against a missing table, got nil")
	}

	if status != http.StatusNotFound {
		t.Errorf("status = %d, want 404 -- err: %v", status, err)
	}
}

// TestDoSelect_MissingTable_ReturnsNotFound is not a regression test (the
// select opcode already routed through dberrors.ExecStatus before this
// audit) -- it pins down the behavior doRows above is now brought in line
// with, so the two can't silently drift apart again.
func TestDoSelect_MissingTable_ReturnsNotFound(t *testing.T) {
	db := openStatusTestDatabase(t)
	syms := &symbolTable{symbols: map[string]any{}}
	task := defs.TXOperation{Opcode: "select", Table: "nosuchtable"}

	_, status, err := doSelect(1, "admin", db, task, 0, syms)
	if err == nil {
		t.Fatal("expected an error for select against a missing table, got nil")
	}

	if status != http.StatusNotFound {
		t.Errorf("status = %d, want 404 -- err: %v", status, err)
	}
}

// TestDoRows_MoreThanOneRow_Succeeds is a sanity check that the "readrows"
// opcode (unlike doSelect) legitimately accepts any number of matching rows
// -- confirming the doSelect-specific "more than one row is an error" fix
// below was not accidentally applied here too.
func TestDoRows_MoreThanOneRow_Succeeds(t *testing.T) {
	db := openStatusTestDatabase(t)

	if _, err := db.Handle.Exec(`CREATE TABLE t (id INTEGER)`); err != nil {
		t.Fatalf("create table: %v", err)
	}

	if _, err := db.Handle.Exec(`INSERT INTO t (id) VALUES (1), (2), (3)`); err != nil {
		t.Fatalf("seed rows: %v", err)
	}

	syms := &symbolTable{symbols: map[string]any{}}
	task := defs.TXOperation{Opcode: "readrows", SQL: "SELECT * FROM t"}

	count, status, err := doRows(1, "admin", db, task, 0, syms)
	if err != nil {
		t.Fatalf("unexpected error: %v (status %d)", err, status)
	}

	if status != http.StatusOK {
		t.Errorf("status = %d, want 200", status)
	}

	if count != 3 {
		t.Errorf("count = %d, want 3", count)
	}
}

// TestReadTxRowData_MoreThanOneRow_ReturnsBadRequest is a regression test
// for a bug found while fixing 7.4: readTxRowData's rowCount++ used to live
// inside the "if rowCount == 0" block that stores the first row's values,
// which meant rowCount could only ever reach 1 and never higher -- so a
// query matching more than one row was silently treated as success instead
// of the documented error.
//
// This calls readTxRowData directly with a raw multi-row-matching query,
// rather than going through doSelect: doSelect's caller always builds its
// query with "limit=1" (see the fakeURL comment above readTxRowData), which
// the query builder turns into a literal SQL "LIMIT 1" -- so the database
// itself never returns more than one row to doSelect regardless of this
// counting fix, a separate, deeper, out-of-REST-3-scope bug (the ambiguity
// check needs LIMIT 2, not LIMIT 1, to have a second row to object to).
// Exercising readTxRowData directly, with a query this test controls, is
// what actually verifies the counting logic itself is now correct.
func TestReadTxRowData_MoreThanOneRow_ReturnsBadRequest(t *testing.T) {
	db := openStatusTestDatabase(t)

	if _, err := db.Handle.Exec(`CREATE TABLE t (id INTEGER)`); err != nil {
		t.Fatalf("create table: %v", err)
	}

	if _, err := db.Handle.Exec(`INSERT INTO t (id) VALUES (1), (2)`); err != nil {
		t.Fatalf("seed rows: %v", err)
	}

	syms := &symbolTable{symbols: map[string]any{}}

	_, status, err := readTxRowData(db, "SELECT * FROM t", 1, syms, false)
	if err == nil {
		t.Fatal("expected an error for a query matching more than one row, got nil")
	}

	if status != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 -- err: %v", status, err)
	}
}
