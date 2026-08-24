package tasks

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tucats/ego/internal/dsns"
)

// useDatabaseStore points the task-state store at a fresh temp SQLite
// database for the duration of the test, and restores the file store
// (matching every other test in this package) on cleanup.
func useDatabaseStore(t *testing.T) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "task-state.db")
	connStr := "sqlite://" + path

	originalURL := dsns.DSNDatabaseURL
	dsns.DSNDatabaseURL = connStr

	InitializeStateStore()

	t.Cleanup(func() {
		_ = activeStore.close()

		activeStore = fileStateStore{}
		dsns.DSNDatabaseURL = originalURL

		os.Remove(path)
		os.Remove(path + "-wal")
		os.Remove(path + "-shm")
	})
}

func TestInitializeStateStore_SelectsDatabaseBackend(t *testing.T) {
	useDatabaseStore(t)

	if _, ok := activeStore.(*databaseStateStore); !ok {
		t.Fatalf("activeStore = %T, want *databaseStateStore", activeStore)
	}
}

func TestInitializeStateStore_DefaultsToFileBackend(t *testing.T) {
	originalURL := dsns.DSNDatabaseURL
	dsns.DSNDatabaseURL = ""

	t.Cleanup(func() {
		dsns.DSNDatabaseURL = originalURL
		activeStore = fileStateStore{}
	})

	InitializeStateStore()

	if _, ok := activeStore.(fileStateStore); !ok {
		t.Fatalf("activeStore = %T, want fileStateStore", activeStore)
	}
}

func TestIsDatabaseBacked(t *testing.T) {
	cases := map[string]bool{
		"":                            false,
		"memory":                      false,
		"/some/plain/path.json":       false,
		"sqlite://test.db":            true,
		"sqlite3://test.db":           true,
		"postgres://user@host/dbname": true,
	}

	for input, want := range cases {
		if got := isDatabaseBacked(input); got != want {
			t.Errorf("isDatabaseBacked(%q) = %v, want %v", input, got, want)
		}
	}
}

func TestDatabaseStore_SaveAndLoadRoundTrip(t *testing.T) {
	root := useTempLibDir(t)
	dir := filepath.Join(root, "tasks")

	if err := os.MkdirAll(dir, requiredDirMode); err != nil {
		t.Fatalf("test setup: %v", err)
	}

	writeTaskFile(t, dir, "example.json", validTaskJSON)

	if err := LoadAll(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	useDatabaseStore(t)

	when := time.Now().UTC().Truncate(time.Second)

	recordRun("11111111-1111-1111-1111-111111111111", 200, true, "", when.Add(-time.Minute))
	recordRun("11111111-1111-1111-1111-111111111111", 503, false, "status check", when)

	if state, found := Status("11111111-1111-1111-1111-111111111111"); !found || state.RunCount != 2 {
		t.Fatalf("RunCount before restart = %+v, want 2", state)
	}

	// Simulate a restart: clear in-memory state, then reload from the
	// database rather than the JSON file.
	loadedAt := time.Now()

	registryLock.Lock()
	states = map[string]*State{"11111111-1111-1111-1111-111111111111": {LoadedAt: loadedAt}}
	registryLock.Unlock()

	if err := LoadState(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	state, found := Status("11111111-1111-1111-1111-111111111111")
	if !found {
		t.Fatal("expected state to survive a reload from the database")
	}

	if !state.LastRun.Equal(when) {
		t.Errorf("LastRun = %v, want %v", state.LastRun, when)
	}

	if state.LastStatus != 503 || state.Success || state.FailedTest != "status check" || state.RunCount != 2 {
		t.Errorf("reloaded state = %+v, want LastStatus=503, Success=false, FailedTest=%q, RunCount=2", state, "status check")
	}

	if !state.LoadedAt.Equal(loadedAt) {
		t.Errorf("LoadedAt = %v, want %v (LoadState must not clobber it)", state.LoadedAt, loadedAt)
	}
}

func TestDatabaseStore_LoadIgnoresEntriesForUnknownTasks(t *testing.T) {
	root := useTempLibDir(t)
	dir := filepath.Join(root, "tasks")

	if err := os.MkdirAll(dir, requiredDirMode); err != nil {
		t.Fatalf("test setup: %v", err)
	}

	useDatabaseStore(t)

	store, ok := activeStore.(*databaseStateStore)
	if !ok {
		t.Fatalf("activeStore = %T, want *databaseStateStore", activeStore)
	}

	if err := store.save(map[string]persistedState{
		"never-loaded-id": {LastStatus: 200, Success: true},
	}); err != nil {
		t.Fatalf("test setup: %v", err)
	}

	if err := LoadAll(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := LoadState(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, found := Status("never-loaded-id"); found {
		t.Error("expected no state entry for a task id that was never loaded")
	}
}

func TestDatabaseStore_EmptyTableIsNotAnError(t *testing.T) {
	useTempLibDir(t)
	useDatabaseStore(t)

	if err := LoadState(); err != nil {
		t.Fatalf("unexpected error for an empty task_state table: %v", err)
	}
}
