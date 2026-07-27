package router

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tucats/ego/internal/cli/settings"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/util"
)

// These tests check that LogHandler is correctly wired to the shared compression
// helper: that both the JSON and plain-text forms of a log response are compressed for
// a client that accepts gzip, that the right headers accompany them, and that the
// configuration switch is honored. The compression logic itself -- thresholds, header
// parsing, and byte accounting -- is tested directly in the util package.
//
// One limitation to be aware of when reading these tests: LogHandler ultimately calls
// ui.Tail(), which reads the server's log FILE. A test process has no log file open,
// so ui.Tail() returns a short placeholder response of a few hundred bytes rather than
// the hundreds of kilobytes a busy server would produce. That is why these tests set a
// deliberately tiny threshold -- it is the only way to exercise the compression path
// against the small payload available here. It does not change what is being verified.

// withThreshold overrides the server compression threshold for one test and restores
// the previous value afterwards, since configuration is shared process-wide state.
func withThreshold(t *testing.T, value string) {
	t.Helper()

	previous := settings.Get(defs.ServerCompressionThresholdSetting)

	settings.SetDefault(defs.ServerCompressionThresholdSetting, value)

	t.Cleanup(func() {
		settings.SetDefault(defs.ServerCompressionThresholdSetting, previous)
	})
}

// callLogHandler drives the real LogHandler with a request carrying the given
// Accept-Encoding header, and returns the recorded response. An empty acceptEncoding
// means the header is omitted entirely, which is how a client that knows nothing about
// compression behaves.
func callLogHandler(t *testing.T, acceptEncoding string, wantJSON bool) *httptest.ResponseRecorder {
	t.Helper()

	r := httptest.NewRequest(http.MethodGet, "/services/admin/log/?tail=200", nil)
	if acceptEncoding != "" {
		r.Header.Set("Accept-Encoding", acceptEncoding)
	}

	// AcceptsJSON, AcceptsText, and AcceptsGzip are the flags the router normally
	// derives from the request headers before calling a handler. The first two select
	// which of the two response forms LogHandler produces; the third says whether the
	// client can decode a compressed body. AcceptsGzip is derived here exactly as
	// ServeHTTP does it, so the test exercises the same path as a live request.
	session := &Session{
		ID:          1,
		Parameters:  map[string][]string{"tail": {"200"}},
		AcceptsJSON: wantJSON,
		AcceptsText: !wantJSON,
		AcceptsGzip: util.AcceptsGzip(r),
		Language:    "en",
	}

	recorder := httptest.NewRecorder()

	if status := LogHandler(session, recorder, r); status != http.StatusOK {
		t.Fatalf("LogHandler returned status %d, want %d", status, http.StatusOK)
	}

	return recorder
}

// decodeGzip expands a gzip response body, failing the test if it is not a well-formed
// gzip stream. A truncated stream is the classic symptom of forgetting to close the
// gzip writer, so this doubles as a check that the response is complete.
func decodeGzip(t *testing.T, body []byte) string {
	t.Helper()

	reader, err := gzip.NewReader(bytes.NewReader(body))
	if err != nil {
		t.Fatalf("response body is not a valid gzip stream: %v", err)
	}

	defer reader.Close()

	decoded, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("failed to decompress the response: %v", err)
	}

	return string(decoded)
}

func TestLogHandlerCompressesJSONPayload(t *testing.T) {
	withThreshold(t, "64")

	recorder := callLogHandler(t, "gzip", true)

	if got := recorder.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("Content-Encoding = %q, want \"gzip\" (payload was %d bytes)",
			got, recorder.Body.Len())
	}

	if got := recorder.Header().Get("Vary"); got != "Accept-Encoding" {
		t.Errorf("Vary = %q, want \"Accept-Encoding\"", got)
	}

	if got := recorder.Header().Get(defs.ContentTypeHeader); got != defs.LogLinesJSONMediaType {
		t.Errorf("Content-Type = %q, want %q", got, defs.LogLinesJSONMediaType)
	}

	// The expanded payload must be exactly the JSON log response the client expects,
	// proving the compression is transparent to the response format.
	decoded := decodeGzip(t, recorder.Body.Bytes())

	if !strings.HasPrefix(decoded, "{") || !strings.Contains(decoded, "\"lines\"") {
		t.Errorf("decompressed payload is not the expected JSON log response: %.120s", decoded)
	}
}

func TestLogHandlerCompressesTextPayload(t *testing.T) {
	withThreshold(t, "64")

	recorder := callLogHandler(t, "gzip", false)

	if got := recorder.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("Content-Encoding = %q, want \"gzip\" (payload was %d bytes)",
			got, recorder.Body.Len())
	}

	if got := recorder.Header().Get(defs.ContentTypeHeader); got != "text/plain" {
		t.Errorf("Content-Type = %q, want \"text/plain\"", got)
	}

	// The text form is one log line per line of output, so it must not look like JSON.
	decoded := decodeGzip(t, recorder.Body.Bytes())

	if strings.HasPrefix(decoded, "{") {
		t.Errorf("text payload unexpectedly looks like JSON: %.120s", decoded)
	}

	if !strings.Contains(decoded, "\n") {
		t.Error("text payload does not contain any line breaks")
	}
}

// TestLogHandlerDoesNotCompressForPlainClient is the backward-compatibility guarantee:
// a client that never mentioned compression still receives readable JSON.
func TestLogHandlerDoesNotCompressForPlainClient(t *testing.T) {
	withThreshold(t, "64")

	recorder := callLogHandler(t, "", true)

	if got := recorder.Header().Get("Content-Encoding"); got != "" {
		t.Errorf("Content-Encoding = %q, want empty", got)
	}

	if !strings.HasPrefix(recorder.Body.String(), "{") {
		t.Errorf("response body is not plain JSON: %.120s", recorder.Body.String())
	}
}

// TestLogHandlerDisabledCompression verifies the configuration switch reaches the
// handler: with a threshold of zero, nothing is compressed no matter who asks.
func TestLogHandlerDisabledCompression(t *testing.T) {
	withThreshold(t, "0")

	recorder := callLogHandler(t, "gzip", true)

	if got := recorder.Header().Get("Content-Encoding"); got != "" {
		t.Errorf("Content-Encoding = %q, want empty when compression is disabled", got)
	}

	if !strings.HasPrefix(recorder.Body.String(), "{") {
		t.Errorf("response body is not plain JSON: %.120s", recorder.Body.String())
	}
}
