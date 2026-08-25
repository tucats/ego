package tables

// TestReadTablePermissions verifies that the table-scoped permission listing
// (GET /dsns/{dsn}/tables/{table}/@permissions) reports every user's grants on
// the requested table -- unlike ReadPermissions, which is scoped to a single
// user -- while excluding grants on a different table in the same DSN.

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

func TestReadTablePermissions(t *testing.T) {
	permsFile := "testing-table-perms-" + uuid.New().String() + ".db"

	t.Cleanup(func() {
		_ = os.Remove(permsFile)
		_ = os.Remove(permsFile + "-wal")
		_ = os.Remove(permsFile + "-shm")
	})

	original := settings.Get(defs.LogonUserdataSetting)
	settings.Set(defs.LogonUserdataSetting, "sqlite://"+permsFile)
	t.Cleanup(func() { settings.Set(defs.LogonUserdataSetting, original) })

	// pValid/pHandle are package-level singletons shared by every test in
	// this package -- force a fresh handle so this test isn't silently
	// operating against another test's already-cleaned-up database file.
	pValid = false
	pHandle = nil

	t.Cleanup(func() { pValid = false; pHandle = nil })

	newRequest := func(t *testing.T, tableName string, body any) *http.Request {
		t.Helper()

		bodyBytes, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}

		req, err := http.NewRequest(http.MethodPut, "/dsns/d1/tables/"+tableName+"/permissions", bytes.NewReader(bodyBytes))
		if err != nil {
			t.Fatalf("http.NewRequest: %v", err)
		}

		return req
	}

	grant := func(user, tableName string, perms []string) {
		t.Helper()

		session := &router.Session{
			ID:         1,
			User:       "admin",
			Admin:      true,
			URLParts:   map[string]any{"dsn": "d1", "table": tableName},
			Parameters: map[string][]string{"user": {user}},
		}

		rr := httptest.NewRecorder()
		status := GrantPermissions(session, rr, newRequest(t, tableName, perms))

		if status != http.StatusOK {
			t.Fatalf("grant %v for %s on d1/%s: expected 200, got %d -- body: %s", perms, user, tableName, status, rr.Body.String())
		}
	}

	grant("alice", "t1", []string{defs.TableReadPermission, defs.TableWritePermission})
	grant("bob", "t1", []string{defs.TableReadPermission})
	grant("carol", "t2", []string{defs.TableReadPermission})

	session := &router.Session{ID: 1, User: "admin", Admin: true, URLParts: map[string]any{"dsn": "d1", "table": "t1"}}

	req, err := http.NewRequest(http.MethodGet, "/dsns/d1/tables/t1/@permissions", nil)
	if err != nil {
		t.Fatalf("http.NewRequest: %v", err)
	}

	rr := httptest.NewRecorder()
	status := ReadTablePermissions(session, rr, req)

	if status != http.StatusOK {
		t.Fatalf("read table permissions for d1/t1: expected 200, got %d -- body: %s", status, rr.Body.String())
	}

	var body defs.AllPermissionResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode permissions response: %v -- body: %s", err, rr.Body.String())
	}

	if body.Count != 2 || len(body.Permissions) != 2 {
		t.Fatalf("expected 2 permission records for d1/t1, got %d -- body: %s", body.Count, rr.Body.String())
	}

	byUser := map[string]defs.PermissionObject{}
	for _, p := range body.Permissions {
		byUser[p.User] = p
	}

	alice, ok := byUser["alice"]
	if !ok {
		t.Fatalf("expected a record for alice, got %v", body.Permissions)
	}

	if got := alice.Permissions; len(got) != 2 || got[0] != defs.TableReadPermission || got[1] != defs.TableWritePermission {
		t.Errorf("alice: expected [read write], got %v", got)
	}

	bob, ok := byUser["bob"]
	if !ok {
		t.Fatalf("expected a record for bob, got %v", body.Permissions)
	}

	if got := bob.Permissions; len(got) != 1 || got[0] != defs.TableReadPermission {
		t.Errorf("bob: expected [read], got %v", got)
	}

	if _, ok := byUser["carol"]; ok {
		t.Errorf("carol's grant is on t2, not t1 -- it should not appear in d1/t1's permission list")
	}
}

