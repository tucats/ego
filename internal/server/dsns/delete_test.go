package dsns

// TestDeleteDSNHandler_CleansUpTablePermissions is a regression test for the
// gap where deleting a DSN left its table_perms grants behind in the system
// database: the grants were never removed, so a later DSN created with the
// same name would silently inherit the old table permissions.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/tucats/ego/internal/cli/settings"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/server/tables"
)

func TestDeleteDSNHandler_CleansUpTablePermissions(t *testing.T) {
	setupTestDSNService(t)

	dbFile := "testing-dsn-perms-" + uuid.New().String() + ".db"

	t.Cleanup(func() {
		_ = os.Remove(dbFile)
		_ = os.Remove(dbFile + "-wal")
		_ = os.Remove(dbFile + "-shm")
	})

	original := settings.Get(defs.LogonUserdataSetting)
	settings.Set(defs.LogonUserdataSetting, "sqlite://"+dbFile)

	t.Cleanup(func() { settings.Set(defs.LogonUserdataSetting, original) })

	for _, dsn := range []defs.DSN{
		{Name: "d1", Provider: defs.SqliteProvider, Database: "d1.db"},
		{Name: "d2", Provider: defs.SqliteProvider, Database: "d2.db"},
	} {
		rr := httptest.NewRecorder()
		status := CreateDSNHandler(makeDSNSession(nil), rr, newDSNRequest(t, http.MethodPost, dsn))

		if status != http.StatusCreated {
			t.Fatalf("create DSN %s: expected 201, got %d -- body: %s", dsn.Name, status, rr.Body.String())
		}
	}

	grantReadPermission := func(dsnName, tableName string) {
		t.Helper()

		session := makeDSNSession(map[string]any{"dsn": dsnName, "table": tableName})

		rr := httptest.NewRecorder()
		req := newDSNRequest(t, http.MethodPut, []string{defs.TableReadPermission})
		status := tables.GrantPermissions(session, rr, req)

		if status != http.StatusOK {
			t.Fatalf("grant permission for %s/%s: expected 200, got %d -- body: %s", dsnName, tableName, status, rr.Body.String())
		}
	}

	grantReadPermission("d1", "t1")
	grantReadPermission("d2", "t1")

	countPermissions := func(dsnName string) int {
		t.Helper()

		session := makeDSNSession(map[string]any{"dsn": dsnName})

		rr := httptest.NewRecorder()
		status := tables.ReadAllPermissions(session, rr, newDSNRequest(t, http.MethodGet, nil))

		if status != http.StatusOK {
			t.Fatalf("read permissions for %s: expected 200, got %d -- body: %s", dsnName, status, rr.Body.String())
		}

		var body defs.AllPermissionResponse
		if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode permissions response for %s: %v -- body: %s", dsnName, err, rr.Body.String())
		}

		return len(body.Permissions)
	}

	// Confirm both grants are visible before either DSN is deleted.
	if count := countPermissions("d1"); count != 1 {
		t.Fatalf("d1: expected 1 permission record before delete, got %d", count)
	}

	if count := countPermissions("d2"); count != 1 {
		t.Fatalf("d2: expected 1 permission record before delete, got %d", count)
	}

	// Deleting d1 must remove its table_perms row, and must not touch d2's.
	rr := httptest.NewRecorder()
	status := DeleteDSNHandler(makeDSNSession(map[string]any{"dsn": "d1"}), rr, newDSNRequest(t, http.MethodDelete, nil))

	if status != http.StatusOK {
		t.Fatalf("delete DSN d1: expected 200, got %d -- body: %s", status, rr.Body.String())
	}

	if count := countPermissions("d1"); count != 0 {
		t.Fatalf("d1: expected 0 permission records after delete, got %d", count)
	}

	if count := countPermissions("d2"); count != 1 {
		t.Fatalf("d2: expected 1 permission record after delete, got %d", count)
	}
}
