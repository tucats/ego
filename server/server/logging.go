package server

import (
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/util"
)

// Debugging tool that dumps interesting things about a request. Only outputs
// when REST logging is enabled.
func LogRequest(r *http.Request, sessionID int) {
	if ui.IsActive(ui.RestLogger) {
		ui.Log(ui.RestLogger, "[%d] *** START NEW REQUEST ***", sessionID)
		ui.Log(ui.RestLogger, "[%d] %s %s from %s (%d bytes of request content)", sessionID, r.Method, r.URL.Path, r.RemoteAddr, r.ContentLength)

		queryParameters := r.URL.Query()
		parmMsg := strings.Builder{}

		// Query parameters are stored as a map to an array of values. Format this for readability. If the
		// parameter has no value or has a single value, just use it. Otherwise, format it as an array of values.
		for k, v := range queryParameters {
			parmMsg.WriteString("  ")
			parmMsg.WriteString(k)

			valueMsg := ""
			switch len(v) {
			case 0:
				valueMsg = ""

			case 1:
				valueMsg = "is " + v[0]

			default:
				valueMsg = "is [" + strings.Join(v, ", ") + "]"
			}

			parmMsg.WriteString(valueMsg)
		}

		if parmMsg.Len() > 0 {
			ui.WriteLog(ui.RestLogger, "[%d] Query parameters:\n%s", sessionID,
				util.SessionLog(sessionID, strings.TrimSuffix(parmMsg.String(), "\n")))
		}

		// Form a message summaring the header fields.
		headerMsg := strings.Builder{}

		for k, v := range r.Header {
			for _, i := range v {
				// A bit of a hack, but if this is the Authorization header, only show
				// the first token in the value (Bearer, Basic, etc) and obscure whatever
				// follows it.
				if strings.EqualFold(k, "Authorization") {
					f := strings.Fields(i)
					if len(f) > 0 {
						i = f[0] + " <hidden value>"
					}
				}

				headerMsg.WriteString("   ")
				headerMsg.WriteString(k)
				headerMsg.WriteString(": ")
				headerMsg.WriteString(i)
				headerMsg.WriteString("\n")
			}
		}

		ui.WriteLog(ui.RestLogger, "[%d] Request headers:\n%s",
			sessionID,
			util.SessionLog(sessionID,
				strings.TrimSuffix(headerMsg.String(), "\n"),
			))
	}
}

// Debugging tool that dumps interesting things about a request. Only outputs
// when REST logging is enabled.
func LogResponse(w http.ResponseWriter, sessionID int) {
	if ui.IsActive(ui.RestLogger) {
		headerMsg := strings.Builder{}

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

				headerMsg.WriteString("   ")
				headerMsg.WriteString(k)
				headerMsg.WriteString(": ")
				headerMsg.WriteString(i)
				headerMsg.WriteString("\n")
			}
		}

		if headerMsg.Len() > 0 {
			ui.WriteLog(ui.RestLogger, "[%d] Response headers:\n%s",
				sessionID,
				util.SessionLog(sessionID,
					strings.TrimSuffix(headerMsg.String(), "\n"),
				))
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
			ui.Log(ui.ServerLogger, "Memory: Allocated(%8.3fmb) Total(%8.3fmb) System(%8.3fmb) GC(%d) ",
				bToMb(currentStats.Alloc), bToMb(currentStats.TotalAlloc), bToMb(currentStats.Sys), currentStats.NumGC)
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
