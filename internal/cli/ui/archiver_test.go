package ui

import (
	"archive/zip"
	"os"
	"path/filepath"
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
