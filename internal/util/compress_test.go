package util

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
)

// withThreshold sets the compression threshold configuration for the duration of a
// single test and restores whatever was there before when the test finishes. Tests in
// the same package share process-wide configuration state, so a test that changes a
// setting must always put it back.
func withThreshold(t *testing.T, value string) {
	t.Helper()

	previous := settings.Get(defs.ServerCompressionThresholdSetting)

	settings.SetDefault(defs.ServerCompressionThresholdSetting, value)

	t.Cleanup(func() {
		settings.SetDefault(defs.ServerCompressionThresholdSetting, previous)
	})
}

// requestWithEncoding builds a throwaway GET request carrying the given
// Accept-Encoding header value. An empty string means the header is not sent at all,
// which is a meaningfully different case from sending an empty header.
func requestWithEncoding(header string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/services/admin/log/", nil)

	if header != "" {
		r.Header.Set("Accept-Encoding", header)
	}

	return r
}

func TestAcceptsGzip(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   bool
	}{
		{name: "no header at all", header: "", want: false},
		{name: "plain gzip", header: "gzip", want: true},
		{name: "gzip among others", header: "gzip, deflate, br", want: true},
		{name: "gzip with quality", header: "gzip;q=0.5", want: true},
		{name: "gzip with spaces", header: "  gzip ; q=0.8 ", want: true},
		{name: "mixed case", header: "GZip", want: true},
		{name: "explicitly refused", header: "gzip;q=0", want: false},
		{name: "refused with decimal", header: "gzip;q=0.0", want: false},
		{name: "identity only", header: "identity", want: false},
		{name: "other encodings only", header: "deflate, br", want: false},
		{name: "wildcard", header: "*", want: true},
		{name: "wildcard refused", header: "*;q=0", want: false},
		// An explicit statement about gzip is more specific than the wildcard, so it
		// wins in both directions.
		{name: "wildcard but gzip refused", header: "*, gzip;q=0", want: false},
		{name: "wildcard refused but gzip allowed", header: "*;q=0, gzip", want: true},
		// A quality value we cannot parse is treated permissively, matching how
		// mainstream HTTP servers behave.
		{name: "unparseable quality", header: "gzip;q=bogus", want: true},
		{name: "empty elements are skipped", header: ", ,gzip", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := AcceptsGzip(requestWithEncoding(tt.header)); got != tt.want {
				t.Errorf("AcceptsGzip(%q) = %v, want %v", tt.header, got, tt.want)
			}
		})
	}
}

func TestAcceptsGzipNilRequest(t *testing.T) {
	if AcceptsGzip(nil) {
		t.Error("AcceptsGzip(nil) = true, want false")
	}
}

func TestCompressionThreshold(t *testing.T) {
	tests := []struct {
		name    string
		setting string
		want    int
	}{
		{name: "unset uses default", setting: "", want: DefaultCompressionThreshold},
		{name: "explicit zero disables", setting: "0", want: 0},
		{name: "explicit value", setting: "1024", want: 1024},
		{name: "surrounding whitespace", setting: "  2048  ", want: 2048},
		{name: "malformed falls back", setting: "large", want: DefaultCompressionThreshold},
		{name: "negative falls back", setting: "-1", want: DefaultCompressionThreshold},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withThreshold(t, tt.setting)

			if got := CompressionThreshold(); got != tt.want {
				t.Errorf("CompressionThreshold() = %d, want %d with setting %q", got, tt.want, tt.setting)
			}
		})
	}
}

// repeatedText builds a payload of at least the requested size out of realistic,
// repetitive log-like text. Repetitive input is what gzip compresses well, which is
// exactly the case this feature exists to exploit.
func repeatedText(minimumSize int) []byte {
	line := "{\"time\":\"2026-07-27T10:15:00Z\",\"class\":\"SERVER\",\"msg\":\"request completed\"}\n"

	var buffer bytes.Buffer

	for buffer.Len() < minimumSize {
		buffer.WriteString(line)
	}

	return buffer.Bytes()
}

