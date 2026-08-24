package tasks

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadStateAppliesPersistedEntries(t *testing.T) {
	root := useTempLibDir(t)
	dir := filepath.Join(root, "tasks")

	if err := os.MkdirAll(dir, requiredDirMode); err != nil {
		t.Fatalf("test setup: %v", err)
	}

	writeTaskFile(t, dir, "example.json", validTaskJSON)

	if err := LoadAll(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	lastRun := time.Now().Add(-1 * time.Hour).UTC().Truncate(time.Second)

	stateJSON := `{
		"11111111-1111-1111-1111-111111111111": {
			"lastRun": "` + lastRun.Format(time.RFC3339) + `",
			"lastStatus": 200,
			"success": true,
			"runCount": 4
		}
	}`

	if err := os.WriteFile(StateFile(), []byte(stateJSON), requiredFileMode); err != nil {
		t.Fatalf("test setup: %v", err)
	}

	if err := LoadState(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	state, found := Status("11111111-1111-1111-1111-111111111111")
	if !found {
		t.Fatal("expected a state entry for the loaded task")
	}

	if !state.LastRun.Equal(lastRun) {
		t.Errorf("LastRun = %v, want %v", state.LastRun, lastRun)
	}

	if state.LastStatus != 200 || !state.Success || state.RunCount != 4 {
		t.Errorf("state = %+v, want status 200, success true, runCount 4", state)
	}
}

func TestLoadStateIgnoresEntriesForUnknownTasks(t *testing.T) {
	root := useTempLibDir(t)
	dir := filepath.Join(root, "tasks")

	if err := os.MkdirAll(dir, requiredDirMode); err != nil {
		t.Fatalf("test setup: %v", err)
	}

	stateJSON := `{"never-loaded-id": {"lastRun": "2020-01-01T00:00:00Z", "lastStatus": 200, "success": true}}`

	if err := os.WriteFile(StateFile(), []byte(stateJSON), requiredFileMode); err != nil {
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

func TestLoadStateMissingFileIsNotAnError(t *testing.T) {
	useTempLibDir(t)

	if err := LoadState(); err != nil {
		t.Fatalf("unexpected error for a missing state file: %v", err)
	}
}

func TestLoadStateCorruptFileIsNotFatal(t *testing.T) {
	root := useTempLibDir(t)
	dir := filepath.Join(root, "tasks")

	if err := os.MkdirAll(dir, requiredDirMode); err != nil {
		t.Fatalf("test setup: %v", err)
	}

	if err := os.WriteFile(StateFile(), []byte("not json"), requiredFileMode); err != nil {
		t.Fatalf("test setup: %v", err)
	}

	if err := LoadState(); err != nil {
		t.Fatalf("expected corrupt state file to be logged, not returned as an error: %v", err)
	}
}

func TestSaveStateRoundTrips(t *testing.T) {
	root := useTempLibDir(t)
	dir := filepath.Join(root, "tasks")

	if err := os.MkdirAll(dir, requiredDirMode); err != nil {
		t.Fatalf("test setup: %v", err)
	}

	writeTaskFile(t, dir, "example.json", validTaskJSON)

	if err := LoadAll(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	when := time.Now().UTC().Truncate(time.Second)

	recordRun("11111111-1111-1111-1111-111111111111", 200, true, "", when.Add(-time.Minute))
	recordRun("11111111-1111-1111-1111-111111111111", 200, true, "", when)

	if state, found := Status("11111111-1111-1111-1111-111111111111"); !found || state.RunCount != 2 {
		t.Fatalf("RunCount before restart = %+v, want 2", state)
	}

	info, err := os.Stat(StateFile())
	if err != nil {
		t.Fatalf("expected state file to be written: %v", err)
	}

	if info.Mode().Perm() != requiredFileMode {
		t.Errorf("state file mode = %04o, want %04o", info.Mode().Perm(), requiredFileMode)
	}

	// Simulate a restart: clear in-memory state (as a fresh register()
	// call would leave it -- just a LoadedAt, no run history yet), reload
	// from disk.
	loadedAt := time.Now()

	registryLock.Lock()
	states = map[string]*State{"11111111-1111-1111-1111-111111111111": {LoadedAt: loadedAt}}
	registryLock.Unlock()

	if err := LoadState(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	state, found := Status("11111111-1111-1111-1111-111111111111")
	if !found {
		t.Fatal("expected state to survive a reload")
	}

	if !state.LastRun.Equal(when) || state.LastStatus != 200 || !state.Success || state.RunCount != 2 {
		t.Errorf("reloaded state = %+v, want LastRun=%v, LastStatus=200, Success=true, RunCount=2", state, when)
	}

	if state.Running {
		t.Error("expected Running to be false after recordRun")
	}

	if !state.LoadedAt.Equal(loadedAt) {
		t.Errorf("LoadedAt = %v, want %v (LoadState must not clobber it -- it isn't persisted)", state.LoadedAt, loadedAt)
	}
}
