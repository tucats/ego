package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/http/server"
	"github.com/tucats/ego/util"
)

// SetLoggingHandler handles POST requests to the logger endpoint. The payload is a map of
// the loggers to be enabled or disabled.
func SetLoggingHandler(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(r.Body)
	loggers := defs.LoggingItem{}

	if err := json.Unmarshal(buf.Bytes(), &loggers); err != nil {
		ui.Log(ui.RestLogger, "[%d] Bad payload: %v", session.ID, err)

		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusBadRequest)
	}

	for loggerName, mode := range loggers.Loggers {
		logger := ui.LoggerByName(loggerName)
		if logger < 0 || (logger == ui.ServerLogger && !mode) {
			return util.ErrorResponse(w, session.ID, "Invalid logger name: "+loggerName, http.StatusBadRequest)
		}

		modeString := "enable"
		if !mode {
			modeString = "disable"
		}

		ui.Log(ui.RestLogger, "[%d] %s %s(%d) logger", session.ID, modeString, loggerName, logger)
		ui.Active(logger, mode)
	}

	return GetLoggingHandler(session, w, r)
}

// GetLoggingHandler handles GET requests to the logging endpoint, and returns a payload describing
// the state of logging in the server. This function is also used by the POST handler to return
// revised logging state.
func GetLoggingHandler(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	response := defs.LoggingResponse{
		ServerInfo: util.MakeServerInfo(session.ID),
	}

	response.Filename = ui.CurrentLogFile()
	response.Loggers = map[string]bool{}
	response.RetainCount = ui.LogRetainCount
	response.ServerInfo = util.MakeServerInfo(session.ID)

	for _, k := range ui.LoggerNames() {
		response.Loggers[k] = ui.IsActive(ui.LoggerByName(k))
	}

	w.Header().Add(defs.ContentTypeHeader, defs.LogStatusMediaType)

	b, _ := json.Marshal(response)
	_, _ = w.Write(b)

	return http.StatusOK
}

// PurgeLogHandler handles the DELETE post to the logging endpoint, which tells the server to
// purge old versions of the log file.
//
// The request can optionally have a "keep" URL parameter which overrides the default number
// of log entries to keep. A keep value of less than 1 is the same as 1.
func PurgeLogHandler(session *server.Session, w http.ResponseWriter, r *http.Request) int {
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
		ServerInfo: util.MakeServerInfo(session.ID),
		Count:      count}

	w.Header().Add(defs.ContentTypeHeader, defs.RowCountMediaType)

	b, _ := json.Marshal(reply)
	_, _ = w.Write(b)

	return http.StatusOK
}
