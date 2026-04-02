package webapp

import (
	"context"
	"net/http"
)

// handleQuit responds immediately then shuts the server down gracefully.
func handleQuit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)

		return
	}

	w.WriteHeader(http.StatusNoContent)

	go func() {
		_ = httpServer.Shutdown(context.Background())
	}()
}
