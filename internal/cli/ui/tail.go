package ui

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
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
			LogTimeStampFormat = "2006-01-02 15:04:05"
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

	defer file.Close()

	text := []string{}
	scanner := bufio.NewScanner(file)

	scanner.Split(bufio.ScanLines)

	for scanner.Scan() {
		line := scanner.Text()

		if !filter.IsEmpty() && !keepLine(line, filter) {
			continue
		}

		text = append(text, line)
	}

	// IF the scanner choked on an error, bail out.
	if e := scanner.Err(); e != nil {
		return nil, e
	}

	position := len(text) - count
	if position < 0 {
		position = 0
	}

	return text[position:], nil
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

	if filter.Session > 0 {
		return strings.Contains(line, fmt.Sprintf(": [%d] ", filter.Session))
	}

	return true
}
