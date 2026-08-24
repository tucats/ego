package tasks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
)

func boolPtr(b bool) *bool    { return &b }
func intPtr(i int) *int       { return &i }
func strPtr(s string) *string { return &s }

// taskPatch is a terse constructor for defs.TaskPatchRequest, used
// throughout this file's table of patch scenarios; pass nil for any field
// that should be left out of the patch entirely.
func taskPatch(active *bool, interval *string, count *int, after *string) defs.TaskPatchRequest {
	return defs.TaskPatchRequest{Active: active, Interval: interval, Count: count, After: after}
}

func baseTestTask() *Task {
	return &Task{
		Description: "example task",
		ID:          "11111111-1111-1111-1111-111111111111",
		Active:      true,
		User:        "admin",
		Method:      "POST",
		Endpoint:    "/services/jiggle",
		Status:      200,
		Interval:    "10s",
		Count:       4,
		After:       "1m",
	}
}

// -- applyTaskPatch ----------------------------------------------------

func TestApplyTaskPatchOnlySetsProvidedFields(t *testing.T) {
	task := baseTestTask()

	updated := applyTaskPatch(task, taskPatch(boolPtr(false), nil, nil, nil))

	if updated.Active != false {
		t.Errorf("Active = %v, want false", updated.Active)
	}

	if updated.Interval != task.Interval {
		t.Errorf("Interval = %q, want unchanged %q", updated.Interval, task.Interval)
	}

	if updated.Count != task.Count {
		t.Errorf("Count = %d, want unchanged %d", updated.Count, task.Count)
	}

	if updated.After != task.After {
		t.Errorf("After = %q, want unchanged %q", updated.After, task.After)
	}
}

func TestApplyTaskPatchAllFields(t *testing.T) {
	task := baseTestTask()

	updated := applyTaskPatch(task, taskPatch(boolPtr(false), strPtr("1h"), intPtr(9), strPtr("30s")))

	if updated.Active != false {
		t.Errorf("Active = %v, want false", updated.Active)
	}

	if updated.Interval != "1h" {
		t.Errorf("Interval = %q, want %q", updated.Interval, "1h")
	}

	if updated.Count != 9 {
		t.Errorf("Count = %d, want 9", updated.Count)
	}

	if updated.After != "30s" {
		t.Errorf("After = %q, want %q", updated.After, "30s")
	}
}

func TestApplyTaskPatchClearsFieldToZeroValueWhenExplicitlySet(t *testing.T) {
	task := baseTestTask()

	// An explicit empty string / zero must actually take effect -- this is
	// exactly why TaskPatchRequest's fields are pointers rather than plain
	// values: "" and 0 are meaningful, deliberate patches (e.g. clearing
	// Interval back to one-shot), not "field omitted".
	updated := applyTaskPatch(task, taskPatch(nil, strPtr(""), intPtr(0), nil))

	if updated.Interval != "" {
		t.Errorf("Interval = %q, want cleared to \"\"", updated.Interval)
	}

	if updated.Count != 0 {
		t.Errorf("Count = %d, want cleared to 0", updated.Count)
	}
}

func TestApplyTaskPatchDoesNotMutateOriginal(t *testing.T) {
	task := baseTestTask()
	wantActive, wantInterval, wantCount, wantAfter := task.Active, task.Interval, task.Count, task.After

	_ = applyTaskPatch(task, taskPatch(boolPtr(false), strPtr("2h"), intPtr(99), strPtr("5m")))

	if task.Active != wantActive || task.Interval != wantInterval || task.Count != wantCount || task.After != wantAfter {
		t.Errorf("applyTaskPatch mutated the original task: got Active=%v Interval=%q Count=%d After=%q, want unchanged Active=%v Interval=%q Count=%d After=%q",
			task.Active, task.Interval, task.Count, task.After, wantActive, wantInterval, wantCount, wantAfter)
	}
}

// -- taskPatchFileFields -------------------------------------------------

func TestTaskPatchFileFieldsOnlyIncludesProvidedFieldsInFixedOrder(t *testing.T) {
	fields := taskPatchFileFields(taskPatch(nil, strPtr("5m"), nil, strPtr("1m")))

	if len(fields) != 2 {
		t.Fatalf("len(fields) = %d, want 2", len(fields))
	}

	if fields[0].Key != "interval" || fields[1].Key != "after" {
		t.Errorf("fields = %+v, want interval before after (fixed order), regardless of TaskPatchRequest field order", fields)
	}
}

