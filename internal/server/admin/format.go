package admin

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/i18n"
	"github.com/tucats/ego/internal/language/parse"
	"github.com/tucats/ego/internal/language/parse/format"
	"github.com/tucats/ego/internal/router"
	"github.com/tucats/ego/internal/util"
)

// Maximum size of the POST /admin/format request body and Code field (256 KiB),
// matching the limit already enforced on POST /admin/run.
const (
	maxFormatBodyBytes = 256 << 10
	maxFormatCodeBytes = 256 << 10
)

// formatRequest is the JSON body expected by POST /admin/format.
//
// Fields:
//
//	Code — the Ego source text to parse and reformat. May be either a
//	       complete program or a bare statement fragment (see parse.ParseAuto);
//	       the caller does not need to say which.
type formatRequest struct {
	Code string `json:"code"`
}

// formatResponse is the JSON body returned by POST /admin/format.
//
//	Formatted — the canonically reformatted source. Empty when Error is set.
//	Error     — non-empty when the source could not be parsed.
type formatResponse struct {
	Formatted string `json:"formatted,omitempty"`
	Error     string `json:"error,omitempty"`
	Elapsed   string `json:"elapsed,omitempty"`
}

// FormatCodeHandler is the HTTP handler for POST /admin/format. It parses the
// submitted Ego source (trying complete-program form first, falling back to
// bare statement-fragment form via parse.ParseAuto -- the same auto-detection
// "ego format" uses) and returns it reprinted in canonical form.
//
// A parse error is reported in the response body's Error field with HTTP 200,
// matching the established convention already used by /admin/run: a program
// that doesn't parse is a normal, expected client outcome (the user's source
// has a mistake), not a server failure.
func FormatCodeHandler(session *router.Session, w http.ResponseWriter, r *http.Request) int {
	var req formatRequest

	start := time.Now()

	// Limit body size before decoding to prevent memory exhaustion, mirroring
	// the same guard on POST /admin/run (CODE-M1).
	r.Body = http.MaxBytesReader(w, r.Body, maxFormatBodyBytes)

	// If the content type is text, we just assume the entire body
	// is the code.
	contentType := r.Header.Get("Content-Type")
	if strings.Contains(strings.ToLower(contentType), "text") {
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			return util.ErrorResponse(w, session.ID, errors.Localize(err, session.Language), http.StatusInternalServerError)
		}

		req.Code = string(bodyBytes)
	} else {
		// Otherwise, parse it as a well-formed JSON request.
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			status := http.StatusBadRequest
			if _, ok := err.(*http.MaxBytesError); ok {
				status = http.StatusRequestEntityTooLarge
			}

			return util.ErrorResponse(w, session.ID, errors.Localize(err, session.Language), status)
		}
	}

	if len(req.Code) > maxFormatCodeBytes {
		return util.ErrorResponse(w, session.ID, i18n.Text(session.Language, "error.admin.format.too.large"), http.StatusRequestEntityTooLarge)
	}

	if ui.IsActive(ui.RestLogger) {
		fake := formatRequest{Code: req.Code}
		if len(fake.Code) > 80 {
			fake.Code = fake.Code[:80]
		}

		b, _ := json.MarshalIndent(fake, ui.JSONIndentPrefix, ui.JSONIndentSpacer)
		ui.Log(ui.RestLogger, "rest.request.payload", ui.A{
			"session": session.ID,
			"body":    string(b),
		})
	}

	var resp formatResponse

	resp.Elapsed = time.Since(start).String()

	if file, err := parse.ParseAuto(req.Code); err != nil {
		resp.Error = err.Error()
	} else if out, err := format.File(file); err != nil {
		resp.Error = err.Error()
	} else {
		resp.Formatted = out
	}

	w.Header().Set("Content-Type", "application/json")

	_ = util.WriteJSON(w, resp, &session.ResponseLength)

	if ui.IsActive(ui.RestLogger) {
		fake := formatResponse{
			Formatted: resp.Formatted,
			Error:     resp.Error,
			Elapsed:   resp.Elapsed,
		}

		if len(fake.Formatted) > 80 {
			fake.Formatted = fake.Formatted[:80]
		}

		b, _ := json.MarshalIndent(fake, ui.JSONIndentPrefix, ui.JSONIndentSpacer)
		ui.Log(ui.RestLogger, "rest.response.payload", ui.A{
			"session": session.ID,
			"body":    string(b),
		})
	}

	return http.StatusOK
}
