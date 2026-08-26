package tables

// End-to-end tests for the "pkey"/"ptable" analysis added to @sql's RowSet
// response (see sql_pkey.go). Unlike sql_pkey_test.go's pure-function unit
// tests, these exercise SQLTransaction directly against a real SQLite file,
// so they also cover sqliteSingleColumnUniqueKeys' PRAGMA-based catalog
// queries -- in particular the INTEGER PRIMARY KEY (rowid-alias) case,
// which getSqliteColumnMetadata (describe.go) is known not to handle the
// same way.

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/tucats/ego/internal/caches"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/dsns"
	"github.com/tucats/ego/internal/router"
	"github.com/tucats/ego/internal/server/tables/database"
	"github.com/tucats/ego/internal/sqlparse/ast"

	_ "modernc.org/sqlite"
)

// setUpSQLPkeyTest wires an unrestricted SQLite-backed DSN "d1" and returns
// a function that posts a []string of SQL statements to SQLTransaction as
// the admin user, decoding a successful rows+json response into a
// defs.DBRowSet.
func setUpSQLPkeyTest(t *testing.T) func(statements []string) (int, defs.DBRowSet) {
	t.Helper()

	dataFile := "testing-sql-pkey-data-" + uuid.New().String() + ".db"

	t.Cleanup(func() {
		_ = os.Remove(dataFile)
		_ = os.Remove(dataFile + "-wal")
		_ = os.Remove(dataFile + "-shm")
	})

	svc, err := dsns.NewFileService(defs.MemoryProvider)
	if err != nil {
		t.Fatalf("create test DSN service: %v", err)
	}

	dsns.DSNService = svc

	if err := dsns.DSNService.WriteDSN(1, "admin", defs.DSN{
		Name:     "d1",
		Provider: defs.SqliteProvider,
		Database: dataFile,
	}); err != nil {
		t.Fatalf("write DSN: %v", err)
	}

	admin := &router.Session{ID: 1, User: "admin", Admin: true}

	post := func(statements []string) (int, defs.DBRowSet) {
		t.Helper()

		body, err := json.Marshal(statements)
		if err != nil {
			t.Fatalf("marshal statements: %v", err)
		}

		req, err := http.NewRequest(http.MethodPost, "/dsns/d1/tables/@sql", bytes.NewReader(body))
		if err != nil {
			t.Fatalf("http.NewRequest: %v", err)
		}

		admin.URLParts = map[string]any{"dsn": "d1"}

		rr := httptest.NewRecorder()
		status := SQLTransaction(admin, rr, req)

		var rowSet defs.DBRowSet

		if status == http.StatusOK {
			if err := json.Unmarshal(rr.Body.Bytes(), &rowSet); err != nil {
				t.Fatalf("unmarshal response: %v -- body: %s", err, rr.Body.String())
			}
		}

		return status, rowSet
	}

	return post
}

// TestSQLTransaction_PKey_IntegerPrimaryKey covers the case
// getSqliteColumnMetadata (describe.go) is known to miss: a single-column
// INTEGER PRIMARY KEY, which sqlite aliases to its own rowid and so never
// gets a separate entry in PRAGMA index_list.
func TestSQLTransaction_PKey_IntegerPrimaryKey(t *testing.T) {
	post := setUpSQLPkeyTest(t)

	status, rowSet := post([]string{
		`CREATE TABLE items (id INTEGER PRIMARY KEY, name TEXT)`,
		`INSERT INTO items (id, name) VALUES (1, 'widget')`,
		`SELECT id, name FROM items`,
	})

	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d", status)
	}

	if rowSet.PKey != "id" {
		t.Errorf("PKey = %q, want %q", rowSet.PKey, "id")
	}

	if rowSet.PTable != "items" {
		t.Errorf("PTable = %q, want %q", rowSet.PTable, "items")
	}
}

// TestSQLTransaction_PKey_StarSelect confirms "SELECT * FROM t" -- the most
// common client shape -- also resolves pkey/ptable.
func TestSQLTransaction_PKey_StarSelect(t *testing.T) {
	post := setUpSQLPkeyTest(t)

	status, rowSet := post([]string{
		`CREATE TABLE items (id INTEGER PRIMARY KEY, name TEXT)`,
		`INSERT INTO items (id, name) VALUES (1, 'widget')`,
		`SELECT * FROM items`,
	})

	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d", status)
	}

	if rowSet.PKey != "id" {
		t.Errorf("PKey = %q, want %q", rowSet.PKey, "id")
	}

	if rowSet.PTable != "items" {
		t.Errorf("PTable = %q, want %q", rowSet.PTable, "items")
	}
}

