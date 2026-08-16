package tables

// TestDeleteTable_CleansUpTablePermissions is a regression test for two bugs
// in DeleteTable's table_perms cleanup:
//
//  1. It only ran when dsnName == "". Since the route is
//     DELETE /dsns/{dsn}/tables/{table}, dsn is a required path segment, so
//     dsnName == "" just means "the default/baseline DSN slot" -- dropping a
//     table via any actual named DSN skipped cleanup entirely.
//  2. Even when it did run, it passed the FullName-qualified/quoted table
//     name (e.g. `"mytable"`), not the raw table name that
//     GrantPermissions/ReadPermissions/DeletePermissions store and filter
//     by -- so the delete filter never matched any row.
//
// This creates a real DSN-backed SQLite table, grants a permission on it (and
// on a second, untouched table in the same DSN), drops the table via
// DeleteTable, and confirms its table_perms row is gone while the other
// table's grant survives.

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/tucats/ego/internal/cli/settings"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/dsns"
	"github.com/tucats/ego/internal/router"

	_ "modernc.org/sqlite"
)

func TestDeleteTable_CleansUpTablePermissions(t *testing.T) {
	// A real DSN-backed SQLite database containing the table being dropped.
	dataFile := "testing-delete-data-" + uuid.New().String() + ".db"

	t.Cleanup(func() {
		_ = os.Remove(dataFile)
		_ = os.Remove(dataFile + "-wal")
		_ = os.Remove(dataFile + "-shm")
	})

	dataHandle, err := sql.Open("sqlite", dataFile)
	if err != nil {
		t.Fatalf("open data db: %v", err)
	}
	defer dataHandle.Close()

	if _, err := dataHandle.Exec(`CREATE TABLE t1 (id INTEGER)`); err != nil {
		t.Fatalf("create table t1: %v", err)
	}

	svc, err := dsns.NewFileService("memory")
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

	// A separate SQLite database standing in for the system catalog that
	// backs table_perms.
	permsFile := "testing-delete-perms-" + uuid.New().String() + ".db"

	t.Cleanup(func() {
		_ = os.Remove(permsFile)
		_ = os.Remove(permsFile + "-wal")
		_ = os.Remove(permsFile + "-shm")
	})

	original := settings.Get(defs.LogonUserdataSetting)
	settings.Set(defs.LogonUserdataSetting, "sqlite://"+permsFile)

	t.Cleanup(func() { settings.Set(defs.LogonUserdataSetting, original) })

	newRequest := func(t *testing.T, method string, body any) *http.Request {
		t.Helper()

		var bodyBytes []byte

		if body != nil {
			bodyBytes, err = json.Marshal(body)
			if err != nil {
				t.Fatalf("marshal request body: %v", err)
			}
		}

		req, err := http.NewRequest(method, "/dsns/d1/tables/t1", bytes.NewReader(bodyBytes))
		if err != nil {
			t.Fatalf("http.NewRequest: %v", err)
		}

		return req
	}

	grantReadPermission := func(tableName string) {
		t.Helper()

		session := &router.Session{ID: 1, User: "admin", Admin: true, URLParts: map[string]any{"dsn": "d1", "table": tableName}}

		rr := httptest.NewRecorder()
		status := GrantPermissions(session, rr, newRequest(t, http.MethodPut, []string{defs.TableReadPermission}))

		if status != http.StatusOK {
			t.Fatalf("grant permission for d1/%s: expected 200, got %d -- body: %s", tableName, status, rr.Body.String())
		}
	}

	grantReadPermission("t1")
	grantReadPermission("t2")

	countPermissions := func() int {
		t.Helper()

		session := &router.Session{ID: 1, User: "admin", Admin: true, URLParts: map[string]any{"dsn": "d1"}}

		rr := httptest.NewRecorder()
		status := ReadAllPermissions(session, rr, newRequest(t, http.MethodGet, nil))

		if status != http.StatusOK {
			t.Fatalf("read permissions for d1: expected 200, got %d -- body: %s", status, rr.Body.String())
		}

		var body defs.AllPermissionResponse
		if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode permissions response: %v -- body: %s", err, rr.Body.String())
		}

		return len(body.Permissions)
	}

	if count := countPermissions(); count != 2 {
		t.Fatalf("expected 2 permission records before delete, got %d", count)
	}

	session := &router.Session{ID: 1, User: "admin", Admin: true, URLParts: map[string]any{"dsn": "d1", "table": "t1"}}

	rr := httptest.NewRecorder()
	status := DeleteTable(session, rr, newRequest(t, http.MethodDelete, nil))

	if status != http.StatusOK {
		t.Fatalf("delete table t1: expected 200, got %d -- body: %s", status, rr.Body.String())
	}

	if count := countPermissions(); count != 1 {
		t.Fatalf("expected 1 permission record after delete (t2's), got %d", count)
	}
}
