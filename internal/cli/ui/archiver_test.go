package ui

import (
	"archive/zip"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestAddToLogArchive(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "archiver_test")
	if err != nil {
		t.Fatalf("Failed to create temporary directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Set the archive log file name
	archiveName := filepath.Join(tempDir, "archive.zip")
	SetArchive(archiveName)

	// Create two test files
	testFile1 := filepath.Join(tempDir, "test1.txt")
	testFile2 := filepath.Join(tempDir, "test2.txt")

	err = os.WriteFile(testFile1, []byte("test content 1"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	err = os.WriteFile(testFile2, []byte("test content 2"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Add the first file to the (non-existent) archive
	err = addToLogArchive(testFile1)
	if err != nil {
		t.Fatalf("addToLogArchive failed: %v", err)
	}

	// Verify that the file was added to the archive
	updatedArchive, err := zip.OpenReader(archiveName)
	if err != nil {
		t.Fatalf("Failed to open updated archive: %v", err)
	}
	defer updatedArchive.Close()

	found := false

	for _, file := range updatedArchive.File {
		if file.Name == filepath.Base(testFile1) {
			found = true

			break
		}
	}

	if !found {
		t.Errorf("File not found in updated archive: %s", filepath.Base(testFile1))
	}

	// Add the second file to the existing archive
	err = addToLogArchive(testFile2)
	if err != nil {
		t.Fatalf("addToLogArchive failed: %v", err)
	}

	// Verify that the file was added to the archive
	updatedArchive, err = zip.OpenReader(archiveName)
	if err != nil {
		t.Fatalf("Failed to open updated archive: %v", err)
	}
	defer updatedArchive.Close()

	found = false

	for _, file := range updatedArchive.File {
		if file.Name == filepath.Base(testFile2) {
			found = true

			break
		}
	}

	if !found {
		t.Errorf("File not found in updated archive: %s", filepath.Base(testFile2))
	}
}

// TestAddToLogArchiveConcurrent simulates multiple cluster node processes
// rolling over their logs into the same shared archive at once. Without the
// archive lock, concurrent read-modify-rename cycles race and lose entries;
// with it, every file added by every goroutine must end up in the final zip.
func TestAddToLogArchiveConcurrent(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "archiver_concurrent_test")
	if err != nil {
		t.Fatalf("Failed to create temporary directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	archiveName := filepath.Join(tempDir, "archive.zip")
	SetArchive(archiveName)

	const writers = 8

	var wg sync.WaitGroup

	errs := make([]error, writers)

	for i := 0; i < writers; i++ {
		fileName := filepath.Join(tempDir, "node-"+string(rune('a'+i))+".log")
		if err := os.WriteFile(fileName, []byte("log contents"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		wg.Add(1)

		go func(i int, fileName string) {
			defer wg.Done()

			errs[i] = addToLogArchive(fileName)
		}(i, fileName)
	}

	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("addToLogArchive goroutine %d failed: %v", i, err)
		}
	}

	updatedArchive, err := zip.OpenReader(archiveName)
	if err != nil {
		t.Fatalf("Failed to open updated archive: %v", err)
	}
	defer updatedArchive.Close()

	if len(updatedArchive.File) != writers {
		names := make([]string, 0, len(updatedArchive.File))
		for _, f := range updatedArchive.File {
			names = append(names, f.Name)
		}

		t.Errorf("expected %d entries in archive, got %d: %v", writers, len(updatedArchive.File), names)
	}
}
