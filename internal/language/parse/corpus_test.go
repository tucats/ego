package parse

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// corpusRoots are directories, relative to the repository root, containing real
// Ego source used as a broad parser regression corpus. Files under tests/ are
// bare-statement fragments (@test blocks and the like); files under lib are
// full programs. We parse each with ParseStatements, which is the more lenient
// entry point (executable statements permitted at top level), so both shapes
// are accepted.
var corpusRoots = []string{
	"tests",
	"lib/packages",
}

// TestCorpusParses parses every .ego file under the corpus roots and reports the
// success rate. It is a coverage/regression signal for the parser against real
// Ego source. It fails only if the number of files that fail to parse exceeds
// maxCorpusFailures, so the count can be ratcheted down as the parser improves
// without the test flapping on a single new grammar wrinkle.
//
// Run "go test -run TestCorpusParses -v ./internal/language/parse/" to see the
// per-file failures during development.
func TestCorpusParses(t *testing.T) {
	const maxCorpusFailures = 0

	root := repoRoot(t)

	var files []string

	for _, dir := range corpusRoots {
		abs := filepath.Join(root, dir)

		err := filepath.Walk(abs, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil // skip unreadable entries
			}

			if !info.IsDir() && strings.HasSuffix(path, ".ego") {
				files = append(files, path)
			}

			return nil
		})
		if err != nil {
			t.Logf("walking %s: %v", abs, err)
		}
	}

	if len(files) == 0 {
		t.Skip("no corpus .ego files found; skipping")
	}

	sort.Strings(files)

	var failures []string

	for _, path := range files {
		src, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		if _, err := ParseStatements(string(src)); err != nil {
			rel, _ := filepath.Rel(root, path)
			failures = append(failures, rel+": "+err.Error())
		}
	}

	passed := len(files) - len(failures)
	t.Logf("corpus: %d/%d files parsed (%.1f%%)", passed, len(files),
		100*float64(passed)/float64(len(files)))

	if len(failures) > 0 {
		sort.Strings(failures)

		shown := failures
		if len(shown) > 40 {
			shown = shown[:40]
		}

		t.Logf("first %d parse failures:\n%s", len(shown), strings.Join(shown, "\n"))
	}

	if len(failures) > maxCorpusFailures {
		t.Errorf("corpus parse failures: %d (allowed %d)", len(failures), maxCorpusFailures)
	}
}

// repoRoot locates the repository root by walking up from the test's working
// directory until it finds a go.mod file.
func repoRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not locate repo root (no go.mod found)")
		}

		dir = parent
	}
}
