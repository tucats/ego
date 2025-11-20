package server

import (
	"net/http"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/util"
)

// Debugging tool that dumps interesting things about a request. Only outputs
// when REST logging is enabled.
func LogRequest(r *http.Request, sessionID int) {
	if ui.IsActive(ui.RestLogger) {
		ui.Log(ui.RestLogger, "rest.start", ui.A{
			"session": sessionID})

		ui.Log(ui.RestLogger, "rest.start.info", ui.A{
			"session": sessionID,
			"method":  r.Method,
			"path":    r.URL.Path,
			"host":    r.RemoteAddr,
			"size":    r.ContentLength})

		// Make simple maps from the headers and query parameters.
		queryParameters := r.URL.Query()

		parmMap := make(map[string][]string)
		for k, v := range queryParameters {
			parmMap[k] = v
		}

		headerMap := make(map[string][]string)

		// Copy the non-sensitive header values from the request.
		for k, v := range r.Header {
			if util.NonSensitiveHeader(k) {
				headerMap[k] = v
			}
		}

		// Log the parameters by making an alphabetical list of them and then logging them.
		keys := make([]string, 0)
		for k := range parmMap {
			keys = append(keys, k)
		}

		sort.Strings(keys)

		for _, k := range keys {
			ui.Log(ui.RestLogger, "rest.parameter.values", ui.A{
				"session": sessionID,
				"key":     k,
				"values":  parmMap[k]})
		}

		// Now repeat again with the header map.
		keys = make([]string, 0)
		for k := range headerMap {
			keys = append(keys, k)
		}

		sort.Strings(keys)

		for _, k := range keys {
			if util.NonSensitiveHeader(k) {
				ui.Log(ui.RestLogger, "rest.header.values", ui.A{
					"session": sessionID,
					"key":     k,
					"values":  headerMap[k]})
			}
		}
	}
}

// Debugging tool that dumps interesting things about a request. Only outputs
// when REST logging is enabled.
func LogResponse(w http.ResponseWriter, sessionID int) {
	if ui.IsActive(ui.RestLogger) {
		for k, v := range w.Header() {
			if strings.EqualFold(k, "Authorization") {
				v = []string{"<hidden value>"}
			}

			ui.WriteLog(ui.RestLogger, "rest.response.header", ui.A{
				"session": sessionID,
				"name":    k,
				"values":  v})
		}
	}
}

// LogMemoryStatistics is a go-routine launched when a server is started. It generates a logging
// entry every ten minutes indicating the current memory allocation, the total memory ever
// allocated, the system memory, and the number of times the garbage-collector has run.
func LogMemoryStatistics() {
	var (
		lastRequestNumber int32
		loggedError       bool
	)

	// Pause for a moment to allow the initialization to complete before putting out
	// the first memory usage message.
	time.Sleep(100 * time.Millisecond)

	for {
		// Has there been a request since the last time we logged? If so, let's log
		// the new information.
		if SequenceNumber > lastRequestNumber {
			var currentStats runtime.MemStats

			runtime.ReadMemStats(&currentStats)

			// Log the information.
			ui.Log(ui.ServerLogger, "server.memory", ui.A{
				"alloc":  bToMb(currentStats.Alloc),
				"total":  bToMb(currentStats.TotalAlloc),
				"system": bToMb(currentStats.Sys),
				"cycles": currentStats.NumGC})

			lastRequestNumber = SequenceNumber
		}

		// Sleep for the expected interval. If not in the configuration, or the
		// duration string is invalid, use the default of 5 minutes.
		defaultDuration := settings.Get(defs.MemoryLogIntervalSetting)
		if defaultDuration == "" {
			defaultDuration = "5m"
		}

		duration, err := time.ParseDuration(defaultDuration)
		if err != nil {
			duration = 5 * time.Minute

			// If we have an error and haven't already logged it, do so now. Remember
			// that we've done this so the error only comes out once.
			if !loggedError {
				ui.Log(ui.ServerLogger, "server.config.error", ui.A{
					"setting": defs.MemoryLogIntervalSetting,
					"error":   errors.ErrInvalidDuration.Clone().Context(defaultDuration).Error()})

				loggedError = true
			}
		} else {
			loggedError = false
		}

		time.Sleep(duration)
	}
}

// bToMb is a helper function that converts a total number of bytes to a fractional
// number of megabytes. This is used for formatting the memory statistics log entries.
func bToMb(b uint64) float64 {
	return float64(b) / 1024.0 / 1024.0
}
