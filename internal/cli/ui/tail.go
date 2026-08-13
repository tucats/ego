package ui

import (
	"archive/zip"
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"time"

	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
)

// Tail returns the last count lines of the log, optionally restricted to a
// single session ID. Pass 0 for session to return lines from every session.
//
// This is the long-standing form of the call, kept so existing callers do not
// have to change. Anything needing to filter by logger class or message
// identifier calls TailFiltered instead.
func Tail(count int, session int) ([]string, error) {
	return TailFiltered(count, LogFilter{Session: session})
}

// TailFiltered returns the last count lines of the log that pass the given
// filter.
//
// Note the order of operations: the filter is applied to the whole file first,
// and only then are the last count lines taken from what survived. That way
// "the last 50 REST lines" means 50 REST lines, not "whichever REST lines
// happen to fall inside the last 50 lines of the file" -- which on a busy
// server would usually be none at all.
func TailFiltered(count int, filter LogFilter) ([]string, error) {
	if err := filter.Validate(); err != nil {
		return nil, err
	}

	// Class and message filtering read fields that only a JSON-format log
	// records. Rather than quietly returning something that does not match what
	// was asked for, refuse the request and say why.
	if filter.NeedsStructuredLog() && LogFormat != JSONFormat {
		return nil, errors.ErrLogFilterNeedsJSON.Context(LogFormat)
	}

	return tailFile(count, filter)
}

func tailFile(count int, filter LogFilter) ([]string, error) {
	// If there is no active log file, manufacture empty log message saying so.
	if logFile == nil {
		if count > 3 {
			count = 3
		}

		if count == 0 {
			count = 1
		}

		if LogTimeStampFormat == "" {
			LogTimeStampFormat = defs.DefaultLogTimestampFormat
		}

		result := []string{}

		for i := range count {
			var entry struct {
				Time  string `json:"time"`
				ID    string `json:"id"`
				Seq   int    `json:"seq"`
				Class string `json:"class"`
				Msg   string `json:"msg"`
			}

			entry.Time = time.Now().Format(LogTimeStampFormat)
			entry.ID = defs.InstanceID
			entry.Seq = i + 1
			entry.Class = "server"
			entry.Msg = "no.log"

			b, _ := json.Marshal(entry)
			result = append(result, string(b))
		}

		return result, nil
	}

	file, err := os.OpenFile(logFile.Name(), os.O_RDONLY, 0700)
	if err != nil {
		return nil, err
	}

	activeLines, err := scanFilteredLines(file, filter)

	file.Close()

	if err != nil {
		return nil, err
	}

	result, remaining := takeNewest(nil, activeLines, count)

	// The active log file alone satisfied the request, or the caller did not
	// ask to look any further back.
	if remaining <= 0 || !filter.Archive {
		return result, nil
	}

	// Walk this instance's rolled-over log files still on disk, newest to
	// oldest, filling in older lines until either the count is satisfied or
	// there are no more files to consult.
	dir, names := olderLogFileNames()

	for i := len(names) - 1; i >= 0 && remaining > 0; i-- {
		lines, err := scanFilteredLinesInFile(path.Join(dir, names[i]), filter)
		if err != nil {
			// A file that vanished or became unreadable between listing and
			// opening (a concurrent rollover or purge) should not fail the
			// whole request -- just move on to the next, older, file.
			Log(InfoLogger, "logging.tail.archive.error", A{
				"filename": names[i],
				"error":    err})

			continue
		}

		result, remaining = takeNewest(result, lines, remaining)
	}

	// If a zip archive is configured, keep going into it, again newest entry
	// first, until the count is satisfied or the archive is exhausted.
	if remaining > 0 {
		zipReader, entries, err := archivedLogEntries()
		if err != nil {
			return nil, err
		}

		if zipReader != nil {
			defer zipReader.Close()

			for _, entry := range entries {
				if remaining <= 0 {
					break
				}

				lines, err := scanFilteredLinesInZipEntry(entry, filter)
				if err != nil {
					Log(InfoLogger, "logging.tail.archive.error", A{
						"filename": entry.Name,
						"error":    err})

					continue
				}

				result, remaining = takeNewest(result, lines, remaining)
			}
		}
	}

	return result, nil
}

// scanFilteredLines reads every line from reader, keeping only those that
// pass filter, in the order they were read.
func scanFilteredLines(reader io.Reader, filter LogFilter) ([]string, error) {
	lines := []string{}
	scanner := bufio.NewScanner(reader)

	scanner.Split(bufio.ScanLines)

	for scanner.Scan() {
		line := scanner.Text()

		if !filter.IsEmpty() && !keepLine(line, filter) {
			continue
		}

		lines = append(lines, line)
	}

	return lines, scanner.Err()
}

func scanFilteredLinesInFile(fileName string, filter LogFilter) ([]string, error) {
	file, err := os.OpenFile(fileName, os.O_RDONLY, 0700)
	if err != nil {
		return nil, err
	}

	defer file.Close()

	return scanFilteredLines(file, filter)
}

func scanFilteredLinesInZipEntry(entry *zip.File, filter LogFilter) ([]string, error) {
	reader, err := entry.Open()
	if err != nil {
		return nil, err
	}

	defer reader.Close()

	return scanFilteredLines(reader, filter)
}

// takeNewest prepends the newest `remaining` lines of `older` (an
// already-filtered, oldest-to-newest slice from a source that chronologically
// precedes everything in `existing`) onto `existing`, and reports how many
// more lines are still needed afterward.
//
// This is how a request that spans several files is assembled: each source is
// visited newest first, but within a source lines stay in the order they were
// written, so only the tail of an older source -- the lines immediately
// preceding what has already been collected -- is ever needed.
func takeNewest(existing []string, older []string, remaining int) ([]string, int) {
	if len(older) > remaining {
		older = older[len(older)-remaining:]
	}

	return append(older, existing...), remaining - len(older)
}

// keepLine reports whether one raw line from the log file passes the filter.
//
// The common case is a JSON-format log, where the line parses into a LogEntry
// and every part of the filter can be checked against a real field. Note that a
// line which fails to parse is kept rather than dropped: log files can contain
// text that the logger did not write (a panic trace, say), and silently hiding
// it would be worse than showing a line the filter cannot judge.
//
// A text-format log has no fields to check. Only the session filter can reach
// this point in that case -- TailFiltered has already refused class and message
// filters against a text log -- so this falls back to looking for the "[42]"
// that the text formatter writes into the line.
func keepLine(line string, filter LogFilter) bool {
	if LogFormat == JSONFormat {
		var entry LogEntry

		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			return true
		}

		return filter.matchesEntry(&entry)
	}

	if filter.Session > 0 && !strings.Contains(line, fmt.Sprintf(": [%d] ", filter.Session)) {
		return false
	}

	if !filter.Since.IsZero() || !filter.Until.IsZero() {
		// A text-format line starts with "[<timestamp>] ...". A line the
		// filter cannot judge (no leading bracket, or a value that does not
		// parse) is kept rather than dropped, for the same reason a line
		// that fails JSON parsing is kept above.
		if ts, ok := leadingTimestamp(line); ok && !inTimeRange(ts, filter) {
			return false
		}
	}

	return true
}

// leadingTimestamp extracts and parses the timestamp FormatLogMessage writes
// at the start of every text-format log line: "[2006-01-02 15:04:05] ...".
func leadingTimestamp(line string) (time.Time, bool) {
	if !strings.HasPrefix(line, "[") {
		return time.Time{}, false
	}

	end := strings.Index(line, "]")
	if end < 0 {
		return time.Time{}, false
	}

	return parseLogTimestamp(line[1:end])
}
