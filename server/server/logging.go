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
				"values":  headerMap[k]})
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

// LogMemoryStatitics is a go-routine launched when a server is started. It generates a logging
// entry every ten minutes indicating the current memory allocation, the total memory ever
// allocated, the system memory, and the number of times the garbage-collector has run.
func LogMemoryStatistics() {
	var (
		lastRequestNumber int32
	)

	// Pause for a moment to allow the initialization to complete before putting out
	// the first memory usage message.
	time.Sleep(100 * time.Millisecond)

	for {
		// Has there been a request since the last time we logged? If so, let's log
		// the new informaiton.
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

		// Generate this report in the log every ten minutes.
		time.Sleep(5 * time.Minute)
	}
}

// bToMb is a helper function that converts a total number of bytes to a fractional
// number of megabytes. This is used for formatting the memory statistics log entries.
func bToMb(b uint64) float64 {
	return float64(b) / 1024.0 / 1024.0
}
