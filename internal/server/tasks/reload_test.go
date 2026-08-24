package tasks

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestReloadAddsNewTask(t *testing.T) {
	root := useTempLibDir(t)
	dir := filepath.Join(root, "tasks")

	if err := os.MkdirAll(dir, requiredDirMode); err != nil {
		t.Fatalf("test setup: %v", err)
	}

	if err := LoadAll(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(Tasks()) != 0 {
		t.Fatalf("expected no tasks before reload, got %d", len(Tasks()))
	}

	writeTaskFile(t, dir, "example.json", validTaskJSON)

	result, err := Reload()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Total != 1 || result.New != 1 || result.Updated != 0 || result.Removed != 0 {
		t.Errorf("result = %+v, want {Total:1 New:1 Updated:0 Removed:0}", result)
	}

	if _, found := Lookup("11111111-1111-1111-1111-111111111111"); !found {
		t.Error("expected the new task to be registered after reload")
	}
}

func TestReloadUpdatesDefinitionButPreservesState(t *testing.T) {
	root := useTempLibDir(t)
	dir := filepath.Join(root, "tasks")

	if err := os.MkdirAll(dir, requiredDirMode); err != nil {
		t.Fatalf("test setup: %v", err)
	}

	path := writeTaskFile(t, dir, "example.json", validTaskJSON)

	if err := LoadAll(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	const id = "11111111-1111-1111-1111-111111111111"

	loadedAt, found := Status(id)
	if !found {
		t.Fatal("test setup: expected state to exist after LoadAll")
	}

	when := time.Now().Add(-time.Hour).UTC().Truncate(time.Second)
	recordRun(id, 200, true, "", when)

	// Edit the file: change the description and endpoint, keep the id.
	edited := `{
		"task": "an edited description",
		"id": "11111111-1111-1111-1111-111111111111",
		"active": "true",
		"user": "admin",
		"method": "post",
		"endpoint": "/services/jiggled",
		"status": 200
	}`

	if err := os.WriteFile(path, []byte(edited), requiredFileMode); err != nil {
		t.Fatalf("test setup: %v", err)
	}

	result, err := Reload()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Total != 1 || result.New != 0 || result.Updated != 1 || result.Removed != 0 {
		t.Errorf("result = %+v, want {Total:1 New:0 Updated:1 Removed:0}", result)
	}

	task, found := Lookup(id)
	if !found {
		t.Fatal("expected task to still be registered")
	}

	if task.Description != "an edited description" || task.Endpoint != "/services/jiggled" {
		t.Errorf("task definition was not updated: %+v", task)
	}

	state, found := Status(id)
	if !found {
		t.Fatal("expected state to survive the reload")
	}

	if !state.LastRun.Equal(when) || state.LastStatus != 200 || !state.Success || state.RunCount != 1 {
		t.Errorf("state was reset by reload: %+v, want LastRun=%v, LastStatus=200, Success=true, RunCount=1", state, when)
	}

	if !state.LoadedAt.Equal(loadedAt.LoadedAt) {
		t.Errorf("LoadedAt = %v, want %v (an update via Reload must not re-anchor it)", state.LoadedAt, loadedAt.LoadedAt)
	}
}

func TestReloadReactivatesEditedTask(t *testing.T) {
	root := useTempLibDir(t)
	dir := filepath.Join(root, "tasks")

	if err := os.MkdirAll(dir, requiredDirMode); err != nil {
		t.Fatalf("test setup: %v", err)
	}

	path := writeTaskFile(t, dir, "example.json", validTaskJSON)

	if err := LoadAll(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	const id = "11111111-1111-1111-1111-111111111111"

	// Deactivate it the same way DELETE /admin/tasks/{id} would.
	if err := deactivateFile(path); err != nil {
		t.Fatalf("test setup: %v", err)
	}

	setActive(id, false)

	if task, _ := Lookup(id); task.Active {
		t.Fatal("test setup: expected task to be inactive before reactivating")
	}

	// Hand-edit the file back to active, as an admin fixing the problem
	// would, then reload instead of restarting the server.
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("test setup: %v", err)
	}

	reactivated := activeFieldPattern.ReplaceAll(content, []byte(`${1}"true"`))

	if err := os.WriteFile(path, reactivated, requiredFileMode); err != nil {
		t.Fatalf("test setup: %v", err)
	}

	if _, err := Reload(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	task, found := Lookup(id)
	if !found {
		t.Fatal("expected task to still be registered")
	}

	if !task.Active {
		t.Error("expected the reactivated task to be Active after reload")
	}
}

func TestReloadRemovesDeletedTaskFile(t *testing.T) {
	root := useTempLibDir(t)
	dir := filepath.Join(root, "tasks")

	if err := os.MkdirAll(dir, requiredDirMode); err != nil {
		t.Fatalf("test setup: %v", err)
	}

	path := writeTaskFile(t, dir, "example.json", validTaskJSON)

	if err := LoadAll(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	const id = "11111111-1111-1111-1111-111111111111"

	if _, found := Lookup(id); !found {
		t.Fatal("test setup: expected task to be loaded")
	}

	if err := os.Remove(path); err != nil {
		t.Fatalf("test setup: %v", err)
	}

	result, err := Reload()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Total != 0 || result.Removed != 1 {
		t.Errorf("result = %+v, want {Total:0 Removed:1}", result)
	}

	if _, found := Lookup(id); found {
		t.Error("expected the task to be removed after its file was deleted")
	}
}

func TestReloadDuplicateIDFirstFileWins(t *testing.T) {
	root := useTempLibDir(t)
	dir := filepath.Join(root, "tasks")

	if err := os.MkdirAll(dir, requiredDirMode); err != nil {
		t.Fatalf("test setup: %v", err)
	}

	if err := LoadAll(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	writeTaskFile(t, dir, "a-first.json", validTaskJSON)
	writeTaskFile(t, dir, "b-second.json", validTaskJSON)

	result, err := Reload()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Total != 1 || result.New != 1 {
		t.Errorf("result = %+v, want {Total:1 New:1}", result)
	}

	task, found := Lookup("11111111-1111-1111-1111-111111111111")
	if !found {
		t.Fatal("expected the task to be registered")
	}

	if filepath.Base(task.Path) != "a-first.json" {
		t.Errorf("registered task came from %q, want %q", filepath.Base(task.Path), "a-first.json")
	}
}

func TestReloadSkipsInvalidFileWithoutTouchingExistingEntry(t *testing.T) {
	root := useTempLibDir(t)
	dir := filepath.Join(root, "tasks")

	if err := os.MkdirAll(dir, requiredDirMode); err != nil {
		t.Fatalf("test setup: %v", err)
	}

	path := writeTaskFile(t, dir, "example.json", validTaskJSON)

	if err := LoadAll(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Corrupt the file in place.
	if err := os.WriteFile(path, []byte("not valid json"), requiredFileMode); err != nil {
		t.Fatalf("test setup: %v", err)
	}

	result, err := Reload()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Total != 0 {
		t.Errorf("result.Total = %d, want 0 for a reload pass with only an invalid file", result.Total)
	}

	// The task should still be registered with its old (last-known-good)
	// definition -- an invalid file on disk doesn't remove the task,
	// since a bad edit shouldn't be able to kill a running task's
	// registration or execution history.
	if _, found := Lookup("11111111-1111-1111-1111-111111111111"); !found {
		t.Error("expected the previously-loaded task to remain registered despite the invalid edit")
	}
}
