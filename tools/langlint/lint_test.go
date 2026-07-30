package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFormatExample(t *testing.T) {
	input := `# This is a test input file
# to illustrate the linter's behavior
#
# Written by Tom
[init]
start=First entry
last=Last entry
[err]
none=No entry found
quit=Ending processing
any=Any values

[log]
count=Processed {{number}} entries
`

	want := `# This is a test input file
# to illustrate the linter's behavior
#
# Written by Tom

[init]
last=Last entry
start=First entry

[err]
any=Any values
none=No entry found
quit=Ending processing

[log]
count=Processed {{number}} entries
`

	got, warnings, err := Format([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}

	if string(got) != want {
		t.Fatalf("output mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestFormatIsIdempotent(t *testing.T) {
	input := `# header

[b]
z=one
a=two

[a]
only=value
`

	first, _, err := Format([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	second, _, err := Format(first)
	if err != nil {
		t.Fatalf("unexpected error on second pass: %v", err)
	}

	if string(first) != string(second) {
		t.Fatalf("formatting is not idempotent\n--- first ---\n%s\n--- second ---\n%s", first, second)
	}
}

func TestFormatNoHeaderPrefix(t *testing.T) {
	input := `# top
top=value

# section comment
[msg]
b=two
a=one
`

	want := `# top

top=value

# section comment

[msg]
a=one
b=two
`

	got, _, err := Format([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if string(got) != want {
		t.Fatalf("output mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestFormatMalformedLineMissingEquals(t *testing.T) {
	input := "[foo]\nnoequals\n"

	if _, _, err := Format([]byte(input)); err == nil {
		t.Fatal("expected an error for a line without '='")
	}
}

func TestFormatMalformedLineEmptyKey(t *testing.T) {
	input := "[foo]\n=value\n"

	if _, _, err := Format([]byte(input)); err == nil {
		t.Fatal("expected an error for a line with an empty key")
	}
}

func TestFormatMalformedHeader(t *testing.T) {
	input := "[foo\na=b\n"

	if _, _, err := Format([]byte(input)); err == nil {
		t.Fatal("expected an error for an unterminated section header")
	}
}

func TestFormatDuplicateKeyWarning(t *testing.T) {
	input := "[foo]\na=one\nb=two\na=three\n"

	_, warnings, err := Format([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(warnings) != 1 {
		t.Fatalf("expected exactly one warning, got %v", warnings)
	}
}

func TestFormatUnmatchedBraceWarning(t *testing.T) {
	input := "[foo]\na=missing {{brace\nb=escaped '{' is fine\n"

	_, warnings, err := Format([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(warnings) != 1 {
		t.Fatalf("expected exactly one warning, got %v", warnings)
	}
}

func TestLintFileRewritesInPlace(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "messages_xx.txt")

	if err := os.WriteFile(path, []byte("[b]\nz=one\na=two\n"), 0o644); err != nil {
		t.Fatalf("failed to write test fixture: %v", err)
	}

	result, err := lintFile(path, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.changed {
		t.Fatal("expected the file to be reported as changed")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read rewritten file: %v", err)
	}

	want := "[b]\na=two\nz=one\n"
	if string(data) != want {
		t.Fatalf("file content mismatch\n--- got ---\n%s\n--- want ---\n%s", data, want)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("failed to read directory: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected exactly one file left in the directory, got %d", len(entries))
	}
}

func TestLintFileCheckModeDoesNotWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "messages_xx.txt")
	original := []byte("[b]\nz=one\na=two\n")

	if err := os.WriteFile(path, original, 0o644); err != nil {
		t.Fatalf("failed to write test fixture: %v", err)
	}

	result, err := lintFile(path, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.changed {
		t.Fatal("expected the file to be reported as needing a change")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	if string(data) != string(original) {
		t.Fatal("check mode must not modify the file on disk")
	}
}

func TestLintFileFatalErrorLeavesFileUntouched(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "messages_xx.txt")
	original := []byte("[foo]\nnoequals\n")

	if err := os.WriteFile(path, original, 0o644); err != nil {
		t.Fatalf("failed to write test fixture: %v", err)
	}

	if _, err := lintFile(path, false); err == nil {
		t.Fatal("expected an error for a malformed line")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	if string(data) != string(original) {
		t.Fatal("a fatal parse error must leave the original file untouched")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("failed to read directory: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected no stray temp/backup files, got %d entries", len(entries))
	}
}
