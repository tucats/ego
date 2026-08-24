package tasks

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tucats/ego/internal/cli/settings"
	"github.com/tucats/ego/internal/defs"
)

// useTempLibDir points the tasks directory resolution at a fresh temp
// directory for the duration of the test, and clears the registry so tests
// don't see tasks left behind by an earlier test.
func useTempLibDir(t *testing.T) string {
	t.Helper()

	root := t.TempDir()

	settings.SetDefault(defs.EgoLibPathSetting, root)

	t.Cleanup(func() {
		settings.DeleteDefault(defs.EgoLibPathSetting)
	})

	resetRegistry(t)

	return root
}

func resetRegistry(t *testing.T) {
	t.Helper()

	registryLock.Lock()
	defer registryLock.Unlock()

	registry = map[string]*Task{}
	states = map[string]*State{}
}

func writeTaskFile(t *testing.T, dir, name, content string) string {
	t.Helper()

	path := filepath.Join(dir, name)

	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("test setup: %v", err)
	}

	return path
}

const validTaskJSON = `{
	"task": "example task",
	"id": "11111111-1111-1111-1111-111111111111",
	"active": "true",
	"user": "admin",
	"method": "post",
	"endpoint": "/services/jiggle",
	"status": 200
}`

func TestLoadAllPreloadsSessionID(t *testing.T) {
	useTempLibDir(t)
	resetSaved(t)

	original := defs.InstanceID
	defs.InstanceID = "22222222-2222-2222-2222-222222222222"

	t.Cleanup(func() { defs.InstanceID = original })

	if err := LoadAll(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := substitute("{{SESSIONID}}"); got != defs.InstanceID {
		t.Errorf("substitute(%q) = %q, want %q", "{{SESSIONID}}", got, defs.InstanceID)
	}
}

func TestLoadAllCreatesMissingDirectory(t *testing.T) {
	root := useTempLibDir(t)

	if err := LoadAll(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	info, err := os.Stat(Directory())
	if err != nil {
		t.Fatalf("tasks directory was not created: %v", err)
	}

	if info.Mode().Perm() != requiredDirMode {
		t.Errorf("directory mode = %04o, want %04o", info.Mode().Perm(), requiredDirMode)
	}

	if len(Tasks()) != 0 {
		t.Errorf("expected no tasks, got %d", len(Tasks()))
	}

	_ = root
}

func TestLoadAllLoadsValidTask(t *testing.T) {
	root := useTempLibDir(t)
	dir := filepath.Join(root, "tasks")

	if err := os.MkdirAll(dir, requiredDirMode); err != nil {
		t.Fatalf("test setup: %v", err)
	}

	writeTaskFile(t, dir, "example.json", validTaskJSON)

	if err := LoadAll(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	task, found := Lookup("11111111-1111-1111-1111-111111111111")
	if !found {
		t.Fatal("expected task to be loaded")
	}

	if task.Method != "POST" {
		t.Errorf("method = %q, want normalized %q", task.Method, "POST")
	}

	if !task.Active {
		t.Error("expected task.Active to be true")
	}
}

func TestLoadAllStripsCommentLines(t *testing.T) {
	root := useTempLibDir(t)
	dir := filepath.Join(root, "tasks")

	if err := os.MkdirAll(dir, requiredDirMode); err != nil {
		t.Fatalf("test setup: %v", err)
	}

	content := "# a comment describing this task\n" +
		"// another comment style\n" +
		validTaskJSON

	writeTaskFile(t, dir, "commented.json", content)

	if err := LoadAll(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, found := Lookup("11111111-1111-1111-1111-111111111111"); !found {
		t.Fatal("expected task with comment lines to load")
	}
}

func TestLoadAllFixesFilePermissions(t *testing.T) {
	root := useTempLibDir(t)
	dir := filepath.Join(root, "tasks")

	if err := os.MkdirAll(dir, requiredDirMode); err != nil {
		t.Fatalf("test setup: %v", err)
	}

	path := writeTaskFile(t, dir, "example.json", validTaskJSON)

	if err := os.Chmod(path, 0644); err != nil {
		t.Fatalf("test setup: %v", err)
	}

	if err := LoadAll(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}

	if info.Mode().Perm() != requiredFileMode {
		t.Errorf("mode = %04o, want %04o", info.Mode().Perm(), requiredFileMode)
	}

	if _, found := Lookup("11111111-1111-1111-1111-111111111111"); !found {
		t.Fatal("expected task to load after permissions were corrected")
	}
}

func TestLoadAllSkipsMissingRequiredField(t *testing.T) {
	root := useTempLibDir(t)
	dir := filepath.Join(root, "tasks")

	if err := os.MkdirAll(dir, requiredDirMode); err != nil {
		t.Fatalf("test setup: %v", err)
	}

	// Missing "endpoint".
	content := `{
		"id": "22222222-2222-2222-2222-222222222222",
		"user": "admin",
		"method": "get"
	}`

	writeTaskFile(t, dir, "invalid.json", content)

	if err := LoadAll(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, found := Lookup("22222222-2222-2222-2222-222222222222"); found {
		t.Error("expected invalid task to be skipped, but it was loaded")
	}
}

func TestLoadAllSkipsInvalidMethod(t *testing.T) {
	root := useTempLibDir(t)
	dir := filepath.Join(root, "tasks")

	if err := os.MkdirAll(dir, requiredDirMode); err != nil {
		t.Fatalf("test setup: %v", err)
	}

	content := `{
		"id": "33333333-3333-3333-3333-333333333333",
		"user": "admin",
		"method": "frobnicate",
		"endpoint": "/services/jiggle"
	}`

	writeTaskFile(t, dir, "invalid-method.json", content)

	if err := LoadAll(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, found := Lookup("33333333-3333-3333-3333-333333333333"); found {
		t.Error("expected task with invalid method to be skipped, but it was loaded")
	}
}

func TestLoadAllSkipsAmbiguousCountWithoutInterval(t *testing.T) {
	root := useTempLibDir(t)
	dir := filepath.Join(root, "tasks")

	if err := os.MkdirAll(dir, requiredDirMode); err != nil {
		t.Fatalf("test setup: %v", err)
	}

	// No interval, but count is neither absent nor 1 -- this can never be
	// satisfied (a one-shot task only ever gets one run), so it should be
	// rejected as ambiguous rather than silently accepted.
	content := `{
		"id": "44444444-4444-4444-4444-444444444444",
		"user": "admin",
		"method": "get",
		"endpoint": "/services/jiggle",
		"count": 5
	}`

	writeTaskFile(t, dir, "ambiguous-count.json", content)

	if err := LoadAll(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, found := Lookup("44444444-4444-4444-4444-444444444444"); found {
		t.Error("expected a count > 1 with no interval to be rejected as ambiguous")
	}
}

func TestLoadAllAcceptsExplicitCountOneWithoutInterval(t *testing.T) {
	root := useTempLibDir(t)
	dir := filepath.Join(root, "tasks")

	if err := os.MkdirAll(dir, requiredDirMode); err != nil {
		t.Fatalf("test setup: %v", err)
	}

	content := `{
		"id": "55555555-5555-5555-5555-555555555555",
		"user": "admin",
		"method": "get",
		"endpoint": "/services/jiggle",
		"count": 1
	}`

	writeTaskFile(t, dir, "explicit-one-shot.json", content)

	if err := LoadAll(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, found := Lookup("55555555-5555-5555-5555-555555555555"); !found {
		t.Error("expected count:1 with no interval to be accepted (equivalent to omitting count)")
	}
}

func TestLoadAllSkipsInvalidAfter(t *testing.T) {
	root := useTempLibDir(t)
	dir := filepath.Join(root, "tasks")

	if err := os.MkdirAll(dir, requiredDirMode); err != nil {
		t.Fatalf("test setup: %v", err)
	}

	content := `{
		"id": "66666666-6666-6666-6666-666666666666",
		"user": "admin",
		"method": "get",
		"endpoint": "/services/jiggle",
		"after": "not-a-duration"
	}`

	writeTaskFile(t, dir, "invalid-after.json", content)

	if err := LoadAll(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, found := Lookup("66666666-6666-6666-6666-666666666666"); found {
		t.Error("expected task with an unparseable after value to be skipped")
	}
}

func TestLoadAllAcceptsIntervalCountAfterCombination(t *testing.T) {
	root := useTempLibDir(t)
	dir := filepath.Join(root, "tasks")

	if err := os.MkdirAll(dir, requiredDirMode); err != nil {
		t.Fatalf("test setup: %v", err)
	}

	content := `{
		"id": "77777777-7777-7777-7777-777777777777",
		"user": "admin",
		"method": "get",
		"endpoint": "/services/jiggle",
		"interval": "1h",
		"count": 3,
		"after": "30m"
	}`

	writeTaskFile(t, dir, "full-combo.json", content)

	if err := LoadAll(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	task, found := Lookup("77777777-7777-7777-7777-777777777777")
	if !found {
		t.Fatal("expected task with a valid interval/count/after combination to load")
	}

	if task.Interval != "1h" || task.Count != 3 || task.After != "30m" {
		t.Errorf("task = %+v, fields not parsed as expected", task)
	}
}

func TestLoadAllAcceptsValidTests(t *testing.T) {
	root := useTempLibDir(t)
	dir := filepath.Join(root, "tasks")

	if err := os.MkdirAll(dir, requiredDirMode); err != nil {
		t.Fatalf("test setup: %v", err)
	}

	content := `{
		"id": "88888888-8888-8888-8888-888888888888",
		"user": "admin",
		"method": "get",
		"endpoint": "/services/jiggle",
		"tests": [
			{"name": "status ok", "query": "status", "value": "ok"},
			{"name": "count present", "query": "items", "op": "len", "value": "2"},
			{"name": "no error field", "query": "error", "op": "not-exists"}
		]
	}`

	writeTaskFile(t, dir, "with-tests.json", content)

	if err := LoadAll(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	task, found := Lookup("88888888-8888-8888-8888-888888888888")
	if !found {
		t.Fatal("expected task with a valid tests block to load")
	}

	if len(task.Tests) != 3 {
		t.Fatalf("len(task.Tests) = %d, want 3", len(task.Tests))
	}
}

func TestLoadAllSkipsTestMissingName(t *testing.T) {
	root := useTempLibDir(t)
	dir := filepath.Join(root, "tasks")

	if err := os.MkdirAll(dir, requiredDirMode); err != nil {
		t.Fatalf("test setup: %v", err)
	}

	content := `{
		"id": "99999999-9999-9999-9999-999999999999",
		"user": "admin",
		"method": "get",
		"endpoint": "/services/jiggle",
		"tests": [{"query": "status", "value": "ok"}]
	}`

	writeTaskFile(t, dir, "test-missing-name.json", content)

	if err := LoadAll(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, found := Lookup("99999999-9999-9999-9999-999999999999"); found {
		t.Error("expected a test entry with no name to be rejected")
	}
}

func TestLoadAllSkipsTestMissingQuery(t *testing.T) {
	root := useTempLibDir(t)
	dir := filepath.Join(root, "tasks")

	if err := os.MkdirAll(dir, requiredDirMode); err != nil {
		t.Fatalf("test setup: %v", err)
	}

	content := `{
		"id": "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
		"user": "admin",
		"method": "get",
		"endpoint": "/services/jiggle",
		"tests": [{"name": "no query", "value": "ok"}]
	}`

	writeTaskFile(t, dir, "test-missing-query.json", content)

	if err := LoadAll(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, found := Lookup("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"); found {
		t.Error("expected a test entry with no query to be rejected")
	}
}

func TestLoadAllSkipsTestInvalidOperator(t *testing.T) {
	root := useTempLibDir(t)
	dir := filepath.Join(root, "tasks")

	if err := os.MkdirAll(dir, requiredDirMode); err != nil {
		t.Fatalf("test setup: %v", err)
	}

	content := `{
		"id": "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb",
		"user": "admin",
		"method": "get",
		"endpoint": "/services/jiggle",
		"tests": [{"name": "bad op", "query": "status", "op": "frobnicate"}]
	}`

	writeTaskFile(t, dir, "test-invalid-op.json", content)

	if err := LoadAll(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, found := Lookup("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"); found {
		t.Error("expected a test entry with an invalid operator to be rejected")
	}
}

func TestLoadAllSkipsTestLenWithNonNumericValue(t *testing.T) {
	root := useTempLibDir(t)
	dir := filepath.Join(root, "tasks")

	if err := os.MkdirAll(dir, requiredDirMode); err != nil {
		t.Fatalf("test setup: %v", err)
	}

	content := `{
		"id": "cccccccc-cccc-cccc-cccc-cccccccccccc",
		"user": "admin",
		"method": "get",
		"endpoint": "/services/jiggle",
		"tests": [{"name": "bad len", "query": "items", "op": "len", "value": "not-a-number"}]
	}`

	writeTaskFile(t, dir, "test-invalid-len.json", content)

	if err := LoadAll(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, found := Lookup("cccccccc-cccc-cccc-cccc-cccccccccccc"); found {
		t.Error("expected a \"len\" test with a non-numeric value to be rejected")
	}
}

func TestLoadAllDuplicateIDFirstFileWins(t *testing.T) {
	root := useTempLibDir(t)
	dir := filepath.Join(root, "tasks")

	if err := os.MkdirAll(dir, requiredDirMode); err != nil {
		t.Fatalf("test setup: %v", err)
	}

	// Filenames are chosen so "a-first.json" sorts before "b-second.json".
	writeTaskFile(t, dir, "a-first.json", validTaskJSON)
	writeTaskFile(t, dir, "b-second.json", validTaskJSON)

	if err := LoadAll(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	task, found := Lookup("11111111-1111-1111-1111-111111111111")
	if !found {
		t.Fatal("expected the first-loaded task to be registered")
	}

	if filepath.Base(task.Path) != "a-first.json" {
		t.Errorf("registered task came from %q, want %q", filepath.Base(task.Path), "a-first.json")
	}

	if len(Tasks()) != 1 {
		t.Errorf("expected exactly one registered task, got %d", len(Tasks()))
	}
}

func TestLoadAllRejectsUnfixablePermissions(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root can chmod anything; this case can't be exercised")
	}

	root := useTempLibDir(t)
	dir := filepath.Join(root, "tasks")

	if err := os.MkdirAll(dir, requiredDirMode); err != nil {
		t.Fatalf("test setup: %v", err)
	}

	path := writeTaskFile(t, dir, "example.json", validTaskJSON)

	// Removing read permission on the containing directory prevents os.Stat
	// (and therefore the chmod path) in loadOne from reaching the file at
	// all, which is the closest a same-user test can get to a permission
	// fix genuinely failing (the realistic case -- a different owner -- can
	// only be exercised manually; see docs/internals/TASKS.md).
	if err := os.Chmod(dir, 0000); err != nil {
		t.Fatalf("test setup: %v", err)
	}

	t.Cleanup(func() {
		_ = os.Chmod(dir, requiredDirMode)
	})

	if err := ensureFilePermissions(path); err == nil {
		t.Error("expected an error accessing the file through an inaccessible directory")
	}
}
