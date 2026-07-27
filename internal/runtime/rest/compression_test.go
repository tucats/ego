package rest

import (
	"bytes"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tucats/ego/internal/cli/settings"
	"github.com/tucats/ego/internal/defs"
)

// gzipOf compresses a string into a complete gzip stream, for use in building fake
// server responses that a real client would have to decode.
func gzipOf(t *testing.T, text string) []byte {
	t.Helper()

	var buffer bytes.Buffer

	writer := gzip.NewWriter(&buffer)

	if _, err := writer.Write([]byte(text)); err != nil {
		t.Fatalf("failed to compress test data: %v", err)
	}

	// Close() writes the gzip trailer. Without it the stream is truncated and no
	// decompressor will accept it.
	if err := writer.Close(); err != nil {
		t.Fatalf("failed to finish compressing test data: %v", err)
	}

	return buffer.Bytes()
}

// withSetting overrides one configuration value for the duration of a test and puts
// the previous value back afterwards, since configuration is process-wide state shared
// by every test in the package.
func withSetting(t *testing.T, key, value string) {
	t.Helper()

	previous := settings.Get(key)

	settings.SetDefault(key, value)

	t.Cleanup(func() {
		settings.SetDefault(key, previous)
	})
}

// TestExchangeDecodesCompressedResponse is the end-to-end check that a gzipped reply
// from a server arrives at the caller as ordinary decoded JSON. It runs against a real
// HTTP server (httptest) so that the whole stack -- Go's transport, the resty client,
// and this package's own decoding -- is exercised exactly as it is in production.
func TestExchangeDecodesCompressedResponse(t *testing.T) {
	expected := "log line one, repeated many times over to make a large payload"

	body := `{"msg":"` + strings.Repeat(expected+" ", 500) + `","status":200}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The client must have told us it can accept gzip; that advertisement is the
		// whole precondition for compressing anything.
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			t.Errorf("request did not advertise gzip support: Accept-Encoding = %q",
				r.Header.Get("Accept-Encoding"))
		}

		w.Header().Set(defs.ContentTypeHeader, defs.JSONMediaType)
		w.Header().Set("Content-Encoding", "gzip")
		w.WriteHeader(http.StatusOK)

		_, _ = w.Write(gzipOf(t, body))
	}))

	defer server.Close()

	withSetting(t, defs.RestClientCompressionSetting, "true")

	response := defs.RestStatusResponse{}

	if err := Exchange(server.URL+"/test", http.MethodGet, nil, &response, defs.AdminAgent); err != nil {
		t.Fatalf("Exchange() returned an error: %v", err)
	}

	// The caller must see the fully expanded text, with no trace of the compression
	// that happened in between.
	if !strings.Contains(response.Message, expected) {
		t.Errorf("decoded message does not contain the expected text; got %d bytes",
			len(response.Message))
	}

	if response.Status != http.StatusOK {
		t.Errorf("status = %d, want %d", response.Status, http.StatusOK)
	}
}

// TestExchangeUncompressedResponseStillWorks confirms the ordinary, uncompressed path
// is unaffected by the decoding logic added for compressed responses.
func TestExchangeUncompressedResponseStillWorks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(defs.ContentTypeHeader, defs.JSONMediaType)
		w.WriteHeader(http.StatusOK)

		_, _ = w.Write([]byte(`{"msg":"plain and uncompressed","status":200}`))
	}))

	defer server.Close()

	response := defs.RestStatusResponse{}

	if err := Exchange(server.URL+"/test", http.MethodGet, nil, &response, defs.AdminAgent); err != nil {
		t.Fatalf("Exchange() returned an error: %v", err)
	}

	if response.Message != "plain and uncompressed" {
		t.Errorf("message = %q, want \"plain and uncompressed\"", response.Message)
	}
}

// TestExchangeRequestsIdentityWhenDisabled verifies that turning compression off in
// the configuration genuinely reaches the server as a request for uncompressed data.
//
// This case is easy to get wrong: simply omitting the Accept-Encoding header does NOT
// disable compression, because Go's HTTP transport helpfully adds "gzip" back on any
// request that does not set the header. Only an explicit "identity" suppresses it.
func TestExchangeRequestsIdentityWhenDisabled(t *testing.T) {
	var seenEncoding string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenEncoding = r.Header.Get("Accept-Encoding")

		w.Header().Set(defs.ContentTypeHeader, defs.JSONMediaType)
		w.WriteHeader(http.StatusOK)

		_, _ = w.Write([]byte(`{"msg":"ok","status":200}`))
	}))

	defer server.Close()

	withSetting(t, defs.RestClientCompressionSetting, "false")

	response := defs.RestStatusResponse{}

	if err := Exchange(server.URL+"/test", http.MethodGet, nil, &response, defs.AdminAgent); err != nil {
		t.Fatalf("Exchange() returned an error: %v", err)
	}

	if seenEncoding != "identity" {
		t.Errorf("Accept-Encoding = %q, want \"identity\"", seenEncoding)
	}
}

// TestResponseBodyDecodesRawGzip exercises the defensive decoding path directly: a
// response body that is still gzip-compressed when this package receives it.
//
// The HTTP client library expands compressed bodies on its own today, so this
// situation does not arise in practice. The safety net exists in case that library is
// ever replaced by one that does not, which would otherwise feed raw gzip bytes into a
// JSON parser. Building the response by hand is the only way to reproduce it.
func TestResponseBodyDecodesRawGzip(t *testing.T) {
	original := "this text was compressed and must come back out intact"

	// A server that claims no content encoding but sends gzip bytes anyway simulates a
	// client library that did not decode the payload for us.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)

		_, _ = w.Write(gzipOf(t, original))
	}))

	defer server.Close()

	client, err := newClient(server.URL, nil)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	restResponse, err := client.NewRequest().Execute(http.MethodGet, server.URL)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}

	if got := string(responseBody(restResponse)); got != original {
		t.Errorf("responseBody() = %q, want %q", got, original)
	}
}

// TestResponseBodyLeavesPlainTextAlone confirms the magic-byte check does not disturb
// ordinary payloads, including short ones where there are fewer than two bytes to test.
func TestResponseBodyLeavesPlainTextAlone(t *testing.T) {
	for _, payload := range []string{"", "x", "{}", `{"msg":"hello"}`, "plain text response"} {
		body := payload

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)

			_, _ = w.Write([]byte(body))
		}))

		client, err := newClient(server.URL, nil)
		if err != nil {
			server.Close()
			t.Fatalf("failed to create client: %v", err)
		}

		restResponse, err := client.NewRequest().Execute(http.MethodGet, server.URL)
		if err != nil {
			server.Close()
			t.Fatalf("request failed: %v", err)
		}

		if got := string(responseBody(restResponse)); got != body {
			t.Errorf("responseBody() = %q, want %q", got, body)
		}

		server.Close()
	}
}
