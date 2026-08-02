package router

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tucats/ego/internal/cli/ui"
)

// These tests cover the query parameters that filter the log endpoint's
// results: session, class, and msg. The filtering logic itself is tested
// against the log file in the ui package; what matters here is the plumbing --
// that the parameters reach the filter at all, and that a filter the caller got
// wrong comes back as a 400 rather than a 500.
//
// That status mapping is the part most at risk of quietly breaking. The error
// travels from ui.TailFiltered, through the Ego builtin util.Log, through
// builtins.CallBuiltin, and back into LogHandler, being wrapped with call
// context along the way. LogHandler classifies it by comparing the underlying
// error rather than by matching strings, and these tests are what prove that
// still works after all that wrapping.
//
// As in the compression tests: a test process has no server log file open, so
// ui.Tail returns a short placeholder rather than real log content. That is
// fine here, because every case either fails during filter validation (before
// the file is ever read) or only checks the status code.

// withLogFormat sets the process-wide log format for one test and restores it
// afterwards. The default is text format, and the class and message filters are
// only accepted against a JSON-format log.
func withLogFormat(t *testing.T, format string) {
	t.Helper()

	previous := ui.LogFormat
	ui.LogFormat = format

	t.Cleanup(func() { ui.LogFormat = previous })
}

// callLogHandlerWithParameters drives LogHandler with an arbitrary set of query
// parameters and returns the status it produced along with the recorded
// response.
func callLogHandlerWithParameters(t *testing.T, parameters map[string][]string) (int, *httptest.ResponseRecorder) {
	t.Helper()

	query := []string{}
	for name, values := range parameters {
		for _, value := range values {
			query = append(query, name+"="+value)
		}
	}

	r := httptest.NewRequest(http.MethodGet, "/services/admin/log/?"+strings.Join(query, "&"), nil)

	session := &Session{
		ID:          1,
		Parameters:  parameters,
		AcceptsJSON: true,
		Language:    "en",
	}

	recorder := httptest.NewRecorder()
	status := LogHandler(session, recorder, r)

	return status, recorder
}

// A filter the caller got wrong is the caller's mistake. It must come back as a
// 400 so the client can correct it, not a 500 suggesting the server broke.
func TestLogHandlerRejectsBadFilters(t *testing.T) {
	tests := []struct {
		name       string
		format     string
		parameters map[string][]string
		wantStatus int
	}{
		{
			name:       "unknown logger class",
			format:     ui.JSONFormat,
			parameters: map[string][]string{"class": {"NOSUCHCLASS"}},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "one bad class among good ones",
			format:     ui.JSONFormat,
			parameters: map[string][]string{"class": {"REST,NOSUCHCLASS"}},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "malformed message pattern",
			format:     ui.JSONFormat,
			parameters: map[string][]string{"msg": {"rest.[a-"}},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "class filter against a text-format log",
			format:     ui.TextFormat,
			parameters: map[string][]string{"class": {"REST"}},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "message filter against a text-format log",
			format:     ui.TextFormat,
			parameters: map[string][]string{"msg": {"rest.*"}},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid tail value",
			format:     ui.JSONFormat,
			parameters: map[string][]string{"tail": {"not-a-number"}},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid session value",
			format:     ui.JSONFormat,
			parameters: map[string][]string{"session": {"not-a-number"}},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withLogFormat(t, tt.format)

			status, recorder := callLogHandlerWithParameters(t, tt.parameters)

			if status != tt.wantStatus {
				t.Errorf("LogHandler returned status %d, want %d (body: %s)",
					status, tt.wantStatus, recorder.Body.String())
			}

			if recorder.Code != tt.wantStatus {
				t.Errorf("response code is %d, want %d", recorder.Code, tt.wantStatus)
			}
		})
	}
}

// Filters that are well formed must be accepted, whether or not they end up
// matching anything in this test process's (absent) log file.
func TestLogHandlerAcceptsValidFilters(t *testing.T) {
	tests := []struct {
		name       string
		format     string
		parameters map[string][]string
	}{
		{
			name:       "no filter at all",
			format:     ui.JSONFormat,
			parameters: map[string][]string{"tail": {"10"}},
		},
		{
			name:       "single class",
			format:     ui.JSONFormat,
			parameters: map[string][]string{"class": {"REST"}},
		},
		{
			name:       "comma-separated classes in one parameter",
			format:     ui.JSONFormat,
			parameters: map[string][]string{"class": {"REST,AUTH,SQL"}},
		},
		{
			// A client may equally send ?class=REST&class=AUTH; the handler
			// joins repeated values into the same comma-separated list.
			name:       "class repeated as separate parameters",
			format:     ui.JSONFormat,
			parameters: map[string][]string{"class": {"REST", "AUTH"}},
		},
		{
			name:       "class name in lower case",
			format:     ui.JSONFormat,
			parameters: map[string][]string{"class": {"rest"}},
		},
		{
			name:       "message glob",
			format:     ui.JSONFormat,
			parameters: map[string][]string{"msg": {"rest.*"}},
		},
		{
			name:       "every filter together",
			format:     ui.JSONFormat,
			parameters: map[string][]string{"session": {"3"}, "class": {"REST"}, "msg": {"rest.*"}, "tail": {"25"}},
		},
		{
			// Session filtering has a text-log fallback, so it is allowed where
			// class and message are not.
			name:       "session filter against a text-format log",
			format:     ui.TextFormat,
			parameters: map[string][]string{"session": {"3"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withLogFormat(t, tt.format)

			status, recorder := callLogHandlerWithParameters(t, tt.parameters)

			if status != http.StatusOK {
				t.Fatalf("LogHandler returned status %d, want %d (body: %s)",
					status, http.StatusOK, recorder.Body.String())
			}

			// The body should still be a well-formed log response.
			var payload struct {
				Lines []string `json:"lines"`
			}

			if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
				t.Errorf("response body is not valid JSON: %v", err)
			}
		})
	}
}

// A rejected filter should say what was wrong with it. An empty or generic body
// would leave the dashboard with nothing useful to show the user.
func TestLogHandlerFilterErrorNamesTheProblem(t *testing.T) {
	withLogFormat(t, ui.JSONFormat)

	_, recorder := callLogHandlerWithParameters(t, map[string][]string{"class": {"NOSUCHCLASS"}})

	body := recorder.Body.String()
	if !strings.Contains(body, "NOSUCHCLASS") {
		t.Errorf("error response does not mention the offending class name: %s", body)
	}
}
