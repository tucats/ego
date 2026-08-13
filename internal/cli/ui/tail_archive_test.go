package ui

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
)

// These tests cover the /services/admin/log endpoint's "archive" and
// "since"/"until" query parameters: reading past the active log file into
// older rolled-over files and a zip archive, and bounding results by
// timestamp.

// TestTailFiltered_Archive verifies that with Archive set, a request that the
// active log file alone cannot satisfy pulls in this instance's older
// rolled-over log files on disk, and then the configured zip archive,
// newest source first, assembling the final result oldest-to-newest -- and
// that a request the active file alone can satisfy never needs to look
// further.
func TestTailFiltered_Archive(t *testing.T) {
	dir := t.TempDir()

	savedLogFile := logFile
	savedBase := baseLogFileName
	savedCurrent := currentLogFileName
	savedArchive := archiveLogFileName
	savedFormat := LogFormat

	t.Cleanup(func() {
		if logFile != nil {
			logFile.Close()
		}

		logFile = savedLogFile
		baseLogFileName = savedBase
		currentLogFileName = savedCurrent
		archiveLogFileName = savedArchive
		LogFormat = savedFormat
	})

	LogFormat = TextFormat
	baseLogFileName = filepath.Join(dir, "test.log")

	// Oldest generation: rolled over and then archived into the zip.
	zipEntryName := "test_2026-08-12-010000.log"
	zipEntryLines := []string{
		"[2026-08-12 01:00:01] SERVER : zip.line.one",
		"[2026-08-12 01:00:02] SERVER : zip.line.two",
	}

	zipPath := filepath.Join(dir, "archive.zip")
	writeZipArchive(t, zipPath, zipEntryName, zipEntryLines)
	archiveLogFileName = zipPath

	// Middle generation: rolled over, still sitting on disk.
	olderName := "test_2026-08-12-020000.log"
	olderLines := []string{
		"[2026-08-12 02:00:01] SERVER : older.line.one",
		"[2026-08-12 02:00:02] SERVER : older.line.two",
	}

	if err := os.WriteFile(filepath.Join(dir, olderName), []byte(strings.Join(olderLines, "\n")+"\n"), 0600); err != nil {
		t.Fatalf("failed to write older log: %v", err)
	}

	// Newest generation: the active log file.
	activeName := "test_2026-08-12-030000.log"
	activeLines := []string{
		"[2026-08-12 03:00:01] SERVER : active.line.one",
		"[2026-08-12 03:00:02] SERVER : active.line.two",
	}
	activePath := filepath.Join(dir, activeName)

	if err := os.WriteFile(activePath, []byte(strings.Join(activeLines, "\n")+"\n"), 0600); err != nil {
		t.Fatalf("failed to write active log: %v", err)
	}

	activeFile, err := os.Open(activePath)
	if err != nil {
		t.Fatalf("failed to open active log: %v", err)
	}

	logFile = activeFile
	currentLogFileName = activePath

	// Without Archive, only the active file's lines come back -- unchanged
	// single-file behavior.
	lines, err := TailFiltered(10, LogFilter{})
	if err != nil {
		t.Fatalf("TailFiltered without archive: %v", err)
	}

	assertLines(t, lines, activeLines)

	// With Archive, and a count the active file alone cannot satisfy, the
	// on-disk rolled-over file and the zip archive are both pulled in,
	// oldest-to-newest.
	lines, err = TailFiltered(10, LogFilter{Archive: true})
	if err != nil {
		t.Fatalf("TailFiltered with archive: %v", err)
	}

	want := append(append(append([]string{}, zipEntryLines...), olderLines...), activeLines...)
	assertLines(t, lines, want)

	// A count the active file cannot satisfy but the on-disk rolled-over
	// file can should stop there and never touch the zip archive.
	lines, err = TailFiltered(3, LogFilter{Archive: true})
	if err != nil {
		t.Fatalf("TailFiltered with archive and small count: %v", err)
	}

	assertLines(t, lines, []string{olderLines[1], activeLines[0], activeLines[1]})
}

func assertLines(t *testing.T, got, want []string) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("expected %d lines, got %d: %v", len(want), len(got), got)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

func writeZipArchive(t *testing.T, zipPath, entryName string, lines []string) {
	t.Helper()

	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("failed to create zip: %v", err)
	}

	defer f.Close()

	zw := zip.NewWriter(f)

	entry, err := zw.Create(entryName)
	if err != nil {
		t.Fatalf("failed to create zip entry: %v", err)
	}

	if _, err := entry.Write([]byte(strings.Join(lines, "\n") + "\n")); err != nil {
		t.Fatalf("failed to write zip entry: %v", err)
	}

	if err := zw.Close(); err != nil {
		t.Fatalf("failed to close zip writer: %v", err)
	}
}

// TestTailFiltered_DateRange verifies that Since/Until bound results by the
// entry's timestamp, and that an inverted range (Since after Until) is
// rejected rather than silently returning nothing.
func TestTailFiltered_DateRange(t *testing.T) {
	savedFormat := LogFormat
	savedTSFormat := LogTimeStampFormat

	t.Cleanup(func() {
		LogFormat = savedFormat
		LogTimeStampFormat = savedTSFormat
	})

	LogTimeStampFormat = defs.DefaultLogTimestampFormat

	entries := []LogEntry{
		{Timestamp: "2026-08-12 01:00:00", Sequence: 1, Class: "SERVER", Message: "one"},
		{Timestamp: "2026-08-12 02:00:00", Sequence: 2, Class: "SERVER", Message: "two"},
		{Timestamp: "2026-08-12 03:00:00", Sequence: 3, Class: "SERVER", Message: "three"},
	}

	writeTestLog(t, JSONFormat, entries)

	parse := func(value string) time.Time {
		ts, err := time.ParseInLocation(LogTimeStampFormat, value, time.Local)
		if err != nil {
			t.Fatalf("failed to parse test timestamp %q: %v", value, err)
		}

		return ts
	}

	since := parse("2026-08-12 01:30:00")
	until := parse("2026-08-12 02:30:00")

	lines, err := TailFiltered(10, LogFilter{Since: since})
	if err != nil {
		t.Fatalf("TailFiltered with Since: %v", err)
	}

	if len(lines) != 2 {
		t.Fatalf("expected 2 lines at or after %v, got %d: %v", since, len(lines), lines)
	}

	lines, err = TailFiltered(10, LogFilter{Until: until})
	if err != nil {
		t.Fatalf("TailFiltered with Until: %v", err)
	}

	if len(lines) != 2 {
		t.Fatalf("expected 2 lines at or before %v, got %d: %v", until, len(lines), lines)
	}

	lines, err = TailFiltered(10, LogFilter{Since: since, Until: until})
	if err != nil {
		t.Fatalf("TailFiltered with Since and Until: %v", err)
	}

	if len(lines) != 1 {
		t.Fatalf("expected 1 line between %v and %v, got %d: %v", since, until, len(lines), lines)
	}

	if _, err := TailFiltered(10, LogFilter{Since: until, Until: since}); err == nil {
		t.Fatalf("expected an error for an inverted date range")
	} else if !errors.Equals(err, errors.ErrInvalidLogDateRange) {
		t.Fatalf("expected ErrInvalidLogDateRange, got %v", err)
	}
}
