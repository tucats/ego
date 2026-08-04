package app

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSafeJoin checks the guard that turns a name read out of a ZIP archive
// into a path on this machine.
//
// The names inside an archive are just strings that whoever built the archive
// chose. Nothing in the ZIP format prevents one from being an absolute path,
// or from using ".." to climb out of the directory the caller asked to
// extract into. A program that trusts those names will cheerfully overwrite
// files anywhere the user running it can write; the mistake is common enough
// to have a name, "zip slip". These cases confirm the guard rejects the
// dangerous shapes and accepts the ordinary ones.
func TestSafeJoin(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "tmp", "extract")

	tests := []struct {
		name    string
		entry   string
		want    string
		wantErr bool
	}{
		{
			name:  "plain file at the top level",
			entry: "defaults.json",
			want:  filepath.Join(root, "defaults.json"),
		},
		{
			name:  "file in a subdirectory",
			entry: "services/count.ego",
			want:  filepath.Join(root, "services", "count.ego"),
		},
		{
			name:  "harmless interior dot-dot that stays inside the root",
			entry: "assets/dashboard/../dashboard.css",
			want:  filepath.Join(root, "assets", "dashboard.css"),
		},
		{
			name:    "empty name",
			entry:   "",
			wantErr: true,
		},
		{
			name:    "leading dot-dot escapes the root",
			entry:   "../escaped.txt",
			wantErr: true,
		},
		{
			name:    "several leading dot-dots, as the old archives contained",
			entry:   "../../../lib/services/count.ego",
			wantErr: true,
		},
		{
			name:    "interior dot-dot that climbs out of the root",
			entry:   "services/../../escaped.txt",
			wantErr: true,
		},
		{
			name:    "absolute path",
			entry:   "/etc/passwd",
			wantErr: true,
		},
		{
			name:    "dot-dot alone",
			entry:   "..",
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := safeJoin(root, test.entry)

			if test.wantErr {
				if err == nil {
					t.Errorf("safeJoin(%q) should have been rejected, but returned %q", test.entry, got)
				}

				return
			}

			if err != nil {
				t.Errorf("safeJoin(%q) returned an unexpected error: %v", test.entry, err)

				return
			}

			if got != test.want {
				t.Errorf("safeJoin(%q)\n  got:  %s\n  want: %s", test.entry, got, test.want)
			}
		})
	}
}

// TestIsEmptyDir checks the test that decides whether a library directory
// that already exists still needs to be populated.
func TestIsEmptyDir(t *testing.T) {
	// t.TempDir creates a directory that Go deletes automatically when the
	// test finishes, so the test leaves nothing behind.
	base := t.TempDir()

	empty := filepath.Join(base, "empty")
	if err := os.Mkdir(empty, directoryPerm); err != nil {
		t.Fatal(err)
	}

	if got, err := isEmptyDir(empty); err != nil || !got {
		t.Errorf("an empty directory should report empty; got %v, err %v", got, err)
	}

	full := filepath.Join(base, "full")
	if err := os.Mkdir(full, directoryPerm); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(full, "marker.txt"), []byte("x"), filePerm); err != nil {
		t.Fatal(err)
	}

	if got, err := isEmptyDir(full); err != nil || got {
		t.Errorf("a directory with a file in it should not report empty; got %v, err %v", got, err)
	}

	// A directory holding only another directory is still not empty.
	nested := filepath.Join(base, "nested")
	if err := os.MkdirAll(filepath.Join(nested, "child"), directoryPerm); err != nil {
		t.Fatal(err)
	}

	if got, err := isEmptyDir(nested); err != nil || got {
		t.Errorf("a directory with a subdirectory should not report empty; got %v, err %v", got, err)
	}

	if _, err := isEmptyDir(filepath.Join(base, "does-not-exist")); err == nil {
		t.Error("a missing directory should report an error")
	}
}

