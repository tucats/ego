package util

import (
	"encoding/json"
	"net/http"

	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/util/strings"
)

// WriteJSON writes a complete JSON response: it encodes the body, sends the status
// line and headers, and writes the payload, compressing it with gzip when that is both
// allowed and worthwhile. A human-readable indented version of the JSON is returned to
// the caller for use in logging.
//
// The parameters are:
//
//	w       the response writer to send the reply on
//	info    the per-request facts from the Session: whether the client can decode a
//	        compressed body, the session ID for logging, and the counter that
//	        accumulates the response size. Handlers obtain one with session.Response()
//	status  the HTTP status code to report (usually http.StatusOK)
//	body    any value that can be marshalled to JSON
//
// Every compressed response is reported to the REST logger with its before and after
// sizes; that happens inside WriteMaybeCompressed, so it covers all callers.
//
// IMPORTANT for callers: this function now owns the response status, so a caller must
// NOT call w.WriteHeader() itself beforehand. WriteHeader is the moment the status line
// and all headers are flushed to the client, and any header set afterwards is silently
// discarded. If a caller sent the headers early, the "Content-Encoding: gzip" header
// set here would be dropped while the compressed bytes were still sent -- producing a
// response that no client can decode, with no error anywhere to explain it. Setting
// headers such as Content-Type before calling here is still correct and expected; it is
// only WriteHeader that must be left to this function.
//
// The bytes counted into info.Length are the bytes that actually crossed the network,
// so for a compressed response that is the compressed size. That keeps the server's
// request log honest about bandwidth rather than reporting a size that was never sent.
func WriteJSON(w http.ResponseWriter, info ResponseInfo, status int, body any) []byte {
	// Create the attractive indented human-readable JSON
	b, _ := json.MarshalIndent(body, ui.JSONIndentPrefix, ui.JSONIndentSpacer)

	// Also minify it to compress the JSON as much as syntactically possible. This is
	// a purely syntactic saving (dropping whitespace); gzip below does the real work.
	minifiedBytes := []byte(egostrings.JSONMinify(string(b)))

	// The empty content type leaves any Content-Type header the caller already set in
	// place; most callers set a specific Ego media type before calling here.
	_, _ = WriteMaybeCompressed(w, info, status, "", minifiedBytes)

	// Return the fluffy human-readable JSON as the result. This is most often
	// used to log the response body for debugging purposes.
	return b
}
