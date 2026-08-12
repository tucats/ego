package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/ego/internal/errors"
)

// These tests cover the log filtering that backs the /services/admin/log
// endpoint's session, class, and msg query parameters.
//
// The important thing being pinned down here is that filtering happens against
// the JSON entry as written to the log file -- the message IDENTIFIER, such as
// "rest.server.log" -- and not against the localized display text the client
// eventually sees. If that ever changed, a pattern that worked for an English
// client would silently stop matching for a French one.

// writeTestLog points the package's log file at a temp file containing the given
// entries, and restores the previous logging state when the test ends. It
// returns nothing: the tests call Tail/TailFiltered, which read the file.
func writeTestLog(t *testing.T, format string, entries []LogEntry) {
	t.Helper()

	savedFile := logFile
	savedFormat := LogFormat

	t.Cleanup(func() {
		if logFile != nil {
			logFile.Close()
		}

		logFile = savedFile
		LogFormat = savedFormat
	})

	name := filepath.Join(t.TempDir(), "test.log")

	lines := make([]string, 0, len(entries))

	for _, entry := range entries {
		if format == JSONFormat {
			b, err := json.Marshal(entry)
			if err != nil {
				t.Fatalf("failed to marshal test log entry: %v", err)
			}

			lines = append(lines, string(b))
		} else {
			// The shape the text formatter produces, which is what the session
			// fallback searches for.
			lines = append(lines, "[2026-08-02 14:31:03] "+
				strings.ToUpper(entry.Class)+" : ["+itoa(entry.Session)+"] "+entry.Message)
		}
	}

	if err := os.WriteFile(name, []byte(strings.Join(lines, "\n")+"\n"), 0600); err != nil {
		t.Fatalf("failed to write test log: %v", err)
	}

	file, err := os.Open(name)
	if err != nil {
		t.Fatalf("failed to open test log: %v", err)
	}

	logFile = file
	LogFormat = format
}

// itoa avoids pulling strconv into the test for one call.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}

	digits := ""
	for ; n > 0; n /= 10 {
		digits = string(rune('0'+n%10)) + digits
	}

	return digits
}

// sampleEntries is a small log with two sessions, four classes, and message
// identifiers that share prefixes, so glob patterns have something to
// discriminate between.
func sampleEntries() []LogEntry {
	return []LogEntry{
		{Timestamp: "t1", Sequence: 1, Session: 0, Class: "SERVER", Message: "server.start"},
		{Timestamp: "t2", Sequence: 2, Session: 1, Class: "REST", Message: "rest.request.start"},
		{Timestamp: "t3", Sequence: 3, Session: 1, Class: "REST", Message: "rest.request.end"},
		{Timestamp: "t4", Sequence: 4, Session: 1, Class: "AUTH", Message: "auth.token.valid"},
		{Timestamp: "t5", Sequence: 5, Session: 2, Class: "REST", Message: "rest.request.start"},
		{Timestamp: "t6", Sequence: 6, Session: 2, Class: "SQL", Message: "sql.query"},
	}
}

// messagesOf extracts the message identifier from each returned raw JSON line,
// so tests can state expectations in terms of which entries survived.
func messagesOf(t *testing.T, lines []string) []string {
	t.Helper()

	result := make([]string, 0, len(lines))

	for _, line := range lines {
		var entry LogEntry

		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("returned line is not valid JSON: %q", line)
		}

		result = append(result, entry.Message)
	}

	return result
}