// TestSQLTransaction_PKey_ExplicitUniqueConstraint covers a UNIQUE
// constraint on a non-primary-key column, which sqlite does back with a
// separate single-column index (unlike the INTEGER PRIMARY KEY case above).
func TestSQLTransaction_PKey_ExplicitUniqueConstraint(t *testing.T) {
	post := setUpSQLPkeyTest(t)

	status, rowSet := post([]string{
		`CREATE TABLE users (id INTEGER PRIMARY KEY, email TEXT UNIQUE, name TEXT)`,
		`INSERT INTO users (id, email, name) VALUES (1, 'a@example.com', 'Alice')`,
		`SELECT email, name FROM users`,
	})

	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d", status)
	}

	// id (the primary key) isn't in the SELECT list at all, so the only
	// qualifying candidate is email, a plain UNIQUE column.
	if rowSet.PKey != "email" {
		t.Errorf("PKey = %q, want %q", rowSet.PKey, "email")
	}

	if rowSet.PTable != "users" {
		t.Errorf("PTable = %q, want %q", rowSet.PTable, "users")
	}
}

// TestSQLTransaction_PKey_PrefersPrimaryKeyOverUnique confirms that when
// both the primary key and a plain UNIQUE column are present in the SELECT
// list, the primary key wins.
func TestSQLTransaction_PKey_PrefersPrimaryKeyOverUnique(t *testing.T) {
	post := setUpSQLPkeyTest(t)

	status, rowSet := post([]string{
		`CREATE TABLE users (id INTEGER PRIMARY KEY, email TEXT UNIQUE, name TEXT)`,
		`INSERT INTO users (id, email, name) VALUES (1, 'a@example.com', 'Alice')`,
		`SELECT email, id, name FROM users`,
	})

	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d", status)
	}

	if rowSet.PKey != "id" {
		t.Errorf("PKey = %q, want %q", rowSet.PKey, "id")
	}
}

// TestSQLTransaction_PKey_NoUniqueColumn confirms a table with no primary
// key or unique constraint at all reports no pkey, though ptable is still
// resolved (the table is still unambiguous).
func TestSQLTransaction_PKey_NoUniqueColumn(t *testing.T) {
	post := setUpSQLPkeyTest(t)

	status, rowSet := post([]string{
		`CREATE TABLE events (kind TEXT, payload TEXT)`,
		`INSERT INTO events (kind, payload) VALUES ('click', '{}')`,
		`SELECT kind, payload FROM events`,
	})

	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d", status)
	}

	if rowSet.PKey != "" {
		t.Errorf("PKey = %q, want empty", rowSet.PKey)
	}

	if rowSet.PTable != "events" {
		t.Errorf("PTable = %q, want %q", rowSet.PTable, "events")
	}
}

// TestSQLTransaction_PKey_CompositeKeyColumnNotReported confirms that a
// column which is only unique as part of a multi-column (composite) UNIQUE
// constraint is never reported as pkey -- knowing just that one column's
// value is not enough, on its own, to identify a single row.
func TestSQLTransaction_PKey_CompositeKeyColumnNotReported(t *testing.T) {
	post := setUpSQLPkeyTest(t)

	status, rowSet := post([]string{
		`CREATE TABLE pairs (a TEXT, b TEXT, UNIQUE(a, b))`,
		`INSERT INTO pairs (a, b) VALUES ('x', 'y')`,
		`SELECT a, b FROM pairs`,
	})

	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d", status)
	}

	if rowSet.PKey != "" {
		t.Errorf("PKey = %q, want empty", rowSet.PKey)
	}

	if rowSet.PTable != "pairs" {
		t.Errorf("PTable = %q, want %q", rowSet.PTable, "pairs")
	}
}

// TestSQLTransaction_PKey_JoinDisqualifies confirms a multi-table SELECT
// (here, a JOIN) reports neither pkey nor ptable, even though each side of
// the join has its own primary key.
func TestSQLTransaction_PKey_JoinDisqualifies(t *testing.T) {
	post := setUpSQLPkeyTest(t)

	status, rowSet := post([]string{
		`CREATE TABLE a (id INTEGER PRIMARY KEY, label TEXT)`,
		`CREATE TABLE b (id INTEGER PRIMARY KEY, a_id INTEGER)`,
		`INSERT INTO a (id, label) VALUES (1, 'x')`,
		`INSERT INTO b (id, a_id) VALUES (1, 1)`,
		`SELECT a.id, b.id FROM a JOIN b ON a.id = b.a_id`,
	})

	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d", status)
	}

	if rowSet.PKey != "" {
		t.Errorf("PKey = %q, want empty", rowSet.PKey)
	}

	if rowSet.PTable != "" {
		t.Errorf("PTable = %q, want empty", rowSet.PTable)
	}
}

// TestSQLTransaction_PKey_AliasedColumnNotReported confirms that aliasing
// the sole qualifying column out of its real name disqualifies it: a client
// filtering a later UPDATE/DELETE needs the real column name, not a display
// alias, so an alias is never reported as pkey even when the underlying
// column is the table's primary key.
func TestSQLTransaction_PKey_AliasedColumnNotReported(t *testing.T) {
	post := setUpSQLPkeyTest(t)

	status, rowSet := post([]string{
		`CREATE TABLE items (id INTEGER PRIMARY KEY, name TEXT)`,
		`INSERT INTO items (id, name) VALUES (1, 'widget')`,
		`SELECT id AS item_id, name FROM items`,
	})

	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d", status)
	}

	if rowSet.PKey != "" {
		t.Errorf("PKey = %q, want empty", rowSet.PKey)
	}

	if rowSet.PTable != "items" {
		t.Errorf("PTable = %q, want %q", rowSet.PTable, "items")
	}
}

