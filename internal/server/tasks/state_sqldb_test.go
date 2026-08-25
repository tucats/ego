package tasks

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/dsns"
	"github.com/tucats/ego/internal/resources"
)

const exampleTaskName = "example task"

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
		defs.MemoryProvider:           false,
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

	// The task_state row must carry the task's description alongside its
	// id, purely so a "SELECT * FROM task_state" shows which task each row
	// belongs to -- see persistedState's doc comment. Checked directly
	// against the store (State itself deliberately has no Description
	// field -- see SaveState's doc comment), not through Status/LoadState.
	store, ok := activeStore.(*databaseStateStore)
	if !ok {
		t.Fatalf("activeStore = %T, want *databaseStateStore", activeStore)
	}

	persisted, err := store.load()
	if err != nil {
		t.Fatalf("store.load(): %v", err)
	}

	if got := persisted["11111111-1111-1111-1111-111111111111"].Description; got != exampleTaskName {
		t.Errorf("persisted Description = %q, want %q (from validTaskJSON's \"task\" field)", got, exampleTaskName)
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

// oldTaskStateRowFixture mimics the task_state row shape from before the
// Description column existed, so TestDatabaseStoreMigratesPreExistingTable
// below can build a table matching what a real pre-existing deployment's
// database would actually have -- through the same internal/resources
// machinery the real table is built with, not a hand-written CREATE TABLE
// that could quietly drift from it.
type oldTaskStateRowFixture struct {
	ID         string
	LastRun    string
	LastStatus int
	Success    bool
	RunCount   int
	FailedTest string
}

// TestDatabaseStoreMigratesPreExistingTable covers addDescriptionColumnIfMissing:
// opening the store against a task_state table that predates the
// Description column must add it (not error, and not silently leave the
// column missing), preserve every pre-existing row, and leave the store
// fully able to both read and write the new column afterward.
func TestDatabaseStoreMigratesPreExistingTable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "premigration.db")
	connStr := "sqlite://" + path

	oldHandle, err := resources.Open(oldTaskStateRowFixture{}, taskStateTable, connStr)
	if err != nil {
		t.Fatalf("test setup: %v", err)
	}

	oldHandle.SetPrimaryKey("ID")

	if err := oldHandle.Create(); err != nil {
		t.Fatalf("test setup: %v", err)
	}

	if err := oldHandle.Begin().Insert(oldTaskStateRowFixture{
		ID:         "11111111-1111-1111-1111-111111111111",
		LastRun:    "2020-01-01T00:00:00Z",
		LastStatus: 200,
		Success:    true,
		RunCount:   3,
	}); err != nil {
		t.Fatalf("test setup: %v", err)
	}

	oldHandle.Close()

	// Opening it the normal way must migrate the table rather than failing
	// or silently leaving the new column unusable.
	store, err := newDatabaseStateStore(connStr)
	if err != nil {
		t.Fatalf("newDatabaseStateStore did not migrate the pre-existing table: %v", err)
	}

	t.Cleanup(func() { _ = store.close() })

	persisted, err := store.load()
	if err != nil {
		t.Fatalf("load() after migration: %v", err)
	}

	entry, found := persisted["11111111-1111-1111-1111-111111111111"]
	if !found {
		t.Fatal("expected the pre-existing row to survive the migration")
	}

	if entry.RunCount != 3 || entry.LastStatus != 200 || !entry.Success {
		t.Errorf("migration lost or corrupted pre-existing data: %+v", entry)
	}

	if entry.Description != "" {
		t.Errorf("Description = %q, want \"\" (column was just added; nothing has backfilled it yet)", entry.Description)
	}

	// The new column must be writable too, not just present.
	if err := store.save(map[string]persistedState{
		"11111111-1111-1111-1111-111111111111": {
			Description: "backfilled", LastStatus: 200, Success: true, RunCount: 3,
		},
	}); err != nil {
		t.Fatalf("save() after migration: %v", err)
	}

	persisted, err = store.load()
	if err != nil {
		t.Fatalf("load() after save: %v", err)
	}

	if got := persisted["11111111-1111-1111-1111-111111111111"].Description; got != "backfilled" {
		t.Errorf("Description after save = %q, want %q", got, "backfilled")
	}
}

// TestDatabaseStoreMigrationIsIdempotent confirms opening the store a
// second time against a table that already has the Description column
// (the common case for every startup after the first one following an
// upgrade) doesn't error -- addDescriptionColumnIfMissing must correctly
// recognize and ignore SQLite's "duplicate column" failure.
func TestDatabaseStoreMigrationIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "already-migrated.db")
	connStr := "sqlite://" + path

	first, err := newDatabaseStateStore(connStr)
	if err != nil {
		t.Fatalf("first open: %v", err)
	}

	first.close()

	second, err := newDatabaseStateStore(connStr)
	if err != nil {
		t.Fatalf("second open (column already present) unexpectedly failed: %v", err)
	}

	second.close()
}

// TestLoadStateBackfillsDescriptionInDatabaseStore is the database-backed
// counterpart of TestLoadStateBackfillsMissingDescriptionInFileStore:
// LoadState must backpatch a task_state row's description to match its
// task's live one, whether the row predates the column (migrated to an
// empty string, as here) or simply has a stale value from before the task
// file's description last changed.
func TestLoadStateBackfillsDescriptionInDatabaseStore(t *testing.T) {
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

	store, ok := activeStore.(*databaseStateStore)
	if !ok {
		t.Fatalf("activeStore = %T, want *databaseStateStore", activeStore)
	}

	// Persist an entry with no description, as a migrated pre-existing row
	// (or one saved by an old build in the narrow window before this
	// feature landed) would have.
	if err := store.save(map[string]persistedState{
		"11111111-1111-1111-1111-111111111111": {LastStatus: 200, Success: true, RunCount: 3},
	}); err != nil {
		t.Fatalf("test setup: %v", err)
	}

	if err := LoadState(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	persisted, err := store.load()
	if err != nil {
		t.Fatalf("load() after backfill: %v", err)
	}

	entry := persisted["11111111-1111-1111-1111-111111111111"]

	if entry.Description != exampleTaskName {
		t.Errorf("backfilled Description = %q, want %q", entry.Description, exampleTaskName)
	}

	if entry.RunCount != 3 || entry.LastStatus != 200 || !entry.Success {
		t.Errorf("backfill save corrupted the rest of the entry's run history: %+v", entry)
	}
}
