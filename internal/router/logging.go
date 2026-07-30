package router

import (
	"net/http"
	"runtime"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/tucats/ego/internal/cli/settings"
	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/util"
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
//
// GORTNS-4: the stop channel lets this task be shut down cleanly. It used to be a
// bare "for { ... time.Sleep(duration) }" with no way out, so the goroutine ran
// until the process died and a second launch would have quietly produced two of
// them logging over each other.
//
// The caller closes the channel to signal shutdown; nothing is ever sent on it.
// Closing is the idiomatic broadcast in Go, because a receive from a closed
// channel returns immediately for every receiver. Passing nil is legal and means
// "never stop" -- a receive on a nil channel blocks forever, so the select below
// simply always takes its timer case.
func LogMemoryStatistics(stop <-chan struct{}) {
	var (
		lastRequestNumber int32
		loggedError       bool
	)

	// Pause for a moment to allow the initialization to complete before putting out
	// the first memory usage message.
	if !sleepOrStop(100*time.Millisecond, stop) {
		return
	}

	for {
		// Has there been a request since the last time we logged? If so, let's log
		// the new information.
		if atomic.LoadInt32(&SequenceNumber) > lastRequestNumber {
			var currentStats runtime.MemStats

			runtime.ReadMemStats(&currentStats)

			// Log the information.
			ui.Log(ui.ServerLogger, "server.memory", ui.A{
				"alloc":  bToMb(currentStats.Alloc),
				"total":  bToMb(currentStats.TotalAlloc),
				"system": bToMb(currentStats.Sys),
				"cycles": currentStats.NumGC})

			lastRequestNumber = atomic.LoadInt32(&SequenceNumber)
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

		// Wait for the next interval, but wake up early and return if the server
		// is shutting down.
		if !sleepOrStop(duration, stop) {
			return
		}
	}
}

// sleepOrStop waits for the given duration, or until the stop channel is closed,
// whichever happens first. It reports true if the full duration elapsed (so the
// caller should keep going) and false if the stop signal arrived (so the caller
// should return).
//
// This is the standard Go alternative to time.Sleep for a cancellable loop. A
// plain Sleep cannot be interrupted: the goroutine is parked for the whole
// duration and cannot notice a shutdown until it wakes up on its own. A select
// over the stop channel and a timer channel wakes on whichever is ready first.
//
// Passing a nil stop channel means "sleep the full duration, never cancel",
// because a receive on a nil channel blocks forever and can never be chosen.
func sleepOrStop(duration time.Duration, stop <-chan struct{}) bool {
	// time.NewTimer rather than time.After so the timer can be released promptly
	// on the stop path instead of being left to fire into a channel nobody reads.
	timer := time.NewTimer(duration)
	defer timer.Stop()

	select {
	case <-stop:
		return false
	case <-timer.C:
		return true
	}
}

// bToMb is a helper function that converts a total number of bytes to a fractional
// number of megabytes. This is used for formatting the memory statistics log entries.
func bToMb(b uint64) float64 {
	return float64(b) / 1024.0 / 1024.0
}
