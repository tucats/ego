package tables

// Regression tests for the @transaction endpoint's permission enforcement.
// Before this, scripting.Handler's per-opcode handlers (select, readrows,
// insert, update, delete, drop) never consulted Authorized() at all, and
// the raw "sql" opcode (doSQL) executed client-supplied SQL text with zero
// permission check of any kind -- not even the defs.SQLPermission the
// dedicated @sql endpoint's own route requires. A caller holding nothing
// but ego.logon could reach a Restricted DSN's tables through
// /dsns/{dsn}/@transaction regardless of table_perms, and run arbitrary SQL
// through it that @sql itself would refuse outright.
//
// These tests exercise scripting.Handler end-to-end (the same way
// delete_test.go exercises DeleteTable) against a real Restricted DSN and a
// real table_perms store, so they cover the actual wiring
// (scripting.AuthorizedFunc, set by this package's init() in
// scripting_authz.go) rather than a hand-built fake.

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
	"github.com/tucats/ego/internal/server/tables/scripting"

	_ "modernc.org/sqlite"
)

// setUpTransactionAuthzTest wires a Restricted DSN "d1" (backed by a real
// SQLite file containing table "secrets") and a real table_perms store,
// mirroring delete_test.go's setup. It returns a function that posts a
// transaction script (a []defs.TXOperation, marshaled to JSON) to
// scripting.Handler under the given session, and the response status.
func setUpTransactionAuthzTest(t *testing.T) func(session *router.Session, ops []defs.TXOperation) (int, *httptest.ResponseRecorder) {
	t.Helper()

	dataFile := "testing-tx-authz-data-" + uuid.New().String() + ".db"
	
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

	if _, err := dataHandle.Exec(`CREATE TABLE secrets (id INTEGER, value TEXT)`); err != nil {
		t.Fatalf("create table secrets: %v", err)
	}

	if _, err := dataHandle.Exec(`INSERT INTO secrets (id, value) VALUES (1, 'shh')`); err != nil {
		t.Fatalf("seed secrets: %v", err)
	}

	svc, err := dsns.NewFileService("memory")
	if err != nil {
		t.Fatalf("create test DSN service: %v", err)
	}

	dsns.DSNService = svc

	if err := dsns.DSNService.WriteDSN(1, "admin", defs.DSN{
		Name:       "d1",
		Provider:   defs.SqliteProvider,
		Database:   dataFile,
		Restricted: true,
	}); err != nil {
		t.Fatalf("write DSN: %v", err)
	}

	permsFile := "testing-tx-authz-perms-" + uuid.New().String() + ".db"
	
	t.Cleanup(func() {
		_ = os.Remove(permsFile)
		_ = os.Remove(permsFile + "-wal")
		_ = os.Remove(permsFile + "-shm")
	})

	original := settings.Get(defs.LogonUserdataSetting)
	settings.Set(defs.LogonUserdataSetting, "sqlite://"+permsFile)
	t.Cleanup(func() { settings.Set(defs.LogonUserdataSetting, original) })

	// initPermissions() (security.go) only ever opens pHandle once per
	// process -- pValid short-circuits every later call, regardless of
	// what defs.LogonUserdataSetting is set to. Since pHandle/pValid are
	// package-level singletons shared by every test in this package (not
	// just this file), a test that ran earlier in the same `go test`
	// invocation and already triggered initPermissions() would otherwise
	// leave every grant/check in this test silently operating against
	// that earlier test's (by-then-cleaned-up) database file instead of
	// the one just configured above. Force a fresh handle for this test.
	pValid = false
	pHandle = nil
	
	t.Cleanup(func() { pValid = false; pHandle = nil })

	post := func(session *router.Session, ops []defs.TXOperation) (int, *httptest.ResponseRecorder) {
		t.Helper()

		body, err := json.Marshal(ops)
		if err != nil {
			t.Fatalf("marshal transaction body: %v", err)
		}

		if session.URLParts == nil {
			session.URLParts = map[string]any{}
		}

		session.URLParts["dsn"] = "d1"

		req, err := http.NewRequest(http.MethodPost, "/dsns/d1/@transaction", bytes.NewReader(body))
		if err != nil {
			t.Fatalf("http.NewRequest: %v", err)
		}

		rr := httptest.NewRecorder()
		status := scripting.Handler(session, rr, req)

		return status, rr
	}

	return post
}

