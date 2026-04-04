package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/egostrings"
	"github.com/tucats/ego/server/server"
	"github.com/tucats/ego/util"
)

// SetLoggingHandler is the HTTP handler for POST /admin/loggers. The request
// body is a JSON object (defs.LoggingItem) that names one or more loggers and
// the desired on/off state for each.  An optional RetainCount field sets how
// many old log files the server keeps before discarding them.
//
// After applying every change the handler delegates to GetLoggingHandler so
// the response always reflects the new, authoritative logging state rather than
// just echoing the request back.
func SetLoggingHandler(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	// bytes.Buffer is an in-memory byte buffer.  We read the entire request body
	// into it at once so the raw bytes are available for both json.Unmarshal and
	// for the REST-logger diagnostic message below.
	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(r.Body)

	// loggers will receive the decoded request payload.  defs.LoggingItem has
	// two fields: RetainCount (int) and Loggers (map[string]bool).
	loggers := defs.LoggingItem{}

	if err := json.Unmarshal(buf.Bytes(), &loggers); err != nil {
		ui.Log(ui.RestLogger, "rest.bad.payload", ui.A{
			"session": session.ID,
			"error":   err})

		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusBadRequest)
	}

	// Log the raw request body for diagnostic purposes.
	ui.Log(ui.RestLogger, "rest.request.payload", ui.A{
		"session": session.ID,
		"body":    buf.String()})

	// If the caller supplied a positive RetainCount, update the in-memory
	// global and persist it as a server default setting.  strconv.Itoa
	// converts the integer to the string form required by settings.SetDefault.
	if loggers.RetainCount > 0 {
		ui.LogRetainCount = loggers.RetainCount
		settings.SetDefault("ego.server.log.retain", strconv.Itoa(loggers.RetainCount))
	}

	// Iterate over every logger name → bool entry in the request map.
	// ui.LoggerByName resolves the string name to an integer logger ID; -1
	// means the name is not recognized.  The server logger cannot be disabled
	// (it is the audit trail), so we reject that combination as well.
	for loggerName, mode := range loggers.Loggers {
		logger := ui.LoggerByName(loggerName)
		if logger < 0 || (logger == ui.ServerLogger && !mode) {
			return util.ErrorResponse(w, session.ID, "Invalid logger name: "+loggerName, http.StatusBadRequest)
		}

		// Build a human-readable mode string purely for the log message.
		modeString := "enable"
		if !mode {
			modeString = "disable"
		}

		ui.Log(ui.RestLogger, "rest.set.logger", ui.A{
			"session":  session.ID,
			"mode":     modeString,
			"loggerid": logger,
			"logger":   loggerName})

		// ui.Active(logger, mode) turns the named logger on (true) or off (false).
		ui.Active(logger, mode)
	}

	// Delegate to GetLoggingHandler to build and write the response.  This
	// ensures the caller receives the full current logging state — including any
	// loggers that were not mentioned in this request — rather than a partial echo.
	return GetLoggingHandler(session, w, r)
}

// GetLoggingHandler is the HTTP handler for GET /admin/loggers. It returns a
// snapshot of the server's current logging configuration: the active log file
// path, the retain count, and the on/off state of every named logger.
//
// SetLoggingHandler also calls this function directly after applying changes so
// that both the GET and POST endpoints return identically shaped responses.
func GetLoggingHandler(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	// Start with the standard response envelope (ServerInfo + Status).
	response := defs.LoggingResponse{
		ServerInfo: util.MakeServerInfo(session.ID),
		Status:     http.StatusOK,
	}

	// Fill in the current log-file path and retain count from the ui package's
	// global state, then rebuild the Loggers map fresh from all known loggers.
	response.Filename = ui.CurrentLogFile()
	response.Loggers = map[string]bool{}
	response.RetainCount = ui.LogRetainCount
	response.ServerInfo = util.MakeServerInfo(session.ID)

	// ui.LoggerNames() returns a slice of every logger name registered with the
	// ui package.  ui.LoggerByName converts each name back to its integer ID so
	// we can call ui.IsActive to get its current state.
	for _, k := range ui.LoggerNames() {
		response.Loggers[k] = ui.IsActive(ui.LoggerByName(k))
	}

	w.Header().Add(defs.ContentTypeHeader, defs.LogStatusMediaType)
	b := util.WriteJSON(w, response, &session.ResponseLength)

	if ui.IsActive(ui.RestLogger) {
		ui.WriteLog(ui.RestLogger, "rest.response.payload", ui.A{
			"session": session.ID,
			"body":    string(b)})
	}

	return http.StatusOK
}

// PurgeLogHandler is the HTTP handler for DELETE /admin/loggers. It tells the
// server to delete old log files, retaining at most `keep` files.
//
// The caller may pass a "keep" query parameter to override the server's current
// retain count for this one purge operation.  A keep value less than 1 is
// clamped to 1 so at least the current log file is always preserved.
func PurgeLogHandler(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	var err error

	// Default to the server's current retain count so a bare DELETE (without
	// a "keep" parameter) behaves consistently with the configured policy.
	keep := ui.LogRetainCount

	// r.URL.Query() parses the raw query string into a url.Values map, which is
	// a map[string][]string.  Each key maps to a slice of values because HTTP
	// allows the same query parameter to appear multiple times.
	q := r.URL.Query()

	if v, found := q["keep"]; found {
		// We only honour the first occurrence of the "keep" parameter.
		if len(v) == 1 {
			// egostrings.Atoi is a locale-aware wrapper around strconv.Atoi.
			keep, err = egostrings.Atoi(v[0])
			if err != nil {
				return util.ErrorResponse(w, session.ID, "Invalid keep value: "+v[0], http.StatusBadRequest)
			}
		}
	}

	// Clamp: the server must always keep at least one log file.
	if keep < 1 {
		keep = 1
	}

	// Update the in-memory retain count and purge old log files.  ui.PurgeLogs
	// returns the number of files that were actually removed.
	ui.LogRetainCount = keep
	count := ui.PurgeLogs()

	// defs.DBRowCount is a generic "how many rows were affected" response
	// envelope — we reuse it here to report how many log files were deleted.
	response := defs.DBRowCount{
		ServerInfo: util.MakeServerInfo(session.ID),
		Count:      count,
		Status:     http.StatusOK,
	}

	w.Header().Add(defs.ContentTypeHeader, defs.RowCountMediaType)
	b := util.WriteJSON(w, response, &session.ResponseLength)

	if ui.IsActive(ui.RestLogger) {
		ui.WriteLog(ui.RestLogger, "rest.response.payload", ui.A{
			"session": session.ID,
			"body":    string(b)})
	}

	return http.StatusOK
}
