package util

import (
	"encoding/json"
	"net/http"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
)

func MakeServerInfo(sessionID int32) defs.ServerInfo {
	hostName := Hostname()
	result := defs.ServerInfo{
		Hostname: hostName,
		ID:       defs.ServerInstanceID,
		Session:  int(sessionID),
		Version:  defs.APIVersion,
	}

	return result
}

func MakeBaseCollection(sessionID int32) defs.BaseCollection {
	result := defs.BaseCollection{
		ServerInfo: MakeServerInfo(sessionID),
	}

	return result
}

func ErrorResponse(w http.ResponseWriter, sessionID int32, msg string, status int) {
	response := defs.RestStatusResponse{
		ServerInfo: MakeServerInfo(sessionID),
		Message:    msg,
		Status:     status,
	}

	b, _ := json.MarshalIndent(response, "", "  ")

	ui.Debug(ui.ServerLogger, "[%d] %s; %d", sessionID, msg, status)

	w.Header().Add("Content-Type", defs.ErrorMediaType)
	w.WriteHeader(status)
	_, _ = w.Write(b)
}
