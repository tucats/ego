package admin

import (
	"net/http"

	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/http/server"
)

// HeartbeatHandler receives the /admin/heartbeat calls. This does nothing
// but respond with success. The event is not logged.
func HeartbeatHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("X-Ego-Server", defs.ServerInstanceID)
	w.WriteHeader(http.StatusOK)
	server.CountRequest(server.HeartbeatRequestCounter)
}
