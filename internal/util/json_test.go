package util

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/defs"
)

// testRowCount is the number of rows in the sample rowset. It is large enough that the
// serialized payload comfortably exceeds the compression threshold used in these tests.
const testRowCount = 200

// rowSet builds a response shaped like the ones the table endpoints return: a set of
// rows, each a map of column name to value. This shape is why compression pays off so
// well here -- every column name is repeated on every single row.
func rowSet() defs.DBRowSet {
	result := make([]map[string]any, 0, testRowCount)

	for i := range testRowCount {
		result = append(result, map[string]any{
			"id":          i,
			"name":        "customer name value",
			"address":     "1 Example Street, Springfield",
			"description": "a reasonably long description column",
		})
	}

	return defs.DBRowSet{
		Columns: []string{"id", "name", "address", "description"},
		Rows:    result,
		Count:   len(result),
		Status:  http.StatusOK,
	}
}

func TestWriteJSONCompressesLargeResponse(t *testing.T) {
	withThreshold(t, "4096")

	recorder := httptest.NewRecorder()
	length := 0

	indented := WriteJSON(recorder, ResponseInfo{AcceptsGzip: true, Length: &length}, http.StatusOK, rowSet())

	if got := recorder.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("Content-Encoding = %q, want \"gzip\"", got)
	}

	if recorder.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusOK)
	}

	// The returned value is the indented form used for logging, and must remain
	// readable JSON regardless of what was sent on the wire.
	if !strings.HasPrefix(string(indented), "{\n") {
		t.Error("returned logging JSON is not in indented form")
	}

	// What was actually sent must expand back to the same data the caller supplied.
	reader, err := gzip.NewReader(bytes.NewReader(recorder.Body.Bytes()))
	if err != nil {
		t.Fatalf("response body is not a valid gzip stream: %v", err)
	}

	defer reader.Close()

	decoded, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("failed to decompress the response: %v", err)
	}

	var round defs.DBRowSet

	if err := json.Unmarshal(decoded, &round); err != nil {
		t.Fatalf("decompressed payload is not valid JSON: %v", err)
	}

	if round.Count != testRowCount || len(round.Rows) != testRowCount {
		t.Errorf("round-tripped rowset has count %d and %d rows, want %d of each",
			round.Count, len(round.Rows), testRowCount)
	}

	// The reported length must be the compressed size, since that is what was sent.
	if length != recorder.Body.Len() {
		t.Errorf("length = %d, want the wire size of %d", length, recorder.Body.Len())
	}

	// A rowset repeating four column names across every row should compress heavily.
	// Anything less than a 2:1 saving would mean compression is not really working.
	if recorder.Body.Len()*2 > len(decoded) {
		t.Errorf("rowset compressed only from %d to %d bytes, expected a much larger saving",
			len(decoded), recorder.Body.Len())
	}
}

func TestWriteJSONPlainWhenClientCannotDecode(t *testing.T) {
	withThreshold(t, "4096")

	recorder := httptest.NewRecorder()
	length := 0

	WriteJSON(recorder, ResponseInfo{AcceptsGzip: false, Length: &length}, http.StatusOK, rowSet())

	if got := recorder.Header().Get("Content-Encoding"); got != "" {
		t.Errorf("Content-Encoding = %q, want empty", got)
	}

	var round defs.DBRowSet

	if err := json.Unmarshal(recorder.Body.Bytes(), &round); err != nil {
		t.Fatalf("response body is not valid JSON: %v", err)
	}

	if round.Count != testRowCount {
		t.Errorf("count = %d, want %d", round.Count, testRowCount)
	}
}

func TestWriteJSONSmallResponseIsNotCompressed(t *testing.T) {
	withThreshold(t, "4096")

	recorder := httptest.NewRecorder()
	length := 0

	// A row count is the small end of the scale: a handful of fields, far under any
	// sensible threshold, so it must go out untouched even though gzip was allowed.
	WriteJSON(recorder, ResponseInfo{AcceptsGzip: true, Length: &length}, http.StatusOK, defs.DBRowCount{Count: 1, Status: http.StatusOK})

	if got := recorder.Header().Get("Content-Encoding"); got != "" {
		t.Errorf("Content-Encoding = %q, want empty for a small payload", got)
	}

	if !strings.HasPrefix(recorder.Body.String(), "{") {
		t.Errorf("response body is not plain JSON: %s", recorder.Body.String())
	}
}

// TestWriteJSONMinifiesWhatItSends confirms the long-standing behavior that the bytes
// on the wire are minified even though the returned logging copy is indented.
func TestWriteJSONMinifiesWhatItSends(t *testing.T) {
	withThreshold(t, "4096")

	recorder := httptest.NewRecorder()
	length := 0

	indented := WriteJSON(recorder, ResponseInfo{AcceptsGzip: true, Length: &length}, http.StatusOK, defs.DBRowCount{Count: 1, Status: http.StatusOK})

	if strings.Contains(recorder.Body.String(), "\n") {
		t.Error("sent payload contains newlines, so it was not minified")
	}

	if !strings.Contains(string(indented), "\n") {
		t.Error("returned logging payload has no newlines, so it was not indented")
	}
}