// TestEmbeddedArchive checks that the archive compiled into this executable
// is a readable ZIP file whose entry names are the clean, relative, forward
// slash separated names the extraction code expects.
//
// This is the test that would have caught the old archives, whose entries
// were named things like "../../../lib/services/count.ego".
func TestEmbeddedArchive(t *testing.T) {
	r, err := zip.NewReader(strings.NewReader(zipdata), int64(len(zipdata)))
	if err != nil {
		t.Fatalf("the embedded library archive could not be read: %v", err)
	}

	if len(r.File) == 0 {
		t.Fatal("the embedded library archive is empty")
	}

	root := t.TempDir()

	for _, f := range r.File {
		if strings.Contains(f.Name, `\`) {
			t.Errorf("entry %q contains a backslash; ZIP entry names must use forward slashes", f.Name)
		}

		if _, err := safeJoin(root, f.Name); err != nil {
			t.Errorf("entry %q would be rejected as unsafe: %v", f.Name, err)
		}

		// The private key material named in the go:generate directive's
		// --omit option must never end up inside a shipped executable.
		if base := filepath.Base(f.Name); base == "https-server.key" || base == "https-server.crt" {
			t.Errorf("entry %q is a certificate file and should have been omitted from the archive", f.Name)
		}
	}
}

// TestInstallLibrary walks the whole installation through, into a scratch
// directory, and confirms that what lands on disk matches what is in the
// archive.
func TestInstallLibrary(t *testing.T) {
	root := filepath.Join(t.TempDir(), "lib")

	if err := installLibraryAtomically(root); err != nil {
		t.Fatalf("installing the library failed: %v", err)
	}

	// Every entry in the archive should now exist on disk, at exactly the
	// path the archive named, with exactly the right size.
	r, err := zip.NewReader(strings.NewReader(zipdata), int64(len(zipdata)))
	if err != nil {
		t.Fatal(err)
	}

	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}

		path := filepath.Join(root, filepath.FromSlash(f.Name))

		info, err := os.Stat(path)
		if err != nil {
			t.Errorf("%s was not extracted: %v", f.Name, err)

			continue
		}

		if info.Size() != int64(f.UncompressedSize64) { //nolint:gosec // sizes in this archive are well under a megabyte
			t.Errorf("%s is %d bytes on disk but %d bytes in the archive",
				f.Name, info.Size(), f.UncompressedSize64)
		}
	}

	// The staging directory used during installation must not survive.
	entries, err := os.ReadDir(filepath.Dir(root))
	if err != nil {
		t.Fatal(err)
	}

	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".lib-install-") {
			t.Errorf("a staging directory was left behind: %s", entry.Name())
		}
	}
}

// TestInstallNeeded covers the decision about whether the library has to be
// unpacked at all.
func TestInstallNeeded(t *testing.T) {
	base := t.TempDir()

	// A path that does not exist needs installing.
	if needed, err := installNeeded(filepath.Join(base, "missing")); err != nil || !needed {
		t.Errorf("a missing directory should need installing; got %v, err %v", needed, err)
	}

	// A directory that exists but is empty needs installing. This is the
	// case that a plain "does it exist" check gets wrong, leaving a library
	// that was interrupted partway through installation broken forever.
	empty := filepath.Join(base, "empty")
	if err := os.Mkdir(empty, directoryPerm); err != nil {
		t.Fatal(err)
	}

	if needed, err := installNeeded(empty); err != nil || !needed {
		t.Errorf("an empty directory should need installing; got %v, err %v", needed, err)
	}

	// A directory with something in it is left alone.
	full := filepath.Join(base, "full")
	if err := os.Mkdir(full, directoryPerm); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(full, "defaults.json"), []byte("{}"), filePerm); err != nil {
		t.Fatal(err)
	}

	if needed, err := installNeeded(full); err != nil || needed {
		t.Errorf("a populated directory should not need installing; got %v, err %v", needed, err)
	}

	// A plain file where the library directory should be is an error, not
	// something to quietly unpack around.
	file := filepath.Join(base, "afile")
	if err := os.WriteFile(file, []byte("x"), filePerm); err != nil {
		t.Fatal(err)
	}

	if _, err := installNeeded(file); err == nil {
		t.Error("a file in place of the library directory should be an error")
	}
}

// TestInstallLibraryNoReplace confirms that extracting with replace set to
// false leaves files that are already there untouched.
func TestInstallLibraryNoReplace(t *testing.T) {
	root := t.TempDir()

	marker := []byte("do not overwrite me")
	if err := os.WriteFile(filepath.Join(root, "defaults.json"), marker, filePerm); err != nil {
		t.Fatal(err)
	}

	if err := InstallLibrary(root, false); err != nil {
		t.Fatalf("installing the library failed: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(root, "defaults.json"))
	if err != nil {
		t.Fatal(err)
	}

	if string(got) != string(marker) {
		t.Error("an existing file was overwritten even though replace was false")
	}

	// Files that were not already present should still have been extracted.
	if _, err := os.Stat(filepath.Join(root, "services")); err != nil {
		t.Errorf("the rest of the library was not extracted: %v", err)
	}
}
