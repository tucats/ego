package webapp

import (
	"context"
	"net/http"
	"sync"
	"time"
)

const (
	// pingInterval is how often the browser sends a heartbeat (must match the JS value).
	pingInterval = 2 * time.Second

	// pingTimeout is how long the server waits without a ping before shutting down.
	// Two missed pings gives a comfortable buffer for local network hiccups.
	pingTimeout = pingInterval * 2
)

var (
	pingMu    sync.Mutex
	pingOnce  bool
	pingTimer *time.Timer
)

// handlePing resets the watchdog timer. The browser calls this on a fixed
// interval; if the pings stop (tab closed, window closed, navigated away)
// the timer fires and shuts the server down gracefully.
func handlePing(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)

		return
	}

	pingMu.Lock()
	if !pingOnce {
		// Arm the watchdog on the first ping so we don't race against browser startup.
		pingOnce = true
		pingTimer = time.AfterFunc(pingTimeout, func() {
			_ = httpServer.Shutdown(context.Background())
		})
	} else {
		pingTimer.Reset(pingTimeout)
	}
	pingMu.Unlock()

	w.WriteHeader(http.StatusNoContent)
}