// grantDSNAccess gives user read+write access to DSN "d1" via a per-DSN
// dsns_auth record, the same kind of grant CreateDSNHandler gives a
// Restricted DSN's creator (DATA-SECURITY.md §3.3). database.Open enforces
// this DSN-level gate before any table_perms check is ever reached -- a
// caller with no standing on the DSN at all is refused here regardless of
// what table_perms says, so every non-admin test below needs it just to
// open the database, independent of whatever table_perms scenario the
// test is actually exercising.
func grantDSNAccess(t *testing.T, user string) {
	t.Helper()

	if err := dsns.DSNService.GrantDSN(1, user, "d1", dsns.DSNReadAction|dsns.DSNWriteAction, true); err != nil {
		t.Fatalf("grant DSN access for %s on d1: %v", user, err)
	}
}

// grantTablePermission grants readPermission (e.g. defs.TableReadPermission)
// on d1.tableName to user, via the real GrantPermissions handler -- the same
// path an operator would use.
func grantTablePermission(t *testing.T, user, tableName, permission string) {
	t.Helper()

	session := &router.Session{
		ID: 1, User: "admin", Admin: true,
		URLParts:   map[string]any{"dsn": "d1", "table": tableName},
		Parameters: map[string][]string{"user": {user}},
	}
	body, _ := json.Marshal([]string{permission})
	req, err := http.NewRequest(http.MethodPut, "/dsns/d1/tables/"+tableName+"/permissions?user="+user, bytes.NewReader(body))
	
	if err != nil {
		t.Fatalf("http.NewRequest: %v", err)
	}

	rr := httptest.NewRecorder()
	status := GrantPermissions(session, rr, req)

	if status != http.StatusOK {
		t.Fatalf("grant %s on d1.%s to %s: expected 200, got %d -- body: %s", permission, tableName, user, status, rr.Body.String())
	}
}

// TestTransactionSelect_RequiresTablePermission is the structured-opcode
// case: a non-admin caller with no table_perms grant on a Restricted DSN's
// table must be refused, and the same caller succeeds once granted
// defs.TableReadPermission.
func TestTransactionSelect_RequiresTablePermission(t *testing.T) {
	post := setUpTransactionAuthzTest(t)

	bob := &router.Session{ID: 2, User: "bob", Admin: false, Permissions: []string{defs.LogonPermission}}
	
	grantDSNAccess(t, "bob")

	ops := []defs.TXOperation{{Opcode: "select", Table: "secrets", Filters: []string{"EQ(id,1)"}}}

	status, rr := post(bob, ops)
	if status != http.StatusForbidden {
		t.Fatalf("select with no grant: expected 403, got %d -- body: %s", status, rr.Body.String())
	}

	grantTablePermission(t, "bob", "secrets", defs.TableReadPermission)

	status, rr = post(bob, ops)
	if status != http.StatusOK {
		t.Fatalf("select after grant: expected 200, got %d -- body: %s", status, rr.Body.String())
	}
}

// TestTransactionSQL_RequiresSQLPermission is the raw-SQL "sql" opcode's
// first gate: a caller lacking defs.SQLPermission (or ego.root) cannot use
// the "sql" opcode at all -- previously @transaction had no such gate,
// unlike the dedicated @sql endpoint's own route.
func TestTransactionSQL_RequiresSQLPermission(t *testing.T) {
	post := setUpTransactionAuthzTest(t)

	bob := &router.Session{ID: 2, User: "bob", Admin: false, Permissions: []string{defs.LogonPermission}}

	ops := []defs.TXOperation{{Opcode: "sql", SQL: "SELECT * FROM secrets"}}

	status, rr := post(bob, ops)
	if status != http.StatusForbidden {
		t.Fatalf("sql opcode without ego.sql: expected 403, got %d -- body: %s", status, rr.Body.String())
	}
}

