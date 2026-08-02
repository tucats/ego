package ui

import (
	"path"
	"strings"

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
	return len(f.Classes) > 0 || f.Message != ""
}

// IsEmpty reports whether this filter selects everything, letting callers skip
// the per-line work entirely.
func (f LogFilter) IsEmpty() bool {
	return f.Session <= 0 && len(f.Classes) == 0 && f.Message == ""
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

	return true
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
