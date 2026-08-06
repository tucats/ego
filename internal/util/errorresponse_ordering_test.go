package util

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tucats/ego/internal/defs"
)

// TestErrorResponseClampsStatusInBody guards against ErrorResponse sending a
// clamped status in the HTTP header while the JSON body still reports the
// original, out-of-range value. The two used to disagree because the status
// was copied into the response struct before the clamp ran.
func TestErrorResponseClampsStatusInBody(t *testing.T) {
	tests := []struct {
		name   string
		status int
	}{
		{name: "status too low", status: 42},
		{name: "status too high", status: 999},
		{name: "status zero", status: 0},
		{name: "status negative", status: -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()

			got := ErrorResponse(recorder, 1, "test message", tt.status)
			if got != http.StatusInternalServerError {
				t.Fatalf("ErrorResponse() returned %d, want %d", got, http.StatusInternalServerError)
			}

			if recorder.Code != http.StatusInternalServerError {
				t.Errorf("HTTP header status = %d, want %d", recorder.Code, http.StatusInternalServerError)
			}

			var body defs.RestStatusResponse
			if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
				t.Fatalf("response body did not parse as JSON: %v (body: %s)", err, recorder.Body.String())
			}

			if body.Status != http.StatusInternalServerError {
				t.Errorf("body.Status = %d, want %d (header and body must agree)", body.Status, http.StatusInternalServerError)
			}
		})
	}
}

// TestErrorResponseStripsPostgresNoiseFromBody guards against the "pq: "
// prefix trim running after the response struct (and therefore the JSON
// body) was already built, which made the trim a no-op as far as the client
// could tell.
func TestErrorResponseStripsPostgresNoiseFromBody(t *testing.T) {
	recorder := httptest.NewRecorder()

	ErrorResponse(recorder, 1, "pq: duplicate key value violates unique constraint", http.StatusConflict)

	var body defs.RestStatusResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("response body did not parse as JSON: %v (body: %s)", err, recorder.Body.String())
	}

	if body.Message != "duplicate key value violates unique constraint" {
		t.Errorf("body.Message = %q, want the \"pq: \" prefix stripped", body.Message)
	}
}
