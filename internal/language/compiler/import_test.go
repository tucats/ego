package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDirectoryContents_CWDFallback is the BUG-93 regression test:
// directoryContents must fall back to resolving name relative to the
// current working directory (mirroring readPackageFile's own CWD-relative
// fallback for single-file packages, just below) when it isn't found under
// the lib/packages root either. Before the fix, a package organized as a
// directory of files had no such fallback at all, while the exact same
// package written as a single file did.
func TestDirectoryContents_CWDFallback(t *testing.T) {
	dir := t.TempDir()

	pkgDir := filepath.Join(dir, "mypkg")
	if err := os.Mkdir(pkgDir, 0o755); err != nil {
		t.Fatalf("failed to create package dir: %v", err)
	}

	source := "package mypkg\n\nvar Value = 42\n"
	if err := os.WriteFile(filepath.Join(pkgDir, "mypkg.ego"), []byte(source), 0o644); err != nil {
		t.Fatalf("failed to write package file: %v", err)
	}

	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}

	defer func() {
		_ = os.Chdir(oldWd)
	}()

	if err := os.Chdir(dir); err != nil {
		t.Fatalf("failed to change to temp directory: %v", err)
	}

	c := &Compiler{}

	content, err := c.directoryContents("mypkg")
	if err != nil {
		t.Fatalf("directoryContents(\"mypkg\") unexpected error: %v", err)
	}

	if !strings.Contains(content, "var Value = 42") {
		t.Errorf("directoryContents(\"mypkg\") = %q, want it to contain the package source", content)
	}
}

// TestDirectoryContents_NotFoundAnywhere confirms the fallback still
// reports an error (rather than panicking or silently succeeding) when the
// name doesn't resolve under lib/packages OR the working directory.
func TestDirectoryContents_NotFoundAnywhere(t *testing.T) {
	dir := t.TempDir()

	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}

	defer func() {
		_ = os.Chdir(oldWd)
	}()

	if err := os.Chdir(dir); err != nil {
		t.Fatalf("failed to change to temp directory: %v", err)
	}

	c := &Compiler{}

	if _, err := c.directoryContents("no-such-package"); err == nil {
		t.Error("directoryContents(\"no-such-package\") = nil error, want a not-found error")
	}
}
