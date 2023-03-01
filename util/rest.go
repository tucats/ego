package util

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
)

func MakeServerInfo(sessionID int) defs.ServerInfo {
	hostName := Hostname()
	result := defs.ServerInfo{
		Hostname: hostName,
		ID:       defs.ServerInstanceID,
		Session:  sessionID,
		Version:  defs.APIVersion,
	}

	return result
}

func MakeBaseCollection(sessionID int) defs.BaseCollection {
	result := defs.BaseCollection{
		ServerInfo: MakeServerInfo(sessionID),
	}

	return result
}

func ErrorResponse(w http.ResponseWriter, sessionID int, msg string, status int) int {
	response := defs.RestStatusResponse{
		ServerInfo: MakeServerInfo(sessionID),
		Message:    msg,
		Status:     status,
	}

	if status < 100 || status >= 600 {
		status = http.StatusInternalServerError
	}

	// Remove noise from postgres errors.
	msg = strings.TrimPrefix(msg, "pq: ")
	msg = strings.Replace(msg, " pq: ", "", 1)

	b, _ := json.MarshalIndent(response, "", "  ")

	ui.Log(ui.RestLogger, "[%d] error, %s; %d", sessionID, msg, status)

	w.Header().Add(defs.ContentTypeHeader, defs.ErrorMediaType)
	w.WriteHeader(status)
	_, _ = w.Write(b)

	if ui.IsActive(ui.RestLogger) {
		ui.WriteLog(ui.RestLogger, "[%d] Error response payload:\n%s", sessionID, SessionLog(sessionID, string(b)))
	}

	return status
}
