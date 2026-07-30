package ui

import (
	"os"
	"path/filepath"
	"testing"
)

// TestPurgeLogsIgnoresOtherInstances is a regression test for a standalone
// server sharing a log directory with cluster member(s). A standalone
// server's log stem (e.g. "ego-server_") is a plain string prefix of a
// cluster member's qualified stem (e.g. "ego-server_gang_8501_"), so a naive
// prefix match would let the standalone instance's purge/archive pass scoop
// up and delete another instance's log files. PurgeLogs must only ever touch
// files that match its own instance's exact naming pattern.
func TestPurgeLogsIgnoresOtherInstances(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "logfile_purge_test")
	if err != nil {
		t.Fatalf("Failed to create temporary directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Save and restore package-level state used by PurgeLogs.
	savedBase := baseLogFileName
	savedCurrent := currentLogFileName
	savedArchive := archiveLogFileName
	savedRetain := LogRetainCount
	savedLogFile := logFile

	defer func() {
		baseLogFileName = savedBase
		currentLogFileName = savedCurrent
		archiveLogFileName = savedArchive
		LogRetainCount = savedRetain
		logFile = savedLogFile
	}()

	archiveLogFileName = ""
	LogRetainCount = 1

	// This process is a standalone (non-cluster) server using the default
	// log name, sharing the directory with a "gang" cluster's node logs.
	baseLogFileName = filepath.Join(tempDir, "ego-server.log")
	currentLogFileName = filepath.Join(tempDir, "ego-server_2024-01-01-000003.log")

	// CurrentLogFile() (and thus PurgeLogs' search directory) only reports
	// currentLogFileName while a log file handle is open, so open one here.
	openCurrent, err := os.Create(currentLogFileName)
	if err != nil {
		t.Fatalf("Failed to create current log file: %v", err)
	}
	defer openCurrent.Close()

	logFile = openCurrent

	standaloneOld := []string{
		"ego-server_2024-01-01-000001.log",
		"ego-server_2024-01-01-000002.log",
	}
	clusterFiles := []string{
		"ego-server_gang_8501_2024-01-01-000001.log",
		"ego-server_gang_8501_2024-01-01-000002.log",
		"ego-server_gang_8502_2024-01-01-000001.log",
	}

	for _, name := range append(append([]string{}, standaloneOld...), clusterFiles...) {
		if err := os.WriteFile(filepath.Join(tempDir, name), []byte("log"), 0644); err != nil {
			t.Fatalf("Failed to create test file %s: %v", name, err)
		}
	}
	
	PurgeLogs()

	// The cluster member's log files must be completely untouched.
	for _, name := range clusterFiles {
		if _, err := os.Stat(filepath.Join(tempDir, name)); err != nil {
			t.Errorf("cluster member log file %s was removed by standalone server's purge: %v", name, err)
		}
	}

	// With retain=1 and two older standalone rollovers (plus the current
	// file not counted), the oldest standalone file should have been purged.
	if _, err := os.Stat(filepath.Join(tempDir, standaloneOld[0])); !os.IsNotExist(err) {
		t.Errorf("expected oldest standalone log file %s to be purged, stat err = %v", standaloneOld[0], err)
	}
}
