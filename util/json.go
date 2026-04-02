package util

import (
	"encoding/json"
	"net/http"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/egostrings"
)

// WriteJSON writes a JSON response with the given body to the HTTP response writer. The object
// is JSON encoded and transmitted in the most compressed (minified) form possible. A human-readable
// indented version of the JSON is returned to the caller for use in logging, etc.
func WriteJSON(w http.ResponseWriter, body any, length *int) []byte {
	// Create the attractive indented human-readable JSON
	b, _ := json.MarshalIndent(body, ui.JSONIndentPrefix, ui.JSONIndentSpacer)

	// Also minify it to compress the JSON as much as syntactically possible.
	minifiedBytes := []byte(egostrings.JSONMinify(string(b)))

	// Write the minified JSON to the response writer, and update the response length
	_, _ = w.Write(minifiedBytes)
	*length += len(minifiedBytes)

	// Return the fluffy human-readable JSON as the result. This is most often
	// used to log the response body for debugging purposes.
	return b
}
