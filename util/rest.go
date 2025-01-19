package util

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
)

// MakeServerInfo creates a server info object for the current server instance
// and the given session ID value. (Session ID is an integer value that uniquely
// identifies the web service call to this server instance.) The resulting header
// can be encoded as JSON and included in the HTTP response body.
func MakeServerInfo(sessionID int) defs.ServerInfo {
	hostName := Hostname()
	result := defs.ServerInfo{
		Hostname: hostName,
		ID:       defs.InstanceID,
		Session:  sessionID,
		Version:  defs.APIVersion,
	}

	return result
}

// MakeBaseCollection creates a base collection object for the current server instance
// that can be returend as a response to a client request. This collection object is
// incomplete; the caller should fill in the additional count and start values.
func MakeBaseCollection(sessionID int) defs.BaseCollection {
	result := defs.BaseCollection{
		ServerInfo: MakeServerInfo(sessionID),
		Status:     http.StatusOK,
	}

	return result
}

// ErrorResponse returns an error response to the client with the given session ID,
// message, and status code. The error response is encoded as JSON and written as
// the HTTP response body. Additionally, the HTTP headers indicating the status code
// is set in the response and is returned as the function value. This is intended
// to be used as the exit opertion from a REST API handler when an error occurs.
func ErrorResponse(w http.ResponseWriter, sessionID int, msg string, status int) int {
	response := defs.RestStatusResponse{
		ServerInfo: MakeServerInfo(sessionID),
		Message:    msg,
		Status:     status,
	}

	if status < 100 || status >= 600 {
		status = http.StatusInternalServerError
	}

	// Remove noise from postgres errors, which all have a "pq: " prefix which we want
	// to remove from the error message text -- no need to indicate that we're using
	// postgres for some underlying functionality.
	msg = strings.TrimPrefix(msg, "pq: ")
	msg = strings.Replace(msg, " pq: ", "", 1)

	// Construct a neatly formatted JSON response.
	b, _ := json.MarshalIndent(response, "", "  ")

	// Add the Content-Type header to indicate we are sending back an error response.
	w.Header().Add(defs.ContentTypeHeader, defs.ErrorMediaType)

	// Write the error response to the HTTP response body.
	w.WriteHeader(status)

	// Write the JSON-encoded error response to the HTTP response body.
	_, _ = w.Write(b)

	// If the REST logger is active log the message to the log along with a text representation
	// of the JSON error response payload.
	if ui.IsActive(ui.RestLogger) {
		ui.Log(ui.RestLogger, "[%d] error, %s; %d", sessionID, msg, status)
		ui.WriteLog(ui.RestLogger, "[%d] Error response payload:\n%s", sessionID, SessionLog(sessionID, string(b)))
	}

	return status
}
