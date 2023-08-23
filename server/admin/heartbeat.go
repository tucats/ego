package admin

import (
	"net/http"

	"github.com/tucats/ego/server/server"
)

// HeartbeatHandler receives the /admin/heartbeat calls. This does nothing
// but respond with success.
func HeartbeatHandler(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	w.WriteHeader(http.StatusOK)

	return http.StatusOK
}
