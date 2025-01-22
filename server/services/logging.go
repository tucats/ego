package services

import (
	"net/http"
	"strings"

	"github.com/tucats/ego/app-cli/ui"
)

func additionalServerRequestLogging(r *http.Request, sessionID int) string {
	requestor := r.RemoteAddr

	if forward := r.Header.Get("X-Forwarded-For"); forward != "" {
		addrs := strings.Split(forward, ",")
		requestor = addrs[0]
	}

	ui.Log(ui.RestLogger, "rest.request", ui.A{
		"session": sessionID,
		"method":  r.Method,
		"path":    r.URL.Path,
		"host":    requestor})
	ui.Log(ui.RestLogger, "rest.agent", ui.A{
		"session": sessionID,
		"agent":   r.UserAgent()})

	if p := parameterString(r); p != "" {
		ui.Log(ui.RestLogger, "rest.parameters", ui.A{
			"session": sessionID,
			"params":  p})
	}

	if ui.IsActive(ui.InfoLogger) {
		for headerName, headerValues := range r.Header {
			if strings.EqualFold(headerName, "Authorization") {
				continue
			}

			ui.WriteLog(ui.InfoLogger, "request.header.values", ui.A{
				"session": sessionID,
				"key":     headerName,
				"values":  headerValues})
		}
	}

	return requestor
}
