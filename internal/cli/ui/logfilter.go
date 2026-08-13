package ui

import (
	"path"
	"strings"
	"time"

	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
)

// LogFilter describes which log entries a Tail operation should return. A zero
// LogFilter selects everything, so callers only set the fields they care about.
//
// Filtering is deliberately done here, against the JSON entry as it was written
// to the log file, rather than against the display text produced later by
// FormatJSONLogEntryAsText. Two reasons: the display text is localized, so a
// pattern that matched in English would stop matching for a caller who asked
// for French; and the display text has already had the message's arguments
// substituted into it, so the message identity is no longer separable from the
// data. The identifier in the "msg" field is the stable thing to match on.
type LogFilter struct {
	// Session restricts results to a single log session ID. Zero means every
	// session.
	Session int

	// Classes restricts results to these logger classes ("REST", "AUTH", ...).
	// Names are compared without regard to case. An empty or nil slice means
	// every class.
	Classes []string

	// Message is a glob pattern matched against the message identifier, such
	// as "rest.*" or "*.error". An empty string means every message.
	Message string

	// Since restricts results to entries at or after this time. The zero
	// value means no lower bound.
	Since time.Time

	// Until restricts results to entries at or before this time. The zero
	// value means no upper bound.
	Until time.Time

	// Archive, when true, extends the search past the active log file to
	// this instance's older rolled-over log files still on disk, and then
	// into the configured zip archive if one exists, oldest source
	// consulted last, stopping as soon as enough lines have been found to
	// satisfy the requested count. It does not affect which individual
	// lines pass the filter, only how many sources are searched, so it is
	// deliberately left out of IsEmpty.
	Archive bool

	// ServerID is a glob pattern (e.g. "da35*" or "*34fd") matched against
	// the writing server's instance UUID. An empty string means every
	// server. This only means anything together with Archive: the active
	// log file is written by exactly one running process, so every entry in
	// it already shares the same ID -- filtering by it can only narrow
	// anything down once older generations (which may carry a different ID,
	// e.g. after a restart) are also being searched. Validate rejects a
	// ServerID set without Archive rather than silently ignoring it.
	ServerID string
}

// NeedsStructuredLog reports whether this filter depends on fields that only
// exist in a JSON-format log file.
//
// The session filter is not included: it has a (crude but working) fallback for
// text logs, matching the "[42]" that the text formatter writes into the line.
// Class and message have no such fallback -- a text log line contains the
// localized message, not the identifier, and matching on that would silently
// mean something different than what the caller asked for. Callers use this to
// refuse the request instead.
func (f LogFilter) NeedsStructuredLog() bool {
	return len(f.Classes) > 0 || f.Message != "" || f.ServerID != ""
}

// IsEmpty reports whether this filter selects everything, letting callers skip
// the per-line work entirely.
func (f LogFilter) IsEmpty() bool {
	return f.Session <= 0 && len(f.Classes) == 0 && f.Message == "" && f.ServerID == "" &&
		f.Since.IsZero() && f.Until.IsZero()
}

// SplitClassList turns a comma-separated list of logger class names into the
// slice a LogFilter wants. Empty items are dropped, so a stray trailing comma
// or a run of spaces does not turn into a class name that matches nothing.
//
// An entirely empty string yields a nil slice, which a LogFilter reads as "every
// class" -- so a caller can pass whatever the client sent without first checking
// whether the client sent anything at all.
func SplitClassList(list string) []string {
	classes := []string{}

	for _, class := range strings.Split(list, ",") {
		if class = strings.TrimSpace(class); class != "" {
			classes = append(classes, class)
		}
	}

	if len(classes) == 0 {
		return nil
	}

	return classes
}

