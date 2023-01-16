package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/util"
)

// loggingAction is the rest handler for /admin/logging endpoint.
func loggingAction(sessionID int32, w http.ResponseWriter, r *http.Request) int {
	loggers := defs.LoggingItem{}
	response := defs.LoggingResponse{
		ServerInfo: util.MakeServerInfo(sessionID),
	}
	status := http.StatusOK

	user, hasAdminPrivileges := isAdminRequestor(r)
	if !hasAdminPrivileges {
		ui.Log(ui.AuthLogger, "[%d] User %s not authorized", sessionID, user)
		util.ErrorResponse(w, sessionID, "Not authorized", http.StatusForbidden)

		return http.StatusForbidden
	}

	logHeaders(r, sessionID)

	switch r.Method {
	case http.MethodPost:
		buf := new(bytes.Buffer)
		_, _ = buf.ReadFrom(r.Body)

		err := json.Unmarshal(buf.Bytes(), &loggers)
		if err != nil {
			status = http.StatusBadRequest
			util.ErrorResponse(w, sessionID, err.Error(), status)
			ui.Log(ui.RestLogger, "[%d] Bad payload: %v", sessionID, err)

			return http.StatusBadRequest
		}

		for loggerName, mode := range loggers.Loggers {
			logger := ui.LoggerByName(loggerName)
			if logger < 0 || (logger == ui.ServerLogger && !mode) {
				status = http.StatusBadRequest
				util.ErrorResponse(w, sessionID, err.Error(), status)
				ui.Log(ui.RestLogger, "[%d] Bad logger name: %s", sessionID, loggerName)

				return http.StatusBadRequest
			}

			modeString := "enable"
			if !mode {
				modeString = "disable"
			}

			ui.Log(ui.RestLogger, "[%d] %s %s(%d) logger", sessionID, modeString, loggerName, logger)
			ui.Active(logger, mode)
		}

		fallthrough

	case http.MethodGet:
		response.Filename = ui.CurrentLogFile()
		response.Loggers = map[string]bool{}
		response.RetainCount = ui.LogRetainCount
		response.ServerInfo = util.MakeServerInfo(sessionID)

		for _, k := range ui.LoggerNames() {
			response.Loggers[k] = ui.IsActive(ui.LoggerByName(k))
		}

		w.Header().Add(contentTypeHeader, defs.LogStatusMediaType)

		b, _ := json.Marshal(response)
		_, _ = w.Write(b)

		return http.StatusOK

	case http.MethodDelete:
		if err := util.ValidateParameters(r.URL, map[string]string{"keep": "int"}); err != nil {
			status = http.StatusBadRequest
			util.ErrorResponse(w, sessionID, err.Error(), status)

			return http.StatusBadRequest
		}

		keep := ui.LogRetainCount
		q := r.URL.Query()

		if v, found := q["keep"]; found {
			if len(v) == 1 {
				keep, _ = strconv.Atoi(v[0])
			}
		}

		if keep < 1 {
			keep = 1
		}

		ui.LogRetainCount = keep
		count := ui.PurgeLogs()

		reply := defs.DBRowCount{
			ServerInfo: util.MakeServerInfo(sessionID),
			Count:      count}

		w.Header().Add(contentTypeHeader, defs.RowCountMediaType)

		b, _ := json.Marshal(reply)
		_, _ = w.Write(b)

		return status

	default:
		status = http.StatusMethodNotAllowed

		ui.Log(ui.RestLogger, "[%d] 405 Unsupported method %s", sessionID, r.Method)
		util.ErrorResponse(w, sessionID, "Method not allowed", status)

		return status
	}
}

func logHeaders(r *http.Request, sessionID int32) {
	if ui.IsActive(ui.InfoLogger) {
		for headerName, headerValues := range r.Header {
			if strings.EqualFold(headerName, "Authorization") {
				continue
			}

			ui.Log(ui.InfoLogger, "[%d] header: %s %v", sessionID, headerName, headerValues)
		}
	}
}
