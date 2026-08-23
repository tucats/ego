package tasks

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureDirPermissionsCreatesMissingDir(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "tasks")

	if err := ensureDirPermissions(dir); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("directory was not created: %v", err)
	}

	if info.Mode().Perm() != requiredDirMode {
		t.Errorf("mode = %04o, want %04o", info.Mode().Perm(), requiredDirMode)
	}
}

func TestEnsureDirPermissionsCorrectsExistingDir(t *testing.T) {
	dir := t.TempDir()

	if err := os.Chmod(dir, 0755); err != nil {
		t.Fatalf("test setup: %v", err)
	}

	if err := ensureDirPermissions(dir); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}

	if info.Mode().Perm() != requiredDirMode {
		t.Errorf("mode = %04o, want %04o", info.Mode().Perm(), requiredDirMode)
	}
}

func TestEnsureDirPermissionsNoopWhenAlreadyCorrect(t *testing.T) {
	dir := t.TempDir()

	if err := os.Chmod(dir, requiredDirMode); err != nil {
		t.Fatalf("test setup: %v", err)
	}

	if err := ensureDirPermissions(dir); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEnsureDirPermissionsRejectsNonDirectory(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "not-a-dir")

	if err := os.WriteFile(path, []byte("x"), 0600); err != nil {
		t.Fatalf("test setup: %v", err)
	}

	if err := ensureDirPermissions(path); err == nil {
		t.Error("expected an error when the path is a file, got nil")
	}
}

func TestEnsureFilePermissionsCorrectsExistingFile(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "task.json")

	if err := os.WriteFile(path, []byte("{}"), 0644); err != nil {
		t.Fatalf("test setup: %v", err)
	}

	if err := ensureFilePermissions(path); err != nil {
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

func TestEnsureFilePermissionsNoopWhenAlreadyCorrect(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "task.json")

	if err := os.WriteFile(path, []byte("{}"), requiredFileMode); err != nil {
		t.Fatalf("test setup: %v", err)
	}

	if err := ensureFilePermissions(path); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEnsureFilePermissionsMissingFileIsError(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "does-not-exist.json")

	if err := ensureFilePermissions(path); err == nil {
		t.Error("expected an error for a missing file, got nil")
	}
}