func TestTailFilteredSelectsMatchingEntries(t *testing.T) {
	tests := []struct {
		name   string
		filter LogFilter
		want   []string
	}{
		{
			name:   "no filter returns everything",
			filter: LogFilter{},
			want: []string{"server.start", "rest.request.start", "rest.request.end",
				"auth.token.valid", "rest.request.start", "sql.query"},
		},
		{
			name:   "session",
			filter: LogFilter{Session: 2},
			want:   []string{"rest.request.start", "sql.query"},
		},
		{
			name:   "single class",
			filter: LogFilter{Classes: []string{"REST"}},
			want:   []string{"rest.request.start", "rest.request.end", "rest.request.start"},
		},
		{
			name:   "several classes",
			filter: LogFilter{Classes: []string{"AUTH", "SQL"}},
			want:   []string{"auth.token.valid", "sql.query"},
		},
		{
			name:   "class name ignores case",
			filter: LogFilter{Classes: []string{"rest"}},
			want:   []string{"rest.request.start", "rest.request.end", "rest.request.start"},
		},
		{
			name:   "message glob on a prefix",
			filter: LogFilter{Message: "rest.*"},
			want:   []string{"rest.request.start", "rest.request.end", "rest.request.start"},
		},
		{
			name:   "message glob on a suffix",
			filter: LogFilter{Message: "*.start"},
			want:   []string{"server.start", "rest.request.start", "rest.request.start"},
		},
		{
			name:   "message glob ignores case",
			filter: LogFilter{Message: "REST.REQUEST.END"},
			want:   []string{"rest.request.end"},
		},
		{
			name:   "exact message with no wildcard",
			filter: LogFilter{Message: "sql.query"},
			want:   []string{"sql.query"},
		},
		{
			name:   "question mark matches one character",
			filter: LogFilter{Message: "sq?.query"},
			want:   []string{"sql.query"},
		},
		{
			// Every condition must hold, not any of them.
			name:   "filters combine as AND",
			filter: LogFilter{Session: 1, Classes: []string{"REST"}, Message: "*.end"},
			want:   []string{"rest.request.end"},
		},
		{
			name:   "no matches yields an empty result, not an error",
			filter: LogFilter{Classes: []string{"CACHE"}},
			want:   []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			writeTestLog(t, JSONFormat, sampleEntries())

			lines, err := TailFiltered(100, tt.filter)
			if err != nil {
				t.Fatalf("TailFiltered returned an unexpected error: %v", err)
			}

			got := messagesOf(t, lines)
			if strings.Join(got, ",") != strings.Join(tt.want, ",") {
				t.Errorf("filter %+v\n got: %v\nwant: %v", tt.filter, got, tt.want)
			}
		})
	}
}

// The count must be applied to what survived the filter, not to the raw file.
// Otherwise "the last 2 REST lines" would mean "any REST lines among the last 2
// lines of the file", which on a busy server is usually none at all.
func TestTailFilteredAppliesCountAfterFiltering(t *testing.T) {
	writeTestLog(t, JSONFormat, sampleEntries())

	lines, err := TailFiltered(2, LogFilter{Classes: []string{"REST"}})
	if err != nil {
		t.Fatalf("TailFiltered returned an unexpected error: %v", err)
	}

	got := messagesOf(t, lines)

	// The last two REST entries, which sit at sequence 3 and 5 -- well outside
	// the last two lines of the file.
	want := []string{"rest.request.end", "rest.request.start"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("got %v, want %v", got, want)
	}
}

// A log file can contain lines the logger did not write -- a panic trace, for
// instance. Those cannot be judged against the filter, and hiding them would be
// worse than showing a line that may not match.
func TestTailFilteredKeepsUnparseableLines(t *testing.T) {
	writeTestLog(t, JSONFormat, sampleEntries())

	name := logFile.Name()

	logFile.Close()

	existing, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("failed to read back the test log: %v", err)
	}

	if err := os.WriteFile(name, append(existing, []byte("panic: runtime error\n")...), 0600); err != nil {
		t.Fatalf("failed to append to the test log: %v", err)
	}

	if logFile, err = os.Open(name); err != nil {
		t.Fatalf("failed to reopen the test log: %v", err)
	}

	lines, err := TailFiltered(100, LogFilter{Classes: []string{"SQL"}})
	if err != nil {
		t.Fatalf("TailFiltered returned an unexpected error: %v", err)
	}

	found := false

	for _, line := range lines {
		if strings.Contains(line, "panic: runtime error") {
			found = true
		}
	}

	if !found {
		t.Error("a line that is not valid JSON was dropped by the filter; it should be kept")
	}
}

