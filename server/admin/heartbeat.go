package admin

import (
	"net/http"

	"github.com/tucats/ego/router"
)

// HeartbeatHandler receives the /admin/heartbeat calls. This does nothing
// but respond with success.
func HeartbeatHandler(session *router.Session, w http.ResponseWriter, r *http.Request) int {
	w.WriteHeader(http.StatusOK)

	return http.StatusOK
}
