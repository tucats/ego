package dsns

import (
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/tucats/ego/internal/defs"
	egostrings "github.com/tucats/ego/internal/util/strings"
)

// testSystemDSNName is the catalog DSN name used by these tests -- an
// arbitrary admin-chosen value, since EnsureSystemDSN no longer has a
// hardcoded default name.
const testSystemDSNName = "ego-system"

// newTestDatabaseService creates a fresh sqlite-backed DSN service, wires it
// up as the package-level DSNService/DSNDatabaseURL (the way Initialize
// would), and returns a cleanup function.
func newTestDatabaseService(t *testing.T) func() {
	t.Helper()

	fileName := "test-" + uuid.NewString() + ".db"
	connStr := "sqlite://" + fileName

	svc, err := defineDSNService(connStr)
	if err != nil {
		t.Fatal(err)
	}

	DSNDatabaseURL = connStr

	return func() {
		svc.Flush()

		if err := svc.Close(); err != nil {
			t.Errorf("service.Close() failed: %v", err)
		}

		os.Remove(fileName)
		os.Remove(fileName + "-wal")
		os.Remove(fileName + "-shm")

		DSNDatabaseURL = ""
	}
}

func TestEnsureSystemDSN_NotDatabaseBacked(t *testing.T) {
	svc, err := defineDSNService(defs.MemoryProvider)
	if err != nil {
		t.Fatal(err)
	}

	defer svc.Close()

	DSNDatabaseURL = defs.MemoryProvider
	defer func() { DSNDatabaseURL = "" }()

	if err := EnsureSystemDSN(testSystemDSNName); err != nil {
		t.Fatalf("EnsureSystemDSN(testSystemDSNName) returned error for non-database store: %v", err)
	}

	if _, err := DSNService.ReadDSN(0, "", testSystemDSNName, true); err == nil {
		t.Fatal("expected no system DSN to be created for a non-database store")
	}
}

func TestEnsureSystemDSN_CreatesRestrictedSQLiteDSN(t *testing.T) {
	cleanup := newTestDatabaseService(t)
	defer cleanup()

	if err := EnsureSystemDSN(testSystemDSNName); err != nil {
		t.Fatalf("EnsureSystemDSN(testSystemDSNName) failed: %v", err)
	}

	dsn, err := DSNService.ReadDSN(0, "", testSystemDSNName, true)
	if err != nil {
		t.Fatalf("expected system DSN to exist, got error: %v", err)
	}

	if !dsn.Restricted {
		t.Error("expected system DSN to be restricted")
	}

	if dsn.Provider != defs.SqliteProvider {
		t.Errorf("expected provider %q, got %q", defs.SqliteProvider, dsn.Provider)
	}

	wantDatabase := egostrings.StripScheme(DSNDatabaseURL)
	if dsn.Database != wantDatabase {
		t.Errorf("expected database %q, got %q", wantDatabase, dsn.Database)
	}
}

func TestEnsureSystemDSN_IdempotentWhenAlreadyRestricted(t *testing.T) {
	cleanup := newTestDatabaseService(t)
	defer cleanup()

	if err := EnsureSystemDSN(testSystemDSNName); err != nil {
		t.Fatalf("first EnsureSystemDSN(testSystemDSNName) failed: %v", err)
	}

	if err := EnsureSystemDSN(testSystemDSNName); err != nil {
		t.Fatalf("second EnsureSystemDSN(testSystemDSNName) failed: %v", err)
	}

	dsn, err := DSNService.ReadDSN(0, "", testSystemDSNName, true)
	if err != nil {
		t.Fatalf("expected system DSN to still exist: %v", err)
	}

	if !dsn.Restricted {
		t.Error("expected system DSN to remain restricted")
	}
}

func TestEnsureSystemDSN_RestoresRestrictedFlag(t *testing.T) {
	cleanup := newTestDatabaseService(t)
	defer cleanup()

	if err := EnsureSystemDSN(testSystemDSNName); err != nil {
		t.Fatalf("EnsureSystemDSN(testSystemDSNName) failed: %v", err)
	}

	dsn, err := DSNService.ReadDSN(0, "", testSystemDSNName, true)
	if err != nil {
		t.Fatalf("expected system DSN to exist: %v", err)
	}

	// Simulate an out-of-band edit that cleared the restricted flag.
	dsn.Restricted = false

	if err := DSNService.WriteDSN(0, "", dsn); err != nil {
		t.Fatalf("failed to unrestrict DSN for test setup: %v", err)
	}

	if err := EnsureSystemDSN(testSystemDSNName); err != nil {
		t.Fatalf("EnsureSystemDSN(testSystemDSNName) failed on repair: %v", err)
	}

	dsn, err = DSNService.ReadDSN(0, "", testSystemDSNName, true)
	if err != nil {
		t.Fatalf("expected system DSN to exist: %v", err)
	}

	if !dsn.Restricted {
		t.Error("expected EnsureSystemDSN(testSystemDSNName) to restore the restricted flag")
	}
}

func TestSystemDSNFromURL_Postgres(t *testing.T) {
	dsn, err := systemDSNFromURL("postgres://scott:tiger@dbhost:5433/catalog?sslmode=disable", testSystemDSNName)
	if err != nil {
		t.Fatalf("systemDSNFromURL() failed: %v", err)
	}

	if dsn.Name != testSystemDSNName {
		t.Errorf("expected name %q, got %q", testSystemDSNName, dsn.Name)
	}

	if dsn.Provider != defs.PostgresProvider {
		t.Errorf("expected provider %q, got %q", defs.PostgresProvider, dsn.Provider)
	}

	if dsn.Host != "dbhost" {
		t.Errorf("expected host %q, got %q", "dbhost", dsn.Host)
	}

	if dsn.Port != 5433 {
		t.Errorf("expected port 5433, got %d", dsn.Port)
	}

	if dsn.Database != "catalog" {
		t.Errorf("expected database %q, got %q", "catalog", dsn.Database)
	}

	if dsn.Username != "scott" {
		t.Errorf("expected username %q, got %q", "scott", dsn.Username)
	}

	if dsn.Secured {
		t.Error("expected Secured to be false for sslmode=disable")
	}

	if !dsn.Restricted {
		t.Error("expected system DSN to be restricted")
	}

	password, err := decrypt(dsn.Password)
	if err != nil {
		t.Fatalf("failed to decrypt password: %v", err)
	}

	if password != "tiger" {
		t.Errorf("expected password %q, got %q", "tiger", password)
	}
}