// Class and message filters read fields only a JSON log has. Asking for them
// against a text-format log must be refused rather than silently ignored.
func TestTailFilteredRejectsStructuredFiltersOnTextLog(t *testing.T) {
	tests := []struct {
		name   string
		filter LogFilter
		wantOK bool
	}{
		{name: "class filter is refused", filter: LogFilter{Classes: []string{"REST"}}},
		{name: "message filter is refused", filter: LogFilter{Message: "rest.*"}},
		{name: "session filter still works", filter: LogFilter{Session: 1}, wantOK: true},
		{name: "no filter still works", filter: LogFilter{}, wantOK: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			writeTestLog(t, TextFormat, sampleEntries())

			_, err := TailFiltered(100, tt.filter)

			if tt.wantOK {
				if err != nil {
					t.Fatalf("expected success, got error: %v", err)
				}

				return
			}

			if err == nil {
				t.Fatal("expected an error for a structured filter on a text log, got none")
			}

			if !errors.Equals(err, errors.ErrLogFilterNeedsJSON) {
				t.Errorf("got error %v, want ErrLogFilterNeedsJSON", err)
			}
		})
	}
}

// A misspelled class or a malformed pattern should be reported, not turned into
// an empty result the caller has to guess the cause of.
func TestTailFilteredValidatesFilter(t *testing.T) {
	tests := []struct {
		name    string
		filter  LogFilter
		wantErr error
	}{
		{
			name:    "unknown logger class",
			filter:  LogFilter{Classes: []string{"NOSUCHCLASS"}},
			wantErr: errors.ErrInvalidLoggerName,
		},
		{
			name:    "one bad name among good ones",
			filter:  LogFilter{Classes: []string{"REST", "NOSUCHCLASS"}},
			wantErr: errors.ErrInvalidLoggerName,
		},
		{
			name:    "malformed glob pattern",
			filter:  LogFilter{Message: "rest.[a-"},
			wantErr: errors.ErrInvalidLogPattern,
		},
		{
			name:    "malformed server ID pattern",
			filter:  LogFilter{ServerID: "da35[a-", Archive: true},
			wantErr: errors.ErrInvalidLogServerIDPattern,
		},
		{
			name:    "server ID without archive",
			filter:  LogFilter{ServerID: "da35*"},
			wantErr: errors.ErrLogServerIDNeedsArchive,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			writeTestLog(t, JSONFormat, sampleEntries())

			_, err := TailFiltered(100, tt.filter)
			if err == nil {
				t.Fatal("expected an error, got none")
			}

			if !errors.Equals(err, tt.wantErr) {
				t.Errorf("got error %v, want %v", err, tt.wantErr)
			}
		})
	}
}

// writeArchiveableTestLog is writeTestLog plus the base/current log file name
// bookkeeping the Archive search path (olderLogFileNames, in logfile.go)
// reads. A ServerID filter requires Archive, so any TailFiltered call in
// these tests exercises that path; without setting these, it would fall back
// to scanning the test binary's own working directory instead of an empty
// temp one.
func writeArchiveableTestLog(t *testing.T, format string, entries []LogEntry) {
	t.Helper()

	writeTestLog(t, format, entries)

	savedBase := baseLogFileName
	savedCurrent := currentLogFileName

	t.Cleanup(func() {
		baseLogFileName = savedBase
		currentLogFileName = savedCurrent
	})

	name := logFile.Name()
	baseLogFileName = name
	currentLogFileName = name
}

// serverIDEntries carries distinct instance UUIDs so a ServerID glob pattern
// has something to discriminate between.
func serverIDEntries() []LogEntry {
	return []LogEntry{
		{Timestamp: "t1", Sequence: 1, Class: "SERVER", Message: "server.start", ID: "da35b6e2-1111-1111-1111-111111111111"},
		{Timestamp: "t2", Sequence: 2, Class: "SERVER", Message: "server.start", ID: "aaaaaaaa-bbbb-cccc-dddd-eeeeeeee34fd"},
		{Timestamp: "t3", Sequence: 3, Class: "SERVER", Message: "server.start", ID: "11112222-3333-4444-5555-666677778888"},
	}
}