// TestWriteJSONPreservesContentType checks that WriteJSON leaves a Content-Type the
// caller set alone. Handlers set specific Ego media types (rowset, row count, DSN list)
// before calling here, and those must survive.
func TestWriteJSONPreservesContentType(t *testing.T) {
	withThreshold(t, "4096")

	recorder := httptest.NewRecorder()
	length := 0

	recorder.Header().Set(defs.ContentTypeHeader, defs.RowSetMediaType)

	WriteJSON(recorder, ResponseInfo{AcceptsGzip: true, Length: &length}, http.StatusOK, rowSet())

	if got := recorder.Header().Get(defs.ContentTypeHeader); got != defs.RowSetMediaType {
		t.Errorf("Content-Type = %q, want %q", got, defs.RowSetMediaType)
	}
}

// TestWriteJSONHonorsStatus covers the reason WriteJSON took over responsibility for
// the status code: callers can no longer send it themselves, so it must be reported
// faithfully from here.
func TestWriteJSONHonorsStatus(t *testing.T) {
	withThreshold(t, "4096")

	for _, status := range []int{http.StatusOK, http.StatusCreated, http.StatusAccepted} {
		recorder := httptest.NewRecorder()
		length := 0

		WriteJSON(recorder, ResponseInfo{AcceptsGzip: true, Length: &length}, status, defs.DBRowCount{Count: 1})

		if recorder.Code != status {
			t.Errorf("status = %d, want %d", recorder.Code, status)
		}
	}
}

// captureLogOutput runs a function with the REST logger enabled and os.Stdout
// redirected, returning everything the logger wrote.
//
// ui.WriteLogString sends log text to the open log file, or to stdout when there is no
// log file. A test process has no log file open, so swapping os.Stdout for a pipe
// captures the output. The pipe is drained on a separate goroutine because a pipe has a
// limited buffer: if nothing were reading while the test wrote, a large enough log
// message would block forever.
func captureLogOutput(t *testing.T, f func()) string {
	t.Helper()

	if ui.CurrentLogFile() != "" {
		t.Skip("a log file is open, so log output does not go to stdout")
	}

	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("cannot create a pipe: %v", err)
	}

	previousStdout := os.Stdout
	previousActive := ui.IsActive(ui.RestLogger)

	os.Stdout = writer

	ui.Active(ui.RestLogger, true)

	captured := make(chan string, 1)

	go func() {
		var buffer bytes.Buffer

		_, _ = io.Copy(&buffer, reader)

		captured <- buffer.String()
	}()

	f()

	// Restore first, so a failure inside f() cannot leave the process without stdout.
	os.Stdout = previousStdout

	ui.Active(ui.RestLogger, previousActive)

	_ = writer.Close()

	return <-captured
}

// TestWriteJSONLogsCompressionSaving is the check that the compression report reaches
// the REST log for an ordinary WriteJSON caller -- that is, for every table, DSN, and
// admin response in the server, not only the log endpoint that first had it.
func TestWriteJSONLogsCompressionSaving(t *testing.T) {
	withThreshold(t, "4096")

	length := 0

	output := captureLogOutput(t, func() {
		WriteJSON(httptest.NewRecorder(),
			ResponseInfo{SessionID: 42, AcceptsGzip: true, Length: &length},
			http.StatusOK, rowSet())
	})

	if !strings.Contains(output, "compressed") {
		t.Fatalf("no compression entry was logged; got: %s", output)
	}

	// The session ID must be present so the entry can be tied to a request. The logger
	// renders it as a bracketed prefix.
	if !strings.Contains(output, "[42]") {
		t.Errorf("compression log entry is missing the session prefix [42]: %s", output)
	}

	// A rowset should report a large saving; a percentage near zero would mean the
	// numbers being reported are wrong even though compression happened.
	if strings.Contains(output, "(0% smaller)") {
		t.Errorf("compression log reports a zero saving: %s", output)
	}
}

// TestWriteJSONLogsNothingWhenNotCompressed confirms the report is tied to compression
// actually happening, rather than being emitted for every response.
func TestWriteJSONLogsNothingWhenNotCompressed(t *testing.T) {
	withThreshold(t, "4096")

	length := 0

	output := captureLogOutput(t, func() {
		// Below the threshold, so nothing is compressed and nothing should be reported.
		WriteJSON(httptest.NewRecorder(),
			ResponseInfo{SessionID: 42, AcceptsGzip: true, Length: &length},
			http.StatusOK, defs.DBRowCount{Count: 1})
	})

	if strings.Contains(output, "compressed") {
		t.Errorf("a compression entry was logged for an uncompressed response: %s", output)
	}
}