// Validate checks the filter for problems that should be reported to the caller
// as a bad request rather than silently matching nothing.
//
// A misspelled logger name and a malformed glob pattern are both easy mistakes,
// and both would otherwise produce an empty log with no explanation. Catching
// them up front turns a confusing empty result into a specific message.
func (f LogFilter) Validate() error {
	for _, class := range f.Classes {
		if LoggerByName(strings.TrimSpace(class)) == NoSuchLogger {
			return errors.ErrInvalidLoggerName.Context(class)
		}
	}

	if f.Message != "" {
		// path.Match reports a bad pattern (an unclosed character class, say)
		// only when it actually tries to match, so probe it with a throwaway
		// subject to find out now rather than once per log line.
		if _, err := path.Match(f.Message, ""); err != nil {
			return errors.ErrInvalidLogPattern.Context(f.Message)
		}
	}

	if f.ServerID != "" {
		if _, err := path.Match(f.ServerID, ""); err != nil {
			return errors.ErrInvalidLogServerIDPattern.Context(f.ServerID)
		}

		if !f.Archive {
			return errors.ErrLogServerIDNeedsArchive.Context(f.ServerID)
		}
	}

	if !f.Since.IsZero() && !f.Until.IsZero() && f.Since.After(f.Until) {
		return errors.ErrInvalidLogDateRange.Context(f.Since.Format(time.RFC3339) + " > " + f.Until.Format(time.RFC3339))
	}

	return nil
}

// matchesEntry reports whether one parsed JSON log entry passes the filter.
// Every condition set on the filter must match; unset conditions match anything.
func (f LogFilter) matchesEntry(entry *LogEntry) bool {
	if f.Session > 0 && entry.Session != f.Session {
		return false
	}

	if len(f.Classes) > 0 && !matchesAnyClass(entry.Class, f.Classes) {
		return false
	}

	if f.Message != "" && !matchesPattern(f.Message, entry.Message) {
		return false
	}

	if f.ServerID != "" && !matchesPattern(f.ServerID, entry.ID) {
		return false
	}

	if !f.Since.IsZero() || !f.Until.IsZero() {
		// A timestamp that fails to parse is not held against the entry: the
		// field is always written by this same logger, so a parse failure
		// means the timestamp format changed underfoot, not that the entry
		// is out of range.
		if ts, ok := parseLogTimestamp(entry.Timestamp); ok && !inTimeRange(ts, f) {
			return false
		}
	}

	return true
}

// inTimeRange reports whether ts falls within the filter's [Since, Until]
// bound, treating a zero value on either end as unbounded.
func inTimeRange(ts time.Time, f LogFilter) bool {
	if !f.Since.IsZero() && ts.Before(f.Since) {
		return false
	}

	if !f.Until.IsZero() && ts.After(f.Until) {
		return false
	}

	return true
}

// parseLogTimestamp parses a timestamp using the same layout the logger used
// to write it (LogTimeStampFormat, defaulting to "2006-01-02 15:04:05").
func parseLogTimestamp(value string) (time.Time, bool) {
	format := LogTimeStampFormat
	if format == "" {
		format = defs.DefaultLogTimestampFormat
	}

	ts, err := time.ParseInLocation(format, value, time.Local)

	return ts, err == nil
}

// matchesAnyClass reports whether a log entry's class is one of the named
// classes. The class is written into the log file as a name string, and the
// case it is written in has varied, so the comparison ignores case.
func matchesAnyClass(entryClass string, classes []string) bool {
	for _, class := range classes {
		if strings.EqualFold(strings.TrimSpace(class), entryClass) {
			return true
		}
	}

	return false
}

// matchesPattern reports whether a message identifier matches a glob pattern.
//
// path.Match (rather than filepath.Match) is used deliberately: filepath.Match
// treats the platform's path separator as special, which would make the same
// pattern behave differently on Windows than on Unix. path.Match always treats
// "/" as the separator, and message identifiers are dotted names containing no
// slashes at all, so "*" spans the whole identifier as a reader would expect.
//
// Matching ignores case, so "REST.*" and "rest.*" behave the same. A pattern
// that fails to compile matches nothing; Validate reports that to the caller
// before any of this runs.
func matchesPattern(pattern, message string) bool {
	matched, err := path.Match(strings.ToLower(pattern), strings.ToLower(message))

	return err == nil && matched
}