// TestTransactionSQL_RequiresTablePermission is doSQL's second gate: once a
// caller holds defs.SQLPermission, the raw SQL text is still parsed and
// each table it touches authorized against table_perms, exactly like the
// dedicated @sql endpoint. This is the core of the fix -- before it, a
// defs.SQLPermission holder could read or write any table in a Restricted
// DSN via a transaction script's "sql" opcode, bypassing table_perms
// entirely.
func TestTransactionSQL_RequiresTablePermission(t *testing.T) {
	post := setUpTransactionAuthzTest(t)

	bob := &router.Session{ID: 2, User: "bob", Admin: false, Permissions: []string{defs.LogonPermission, defs.SQLPermission}}
	
	grantDSNAccess(t, "bob")

	ops := []defs.TXOperation{{Opcode: "sql", SQL: "SELECT * FROM secrets"}}

	status, rr := post(bob, ops)
	if status != http.StatusForbidden {
		t.Fatalf("sql select with ego.sql but no table grant: expected 403, got %d -- body: %s", status, rr.Body.String())
	}

	grantTablePermission(t, "bob", "secrets", defs.TableReadPermission)

	status, rr = post(bob, ops)
	if status != http.StatusOK {
		t.Fatalf("sql select after grant: expected 200, got %d -- body: %s", status, rr.Body.String())
	}
}

// TestTransactionSQL_DDLRequiresDSNAdmin covers doSQL's UsageAdmin branch:
// a CREATE TABLE statement (schema-altering DDL) requires identity-level
// ego.dsn.admin (or ego.root), not a table_perms grant -- table_perms
// grants a caller Read/Write/Update/Delete/Admin does not exist for a
// table that doesn't exist yet, matching how @sql's own authorizeStatement
// treats DDL.
func TestTransactionSQL_DDLRequiresDSNAdmin(t *testing.T) {
	post := setUpTransactionAuthzTest(t)

	bob := &router.Session{ID: 2, User: "bob", Admin: false, Permissions: []string{defs.LogonPermission, defs.SQLPermission}}
	// Read+write DSN access (only) is enough to open the database -- this
	// isolates the assertion below to authorizedForDDL's own check, rather
	// than incidentally failing at the earlier database.Open gate for
	// having no DSN standing at all.
	grantDSNAccess(t, "bob")

	ops := []defs.TXOperation{{Opcode: "sql", SQL: "CREATE TABLE newtable (id INTEGER)"}}

	status, rr := post(bob, ops)
	if status != http.StatusForbidden {
		t.Fatalf("CREATE TABLE without ego.dsn.admin: expected 403, got %d -- body: %s", status, rr.Body.String())
	}

	bob.Permissions = append(bob.Permissions, defs.DSNAdminPermission)

	status, rr = post(bob, ops)
	if status != http.StatusOK {
		t.Fatalf("CREATE TABLE with ego.dsn.admin: expected 200, got %d -- body: %s", status, rr.Body.String())
	}
}

// TestTransactionSelect_AdminBypassesTablePermission confirms session.Admin
// (ego.root) is unaffected by any of the above -- it always bypasses
// table_perms, matching every other call site of Authorized() in this
// package.
func TestTransactionSelect_AdminBypassesTablePermission(t *testing.T) {
	post := setUpTransactionAuthzTest(t)

	admin := &router.Session{ID: 1, User: "admin", Admin: true}

	ops := []defs.TXOperation{{Opcode: "select", Table: "secrets", Filters: []string{"EQ(id,1)"}}}

	status, rr := post(admin, ops)
	if status != http.StatusOK {
		t.Fatalf("select as admin: expected 200, got %d -- body: %s", status, rr.Body.String())
	}
}
