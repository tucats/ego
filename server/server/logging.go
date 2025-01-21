package server

import (
	"net/http"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/tucats/ego/app-cli/ui"
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

		for k, v := range r.Header {
			if strings.EqualFold(k, "Authorization") {
				v = []string{"<hidden values>"}
			}

			headerMap[k] = v
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
			ui.Log(ui.RestLogger, "rest.header.values", ui.A{
				"session": sessionID,
				"key":     k,
				"value":   headerMap[k]})
		}
	}
}

// Debugging tool that dumps interesting things about a request. Only outputs
// when REST logging is enabled.
func LogResponse(w http.ResponseWriter, sessionID int) {
	if ui.IsActive(ui.RestLogger) {
		keys := []string{}
		values := []string{}

		for k, v := range w.Header() {
			for _, i := range v {
				// A bit of a hack, but if this is the Authorization header, only show
				// the first token in the value (Bearer, Basic, etc) and obscure whatever
				// data follows it.
				if strings.EqualFold(k, "Authorization") {
					f := strings.Fields(i)
					if len(f) > 0 {
						i = f[0] + " <hidden value>"
					}
				}

				keys = append(keys, k)
				values = append(values, i)
			}
		}

		for n, value := range values {
			ui.WriteLog(ui.RestLogger, "rest.response.header", ui.A{
				"session": sessionID,
				"key":     keys[n],
				"value":   value})
		}
	}
}

// LogMemoryStatitics is a go-routine launched when a server is started. It generates a logging
// entry every ten minutes indicating the current memory allocation, the total memory ever
// allocated, the system memory, and the number of times the garbage-collector has run.
func LogMemoryStatistics() {
	var previousStats runtime.MemStats

	// Pause for a moment to allow the initialization to complete before putting out
	// the first memory usage message.
	time.Sleep(100 * time.Millisecond)

	for {
		// For info on each, see: https://golang.org/pkg/runtime/#MemStats
		var currentStats runtime.MemStats

		runtime.ReadMemStats(&currentStats)

		// If any of the values have changed since last time, put out the memory report. This is meant to keep the
		// log quiet when the server is idle for an extended period of time.
		if (currentStats.Alloc != previousStats.Alloc) ||
			(currentStats.TotalAlloc != previousStats.TotalAlloc) ||
			(currentStats.Sys != previousStats.Sys) ||
			(currentStats.NumGC != previousStats.NumGC) {
			ui.Log(ui.ServerLogger, "server.memory", ui.A{
				"alloc":  bToMb(currentStats.Alloc),
				"total":  bToMb(currentStats.TotalAlloc),
				"system": bToMb(currentStats.Sys),
				"cycles": currentStats.NumGC})
		}

		previousStats = currentStats

		// Generate this report in the log every ten minutes.
		time.Sleep(10 * time.Minute)
	}
}

// bToMb is a helper function that converts a total number of bytes to a fractional
// number of megabytes. This is used for formatting the memory statistics log entries.
func bToMb(b uint64) float64 {
	return float64(b) / 1024.0 / 1024.0
}
