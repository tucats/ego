package services

import (
	"net/http"
	"strings"

	"github.com/tucats/ego/app-cli/ui"
)

func additionalServerRequestLogging(r *http.Request, sessionID int32) string {
	requestor := r.RemoteAddr
	if forward := r.Header.Get("X-Forwarded-For"); forward != "" {
		addrs := strings.Split(forward, ",")
		requestor = addrs[0]
	}

	ui.Log(ui.RestLogger, "[%d] %s %s from %v", sessionID, r.Method, r.URL.Path, requestor)
	ui.Log(ui.RestLogger, "[%d] User agent: %s", sessionID, r.Header.Get("User-Agent"))

	if p := parameterString(r); p != "" {
		ui.Log(ui.RestLogger, "[%d] request parameters:  %s", sessionID, p)
	}

	if ui.IsActive(ui.InfoLogger) {
		for headerName, headerValues := range r.Header {
			if strings.EqualFold(headerName, "Authorization") {
				continue
			}

			ui.WriteLog(ui.InfoLogger, "[%d] header: %s %v", sessionID, headerName, headerValues)
		}
	}

	return requestor
}