// TestReadTablePermissions_NoGrants verifies that a table with no grants
// reports an empty (not nil) list rather than erroring.
func TestReadTablePermissions_NoGrants(t *testing.T) {
	permsFile := "testing-table-perms-empty-" + uuid.New().String() + ".db"

	t.Cleanup(func() {
		_ = os.Remove(permsFile)
		_ = os.Remove(permsFile + "-wal")
		_ = os.Remove(permsFile + "-shm")
	})

	original := settings.Get(defs.LogonUserdataSetting)
	settings.Set(defs.LogonUserdataSetting, "sqlite://"+permsFile)
	t.Cleanup(func() { settings.Set(defs.LogonUserdataSetting, original) })

	pValid = false
	pHandle = nil

	t.Cleanup(func() { pValid = false; pHandle = nil })

	session := &router.Session{ID: 1, User: "admin", Admin: true, URLParts: map[string]any{"dsn": "d1", "table": "empty"}}

	req, err := http.NewRequest(http.MethodGet, "/dsns/d1/tables/empty/@permissions", nil)
	if err != nil {
		t.Fatalf("http.NewRequest: %v", err)
	}

	rr := httptest.NewRecorder()
	status := ReadTablePermissions(session, rr, req)

	if status != http.StatusOK {
		t.Fatalf("read table permissions for d1/empty: expected 200, got %d -- body: %s", status, rr.Body.String())
	}

	var body defs.AllPermissionResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode permissions response: %v -- body: %s", err, rr.Body.String())
	}

	if body.Count != 0 || len(body.Permissions) != 0 {
		t.Fatalf("expected 0 permission records for d1/empty, got %d -- body: %s", body.Count, rr.Body.String())
	}
}

// TestReadTablePermissions_IncludesCreatorAutoGrant is a regression test for
// a bug found while adding the apitest coverage for this endpoint: TableCreate
// passed the FullName-qualified/quoted table name (e.g. `"t1"`) to
// createTablePermissions instead of the raw name that GrantPermissions,
// ReadPermissions, and ReadTablePermissions all store and filter table_perms.table
// by -- the same class of bug already fixed for DeleteTable's removeTablePermissions
// call (see TestDeleteTable_CleansUpTablePermissions). The creator's own full-access
// auto-grant was therefore stored under a name no later lookup could ever match,
// making it silently invisible to every one of those three handlers.
func TestReadTablePermissions_IncludesCreatorAutoGrant(t *testing.T) {
	dataFile := "testing-create-data-" + uuid.New().String() + ".db"

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

	permsFile := "testing-create-perms-" + uuid.New().String() + ".db"

	t.Cleanup(func() {
		_ = os.Remove(permsFile)
		_ = os.Remove(permsFile + "-wal")
		_ = os.Remove(permsFile + "-shm")
	})

	original := settings.Get(defs.LogonUserdataSetting)
	settings.Set(defs.LogonUserdataSetting, "sqlite://"+permsFile)
	t.Cleanup(func() { settings.Set(defs.LogonUserdataSetting, original) })

	pValid = false
	pHandle = nil

	t.Cleanup(func() { pValid = false; pHandle = nil })

	columns := []defs.DBColumn{{Name: "id", Type: "int"}}

	body, err := json.Marshal(columns)
	if err != nil {
		t.Fatalf("marshal column payload: %v", err)
	}

	req, err := http.NewRequest(http.MethodPut, "/dsns/d1/tables/t1", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("http.NewRequest: %v", err)
	}

	session := &router.Session{ID: 1, User: "admin", Admin: true, URLParts: map[string]any{"dsn": "d1", "table": "t1"}}

	rr := httptest.NewRecorder()
	status := TableCreate(session, rr, req)

	if status != http.StatusCreated {
		t.Fatalf("create table d1/t1: expected 201, got %d -- body: %s", status, rr.Body.String())
	}

	listReq, err := http.NewRequest(http.MethodGet, "/dsns/d1/tables/t1/@permissions", nil)
	if err != nil {
		t.Fatalf("http.NewRequest: %v", err)
	}

	rr = httptest.NewRecorder()
	status = ReadTablePermissions(session, rr, listReq)

	if status != http.StatusOK {
		t.Fatalf("read table permissions for d1/t1: expected 200, got %d -- body: %s", status, rr.Body.String())
	}

	var perms defs.AllPermissionResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &perms); err != nil {
		t.Fatalf("decode permissions response: %v -- body: %s", err, rr.Body.String())
	}

	if perms.Count != 1 || len(perms.Permissions) != 1 {
		t.Fatalf("expected exactly 1 permission record (the creator's auto-grant), got %d -- body: %s", perms.Count, rr.Body.String())
	}

	got := perms.Permissions[0]
	if got.User != "admin" || got.Table != "t1" || got.DSNName != "d1" {
		t.Fatalf("unexpected record: %+v", got)
	}

	want := []string{
		defs.TableAdminPermission,
		defs.TableDeletePermission,
		defs.TableReadPermission,
		defs.TableUpdatePermission,
		defs.TableWritePermission,
	}

	if len(got.Permissions) != len(want) {
		t.Fatalf("expected all 5 permissions, got %v", got.Permissions)
	}

	for i, p := range want {
		if got.Permissions[i] != p {
			t.Errorf("permission %d: expected %s, got %s", i, p, got.Permissions[i])
		}
	}
}