// TestSQLTransaction_PKey_CacheInvalidatedByIndexChange is a regression test
// for singleColumnUniqueKeys' caches.SchemaCache entry (sql_pkey.go) going
// stale after CREATE UNIQUE INDEX in a later, separate @sql request.
//
// Before isSchemaAlteringKind (sql_permissions.go) replaced the old
// parsing.IsSchemaAlteringStatement text-prefix check, a CREATE INDEX/DROP
// INDEX statement never set cacheFlush, so caches.SchemaCache was never
// purged after one -- meaning a cached "no unique column" answer from
// before the index existed would have kept being served after it was
// created. This creates a table with no unique column, runs a SELECT (which
// caches that answer), adds a unique index in a separate request, and
// confirms a repeat of the same SELECT reports the new index rather than
// the stale cached answer.
func TestSQLTransaction_PKey_CacheInvalidatedByIndexChange(t *testing.T) {
	// The schema cache is a package-level global shared by every test in
	// this file; start from a clean slate regardless of what ran before.
	caches.Purge(caches.SchemaCache)

	post := setUpSQLPkeyTest(t)

	status, rowSet := post([]string{
		`CREATE TABLE pkey_cache_probe (id INTEGER, label TEXT)`,
		`INSERT INTO pkey_cache_probe (id, label) VALUES (1, 'a')`,
		`SELECT id, label FROM pkey_cache_probe`,
	})

	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d", status)
	}

	if rowSet.PKey != "" {
		t.Fatalf("PKey = %q before any unique index exists, want empty", rowSet.PKey)
	}

	status, _ = post([]string{
		`CREATE UNIQUE INDEX idx_pkey_cache_probe_label ON pkey_cache_probe (label)`,
	})

	if status != http.StatusOK {
		t.Fatalf("expected 200 from CREATE UNIQUE INDEX, got %d", status)
	}

	status, rowSet = post([]string{
		`SELECT id, label FROM pkey_cache_probe`,
	})

	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d", status)
	}

	if rowSet.PKey != "label" {
		t.Errorf("PKey = %q after CREATE UNIQUE INDEX, want %q (cache was not invalidated)", rowSet.PKey, "label")
	}
}

// TestSingleColumnUniqueKeys_ServesFromCache proves singleColumnUniqueKeys
// (sql_pkey.go) actually short-circuits the database round-trip on a repeat
// call, rather than merely never returning a wrong answer (which
// TestSQLTransaction_PKey_CacheInvalidatedByIndexChange above already
// covers, but would pass even with no caching at all).
//
// It drops the unique index directly against the database handle -- not
// through any Ego DDL path, so none of Ego's own cache-purge machinery ever
// runs -- and confirms a second singleColumnUniqueKeys call still reports
// the first call's (now stale) answer. Only caches.Purge, called explicitly
// at the end, makes the fresh answer visible.
func TestSingleColumnUniqueKeys_ServesFromCache(t *testing.T) {
	caches.Purge(caches.SchemaCache)

	dbname := "testing-pkey-cache-hit-" + uuid.New().String() + ".db"

	handle, err := sql.Open("sqlite", dbname)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}

	t.Cleanup(func() {
		_ = handle.Close()
		_ = os.Remove(dbname)
	})

	if _, err := handle.Exec(`CREATE TABLE probe (id INTEGER, label TEXT)`); err != nil {
		t.Fatalf("create table: %v", err)
	}

	if _, err := handle.Exec(`CREATE UNIQUE INDEX idx_probe_label ON probe (label)`); err != nil {
		t.Fatalf("create index: %v", err)
	}

	db := &database.Database{Handle: handle, Provider: defs.SqliteProvider, User: "admin", DSN: "cache-hit-test-dsn"}
	ref := &ast.TableRef{Name: "probe"}

	_, unique, err := singleColumnUniqueKeys(nil, db, ref)
	if err != nil {
		t.Fatalf("singleColumnUniqueKeys: %v", err)
	}

	if !unique["label"] {
		t.Fatalf("expected label to be reported unique before the drop, got %v", unique)
	}

	if _, err := handle.Exec(`DROP INDEX idx_probe_label`); err != nil {
		t.Fatalf("drop index: %v", err)
	}

	_, unique, err = singleColumnUniqueKeys(nil, db, ref)
	if err != nil {
		t.Fatalf("singleColumnUniqueKeys (should be served from cache): %v", err)
	}

	if !unique["label"] {
		t.Errorf("cache was not used: expected the stale pre-drop answer (label still unique), got %v", unique)
	}

	caches.Purge(caches.SchemaCache)

	_, unique, err = singleColumnUniqueKeys(nil, db, ref)
	if err != nil {
		t.Fatalf("singleColumnUniqueKeys (after purge): %v", err)
	}

	if unique["label"] {
		t.Errorf("expected label to no longer be reported unique after the cache was purged, got %v", unique)
	}
}
