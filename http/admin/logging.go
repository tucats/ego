package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/util"
)

// loggingAction is the rest handler for /admin/logging endpoint.
func loggingAction(sessionID int, w http.ResponseWriter, r *http.Request) int {
	loggers := defs.LoggingItem{}
	response := defs.LoggingResponse{
		ServerInfo: util.MakeServerInfo(sessionID),
	}
	status := http.StatusOK

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
				ui.Log(ui.RestLogger, "[%d] Bad logger name: %s", sessionID, loggerName)

				return util.ErrorResponse(w, sessionID, err.Error(), http.StatusBadRequest)
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
			return util.ErrorResponse(w, sessionID, err.Error(), http.StatusBadRequest)
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
		ui.Log(ui.RestLogger, "[%d] 405 Unsupported method %s", sessionID, r.Method)

		return util.ErrorResponse(w, sessionID, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