func TestTailFilteredServerIDGlob(t *testing.T) {
	tests := []struct {
		name   string
		filter LogFilter
		want   []string
	}{
		{
			name:   "prefix pattern",
			filter: LogFilter{ServerID: "da35*", Archive: true},
			want:   []string{"da35b6e2-1111-1111-1111-111111111111"},
		},
		{
			name:   "suffix pattern",
			filter: LogFilter{ServerID: "*34fd", Archive: true},
			want:   []string{"aaaaaaaa-bbbb-cccc-dddd-eeeeeeee34fd"},
		},
		{
			name:   "exact match with no wildcard",
			filter: LogFilter{ServerID: "11112222-3333-4444-5555-666677778888", Archive: true},
			want:   []string{"11112222-3333-4444-5555-666677778888"},
		},
		{
			name:   "pattern ignores case",
			filter: LogFilter{ServerID: "DA35*", Archive: true},
			want:   []string{"da35b6e2-1111-1111-1111-111111111111"},
		},
		{
			name:   "no matches yields an empty result, not an error",
			filter: LogFilter{ServerID: "zzzz*", Archive: true},
			want:   []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			writeArchiveableTestLog(t, JSONFormat, serverIDEntries())

			lines, err := TailFiltered(100, tt.filter)
			if err != nil {
				t.Fatalf("TailFiltered returned an unexpected error: %v", err)
			}

			got := make([]string, 0, len(lines))

			for _, line := range lines {
				var entry LogEntry

				if err := json.Unmarshal([]byte(line), &entry); err != nil {
					t.Fatalf("returned line is not valid JSON: %q", line)
				}

				got = append(got, entry.ID)
			}

			if strings.Join(got, ",") != strings.Join(tt.want, ",") {
				t.Errorf("filter %+v\n got: %v\nwant: %v", tt.filter, got, tt.want)
			}
		})
	}
}

// A ServerID filter passes Validate (Archive is set and the pattern
// compiles), but still needs a JSON-format log for the same reason class and
// message filters do: a text-format line has no ID field to match against.
func TestTailFilteredServerIDNeedsJSON(t *testing.T) {
	writeArchiveableTestLog(t, TextFormat, serverIDEntries())

	_, err := TailFiltered(100, LogFilter{ServerID: "da35*", Archive: true})
	if !errors.Equals(err, errors.ErrLogFilterNeedsJSON) {
		t.Errorf("got error %v, want %v", err, errors.ErrLogFilterNeedsJSON)
	}
}

// Tail is the older two-argument form, kept for existing callers. It must stay
// equivalent to a session-only filter.
func TestTailRemainsEquivalentToSessionFilter(t *testing.T) {
	writeTestLog(t, JSONFormat, sampleEntries())

	fromTail, err := Tail(100, 1)
	if err != nil {
		t.Fatalf("Tail returned an unexpected error: %v", err)
	}

	fromFiltered, err := TailFiltered(100, LogFilter{Session: 1})
	if err != nil {
		t.Fatalf("TailFiltered returned an unexpected error: %v", err)
	}

	if strings.Join(fromTail, "\n") != strings.Join(fromFiltered, "\n") {
		t.Errorf("Tail and TailFiltered disagree:\n Tail: %v\nFiltered: %v", fromTail, fromFiltered)
	}
}

func TestSplitClassList(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{name: "empty string means every class", in: "", want: nil},
		{name: "only separators means every class", in: " , , ", want: nil},
		{name: "single name", in: "REST", want: []string{"REST"}},
		{name: "several names", in: "REST,AUTH", want: []string{"REST", "AUTH"}},
		{name: "spaces are trimmed", in: " REST , AUTH ", want: []string{"REST", "AUTH"}},
		{name: "trailing comma is ignored", in: "REST,", want: []string{"REST"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SplitClassList(tt.in)

			if len(got) != len(tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}

			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("got %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestLogFilterNeedsStructuredLog(t *testing.T) {
	tests := []struct {
		name   string
		filter LogFilter
		want   bool
	}{
		{name: "empty filter", filter: LogFilter{}, want: false},
		{name: "session only has a text fallback", filter: LogFilter{Session: 3}, want: false},
		{name: "class needs JSON", filter: LogFilter{Classes: []string{"REST"}}, want: true},
		{name: "message needs JSON", filter: LogFilter{Message: "rest.*"}, want: true},
		{name: "server ID needs JSON", filter: LogFilter{ServerID: "da35*", Archive: true}, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.filter.NeedsStructuredLog(); got != tt.want {
				t.Errorf("NeedsStructuredLog() = %v, want %v", got, tt.want)
			}
		})
	}
}