func TestWriteMaybeCompressedBelowThreshold(t *testing.T) {
	withThreshold(t, "4096")

	body := repeatedText(100)
	recorder := httptest.NewRecorder()
	length := 0

	compressed, err := WriteMaybeCompressed(recorder,
		ResponseInfo{AcceptsGzip: true, Length: &length},
		http.StatusOK, "application/json", body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if compressed {
		t.Error("payload below the threshold was compressed")
	}

	if got := recorder.Header().Get("Content-Encoding"); got != "" {
		t.Errorf("Content-Encoding = %q, want empty", got)
	}

	if !bytes.Equal(recorder.Body.Bytes(), body) {
		t.Error("response body does not match the original payload")
	}

	if length != len(body) {
		t.Errorf("length = %d, want %d", length, len(body))
	}
}

func TestWriteMaybeCompressedAboveThreshold(t *testing.T) {
	withThreshold(t, "4096")

	body := repeatedText(64 * 1024)
	recorder := httptest.NewRecorder()
	length := 0

	compressed, err := WriteMaybeCompressed(recorder,
		ResponseInfo{AcceptsGzip: true, Length: &length},
		http.StatusOK, "application/json", body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !compressed {
		t.Fatal("payload above the threshold was not compressed")
	}

	if got := recorder.Header().Get("Content-Encoding"); got != "gzip" {
		t.Errorf("Content-Encoding = %q, want \"gzip\"", got)
	}

	// Without this header a shared cache could hand the compressed bytes to a client
	// that never asked for them and cannot decode them.
	if got := recorder.Header().Get("Vary"); got != "Accept-Encoding" {
		t.Errorf("Vary = %q, want \"Accept-Encoding\"", got)
	}

	if got := recorder.Header().Get(defs.ContentTypeHeader); got != "application/json" {
		t.Errorf("Content-Type = %q, want \"application/json\"", got)
	}

	// The bytes on the wire must be a valid gzip stream that expands back to exactly
	// the original payload. This is the property that actually matters to a client.
	reader, err := gzip.NewReader(bytes.NewReader(recorder.Body.Bytes()))
	if err != nil {
		t.Fatalf("response body is not a valid gzip stream: %v", err)
	}

	defer reader.Close()

	decoded, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("failed to decompress response body: %v", err)
	}

	if !bytes.Equal(decoded, body) {
		t.Error("decompressed body does not match the original payload")
	}

	// The reported length must be the compressed size, since that is what was
	// actually sent over the network.
	if length != recorder.Body.Len() {
		t.Errorf("length = %d, want the wire size of %d", length, recorder.Body.Len())
	}

	if length >= len(body) {
		t.Errorf("compressed size %d is not smaller than the original %d", length, len(body))
	}
}

func TestWriteMaybeCompressedClientRefuses(t *testing.T) {
	withThreshold(t, "4096")

	body := repeatedText(64 * 1024)

	// Both a client that asks for identity and a client that says nothing at all must
	// receive the payload untouched.
	for _, header := range []string{"identity", "gzip;q=0", ""} {
		recorder := httptest.NewRecorder()
		length := 0

		compressed, err := WriteMaybeCompressed(recorder,
			ResponseInfo{AcceptsGzip: AcceptsGzip(requestWithEncoding(header)), Length: &length},
			http.StatusOK, "text/plain", body)
		if err != nil {
			t.Fatalf("unexpected error for Accept-Encoding %q: %v", header, err)
		}

		if compressed {
			t.Errorf("payload was compressed for a client sending Accept-Encoding %q", header)
		}

		if !bytes.Equal(recorder.Body.Bytes(), body) {
			t.Errorf("body was altered for a client sending Accept-Encoding %q", header)
		}
	}
}

func TestWriteMaybeCompressedDisabledByConfiguration(t *testing.T) {
	// A threshold of zero means "never compress", regardless of size or client support.
	withThreshold(t, "0")

	body := repeatedText(64 * 1024)
	recorder := httptest.NewRecorder()
	length := 0

	compressed, err := WriteMaybeCompressed(recorder,
		ResponseInfo{AcceptsGzip: true, Length: &length},
		http.StatusOK, "text/plain", body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if compressed {
		t.Error("payload was compressed even though compression is disabled")
	}

	if !bytes.Equal(recorder.Body.Bytes(), body) {
		t.Error("response body does not match the original payload")
	}
}

func TestWriteMaybeCompressedIncompressiblePayload(t *testing.T) {
	withThreshold(t, "16")

	// Data that gzip cannot shrink must be sent as-is rather than sent larger. A short
	// run of non-repeating bytes reliably produces a gzip stream bigger than its input,
	// because the gzip envelope alone is 18 bytes.
	body := []byte(strings.Repeat("abcdefghijklmnopqrstuvwxyz", 1)[:20])

	recorder := httptest.NewRecorder()
	length := 0

	compressed, err := WriteMaybeCompressed(recorder,
		ResponseInfo{AcceptsGzip: true, Length: &length},
		http.StatusOK, "text/plain", body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if compressed {
		t.Error("an incompressible payload was sent compressed, making it larger")
	}

	if !bytes.Equal(recorder.Body.Bytes(), body) {
		t.Error("response body does not match the original payload")
	}
}

func TestWriteMaybeCompressedStatusIsPreserved(t *testing.T) {
	withThreshold(t, "4096")

	recorder := httptest.NewRecorder()
	length := 0

	_, err := WriteMaybeCompressed(recorder,
		ResponseInfo{AcceptsGzip: true, Length: &length},
		http.StatusCreated, "text/plain", []byte("short"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if recorder.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusCreated)
	}
}