func TestTaskPatchFileFieldsStringifiesActiveForOnDiskStringTag(t *testing.T) {
	// Task.Active is tagged `json:"active,string"` -- on disk it is the
	// JSON string "true"/"false", not a bare boolean -- so the jsonFieldPatch
	// Value must already be that Go string, which patchJSONFields then
	// marshals into a quoted JSON string.
	fields := taskPatchFileFields(taskPatch(boolPtr(true), nil, nil, nil))

	if len(fields) != 1 {
		t.Fatalf("len(fields) = %d, want 1", len(fields))
	}

	value, ok := fields[0].Value.(string)
	if !ok || value != "true" {
		t.Errorf("active field value = %#v, want the Go string \"true\"", fields[0].Value)
	}
}

func TestTaskPatchFileFieldsCountRemainsAnInt(t *testing.T) {
	fields := taskPatchFileFields(taskPatch(nil, nil, intPtr(7), nil))

	if len(fields) != 1 {
		t.Fatalf("len(fields) = %d, want 1", len(fields))
	}

	if value, ok := fields[0].Value.(int); !ok || value != 7 {
		t.Errorf("count field value = %#v, want the Go int 7", fields[0].Value)
	}
}

func TestTaskPatchFileFieldsEmptyPatchProducesNoFields(t *testing.T) {
	fields := taskPatchFileFields(taskPatch(nil, nil, nil, nil))

	if len(fields) != 0 {
		t.Errorf("len(fields) = %d, want 0 for an all-nil patch", len(fields))
	}
}

// -- patchTask (validate + write file + upsert) ---------------------------

