package tasks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDeactivateFilePreservesCommentsAndFormatting(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "task.json")

	original := "# a comment describing this task\n" +
		"// another comment style\n" +
		"{\n" +
		"\t\"task\": \"example\",\n" +
		"\t\"active\": \"true\",\n" +
		"\t\"user\": \"admin\"\n" +
		"}\n"

	if err := os.WriteFile(path, []byte(original), requiredFileMode); err != nil {
		t.Fatalf("test setup: %v", err)
	}

	if err := deactivateFile(path); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}

	want := strings.Replace(original, `"active": "true"`, `"active": "false"`, 1)

	if string(got) != want {
		t.Errorf("file content after deactivate =\n%s\nwant\n%s", got, want)
	}
}

func TestDeactivateFilePreservesPermissions(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "task.json")

	if err := os.WriteFile(path, []byte(`{"active": "true"}`), requiredFileMode); err != nil {
		t.Fatalf("test setup: %v", err)
	}

	if err := deactivateFile(path); err != nil {
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

func TestDeactivateFileMissingActiveFieldIsNotAnError(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "task.json")

	original := `{"task": "example", "user": "admin"}`

	if err := os.WriteFile(path, []byte(original), requiredFileMode); err != nil {
		t.Fatalf("test setup: %v", err)
	}

	if err := deactivateFile(path); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}

	if string(got) != original {
		t.Errorf("file was modified despite having no \"active\" field: got %q, want unchanged %q", got, original)
	}
}

func TestDeactivateFileMissingFileIsError(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "does-not-exist.json")

	if err := deactivateFile(path); err == nil {
		t.Error("expected an error for a missing file, got nil")
	}
}