func TestPatchTaskWritesFilePreservesCommentsAndUpsertsRegistry(t *testing.T) {
	useTempLibDir(t)

	dir := filepath.Join(Directory())
	if err := os.MkdirAll(dir, requiredDirMode); err != nil {
		t.Fatalf("test setup: %v", err)
	}

	original := "# a hand-written comment\n" + validTaskJSON + "\n"
	path := writeTaskFile(t, dir, "example.json", original)

	if err := LoadAll(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	const id = "11111111-1111-1111-1111-111111111111"

	task, found := Lookup(id)
	if !found {
		t.Fatalf("test setup: task %s did not load", id)
	}

	// Give the task some execution history that a patch must NOT disturb.
	registryLock.Lock()
	states[id] = &State{LastRun: time.Unix(1000, 0), LastStatus: 200, Success: true, RunCount: 3}
	registryLock.Unlock()

	updated, err := patchTask(task, taskPatch(boolPtr(false), strPtr("5m"), intPtr(10), nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if updated.Active != false {
		t.Errorf("returned task Active = %v, want false", updated.Active)
	}

	if updated.Interval != "5m" {
		t.Errorf("returned task Interval = %q, want %q", updated.Interval, "5m")
	}

	if updated.Count != 10 {
		t.Errorf("returned task Count = %d, want 10", updated.Count)
	}

	// The registry must now report the patched definition...
	registered, found := Lookup(id)
	if !found {
		t.Fatal("task disappeared from the registry after patchTask")
	}

	if registered != updated {
		t.Error("Lookup after patchTask did not return the same *Task patchTask returned (upsert not applied?)")
	}

	if registered.Interval != "5m" || registered.Count != 10 || registered.Active {
		t.Errorf("registered task after patch = %+v, fields did not take effect", registered)
	}

	// ...but execution history must survive untouched, exactly like an
	// ordinary Reload edit (see upsert's doc comment in defs.go).
	registryLock.RLock()
	state := *states[id]
	registryLock.RUnlock()

	if state.RunCount != 3 || state.LastStatus != 200 || !state.Success {
		t.Errorf("task State was disturbed by patchTask: %+v", state)
	}

	// The file on disk must reflect the new values...
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}

	text := string(content)

	if !strings.Contains(text, `"active": "false"`) {
		t.Errorf("file was not patched to active:false:\n%s", text)
	}

	if !strings.Contains(text, `"interval": "5m"`) {
		t.Errorf("file did not gain the new interval field:\n%s", text)
	}

	if !strings.Contains(text, `"count": 10`) {
		t.Errorf("file count was not updated:\n%s", text)
	}

	// ...while its hand-written comment survives.
	if !strings.Contains(text, "# a hand-written comment") {
		t.Errorf("comment was lost while patching the file:\n%s", text)
	}
}

func TestPatchTaskRejectsInvalidIntervalAndLeavesFileUnchanged(t *testing.T) {
	useTempLibDir(t)

	dir := Directory()
	if err := os.MkdirAll(dir, requiredDirMode); err != nil {
		t.Fatalf("test setup: %v", err)
	}

	path := writeTaskFile(t, dir, "example.json", validTaskJSON)

	if err := LoadAll(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	const id = "11111111-1111-1111-1111-111111111111"

	task, found := Lookup(id)
	if !found {
		t.Fatalf("test setup: task %s did not load", id)
	}

	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("test setup: %v", err)
	}

	_, err = patchTask(task, taskPatch(nil, strPtr("not-a-duration"), nil, nil))
	if err == nil {
		t.Fatal("expected an error for an invalid interval, got nil")
	}

	if !errors.Equals(err, errors.ErrTasksInvalidField) {
		t.Errorf("error = %v, want ErrTasksInvalidField", err)
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}

	if string(after) != string(before) {
		t.Errorf("file was modified despite a validation failure:\nbefore:\n%s\nafter:\n%s", before, after)
	}

	registered, _ := Lookup(id)
	if registered.Interval != task.Interval {
		t.Error("in-memory task was modified despite a validation failure")
	}
}

func TestPatchTaskRejectsAmbiguousCountWithoutInterval(t *testing.T) {
	useTempLibDir(t)

	dir := Directory()
	if err := os.MkdirAll(dir, requiredDirMode); err != nil {
		t.Fatalf("test setup: %v", err)
	}

	// No "interval" field at all -- a one-shot task.
	oneShot := `{
		"task": "example",
		"id": "11111111-1111-1111-1111-111111111111",
		"active": "true",
		"user": "admin",
		"method": "post",
		"endpoint": "/services/jiggle",
		"status": 200
	}`

	writeTaskFile(t, dir, "example.json", oneShot)

	if err := LoadAll(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	const id = "11111111-1111-1111-1111-111111111111"

	task, found := Lookup(id)
	if !found {
		t.Fatalf("test setup: task %s did not load", id)
	}

	// Setting Count to something other than 0/1 without also supplying an
	// Interval can never be honored (a one-shot task never gets a second
	// chance to run) -- validateTask must reject this the same way it
	// rejects the equivalent load-time file content.
	_, err := patchTask(task, taskPatch(nil, nil, intPtr(5), nil))
	if err == nil {
		t.Fatal("expected an error for count without interval, got nil")
	}

	if !errors.Equals(err, errors.ErrTasksInvalidField) {
		t.Errorf("error = %v, want ErrTasksInvalidField", err)
	}
}

func TestPatchTaskCountWithIntervalSetTogetherSucceeds(t *testing.T) {
	useTempLibDir(t)

	dir := Directory()
	if err := os.MkdirAll(dir, requiredDirMode); err != nil {
		t.Fatalf("test setup: %v", err)
	}

	oneShot := `{
		"task": "example",
		"id": "11111111-1111-1111-1111-111111111111",
		"active": "true",
		"user": "admin",
		"method": "post",
		"endpoint": "/services/jiggle",
		"status": 200
	}`

	writeTaskFile(t, dir, "example.json", oneShot)

	if err := LoadAll(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	const id = "11111111-1111-1111-1111-111111111111"

	task, found := Lookup(id)
	if !found {
		t.Fatalf("test setup: task %s did not load", id)
	}

	// Setting BOTH count and interval in the same patch resolves the
	// ambiguity that setting count alone would trigger.
	updated, err := patchTask(task, taskPatch(nil, strPtr("1h"), intPtr(5), nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if updated.Interval != "1h" || updated.Count != 5 {
		t.Errorf("updated task = %+v, want Interval=1h Count=5", updated)
	}
}

func TestPatchTaskEmptyPatchIsANoOpAndDoesNotTouchFile(t *testing.T) {
	useTempLibDir(t)

	dir := Directory()
	if err := os.MkdirAll(dir, requiredDirMode); err != nil {
		t.Fatalf("test setup: %v", err)
	}

	path := writeTaskFile(t, dir, "example.json", validTaskJSON)

	if err := LoadAll(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	const id = "11111111-1111-1111-1111-111111111111"

	task, found := Lookup(id)
	if !found {
		t.Fatalf("test setup: task %s did not load", id)
	}

	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("test setup: %v", err)
	}

	updated, err := patchTask(task, taskPatch(nil, nil, nil, nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if updated != task {
		t.Error("expected patchTask to return the same *Task unchanged for an empty patch")
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}

	if string(after) != string(before) {
		t.Error("file was modified despite an empty patch")
	}
}

func TestPatchTaskFileMissingFileIsError(t *testing.T) {
	if err := patchTaskFile(filepath.Join(t.TempDir(), "does-not-exist.json"), []jsonFieldPatch{
		{Key: "active", Value: "false"},
	}); err == nil {
		t.Error("expected an error for a missing file, got nil")
	}
}

func TestPatchTaskFilePreservesPermissions(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "task.json")

	if err := os.WriteFile(path, []byte(validTaskJSON), requiredFileMode); err != nil {
		t.Fatalf("test setup: %v", err)
	}

	if err := patchTaskFile(path, []jsonFieldPatch{{Key: "active", Value: "false"}}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}

	if info.Mode().Perm() != requiredFileMode {
		t.Errorf("mode = %04o, want %04o", info.Mode().Perm(), requiredFileMode)
	}
}
