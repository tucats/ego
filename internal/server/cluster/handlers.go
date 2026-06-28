package cluster

import (
	"encoding/json"
	"net/http"

	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/caches"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/i18n"
	"github.com/tucats/ego/internal/router"
	"github.com/tucats/ego/internal/util"
)

// ClusterStatusHandler handles GET /services/cluster. It returns the full list
// of cluster members (both active and recently removed) from the system database.
//
// Authentication: accepts either a standard admin token or the cluster HMAC
// token. This allows both human operators (using admin credentials) and peer
// nodes (using the cluster token) to query membership.
//
// Route registration (in commands/server.go):
//
//	r.New("/services/cluster", cluster.ClusterStatusHandler, http.MethodGet).
//	    Authentication(true, true)
func ClusterStatusHandler(session *router.Session, w http.ResponseWriter, r *http.Request) int {
	if systemDB == nil {
		return util.ErrorResponse(w, session.ID, i18n.Text(session.Language, "error.cluster.not.running"), http.StatusNotFound)
	}

	members, err := ListMembers(systemDB)
	if err != nil {
		return util.ErrorResponse(w, session.ID, errors.Localize(err, session.Language), http.StatusInternalServerError)
	}

	response := defs.ClusterStatusResponse{
		ClusterName: ClusterName,
		Members:     members,
		ServerInfo:  util.MakeServerInfo(session.ID),
		Status:      http.StatusOK,
	}

	w.Header().Add(defs.ContentTypeHeader, defs.JSONMediaType)
	b := util.WriteJSON(w, response, &session.ResponseLength)

	if ui.IsActive(ui.RestLogger) {
		ui.WriteLog(ui.RestLogger, "rest.response.payload", ui.A{
			"session": session.ID,
			"body":    string(b),
		})
	}

	return http.StatusOK
}

// FlushCacheHandler handles POST /services/cluster/flush/{cache-id}. It is
// called by a peer node to notify this node that a particular in-memory cache
// has become stale and must be discarded. The next request that needs that data
// will reload it fresh from the shared system database.
//
// Authentication: cluster HMAC token only (not standard admin credentials).
// The route is registered without requiring auth so that the auth layer does
// not reject the non-standard token format; the handler performs its own
// token validation.
//
// Route registration:
//
//	r.New("/services/cluster/flush/{cache-id}", cluster.FlushCacheHandler, http.MethodPost).
//	    Authentication(false, false)
func FlushCacheHandler(session *router.Session, w http.ResponseWriter, r *http.Request) int {
	if !ValidateClusterToken(r) {
		return util.ErrorResponse(w, session.ID, i18n.Text(session.Language, "error.cluster.token.invalid"), http.StatusForbidden)
	}

	// Decode the cache flush request from the JSON body.
	var req defs.ClusterFlushRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return util.ErrorResponse(w, session.ID, "invalid request body: "+errors.Localize(err, session.Language), http.StatusBadRequest)
	}

	cacheID := req.CacheID
	caches.Purge(cacheID)

	ui.Log(ui.ServerLogger, "cluster.flush", ui.A{
		"session": session.ID,
		"cache":   cacheName(cacheID),
		"peer":    req.SenderID,
	})

	response := struct {
		defs.ServerInfo `json:"server"`
		Status          int    `json:"status"`
		Message         string `json:"msg"`
		Cache           string `json:"cache"`
	}{
		ServerInfo: util.MakeServerInfo(session.ID),
		Status:     http.StatusOK,
		Cache:      cacheName(cacheID),
	}

	w.Header().Add(defs.ContentTypeHeader, defs.JSONMediaType)
	util.WriteJSON(w, response, &session.ResponseLength)

	return http.StatusOK
}

// ClusterShutdownHandler handles POST /services/cluster/shutdown. A peer node
// (or an operator using the CLI) sends this request to instruct this node to
// leave the cluster cleanly and shut down its HTTP server.
//
// The handler updates the cluster table to mark this node "removed", then
// triggers a graceful server shutdown by calling router.DownHandler. The HTTP
// response is sent before the shutdown completes so the caller receives a
// confirmation.
//
// Authentication: cluster HMAC token only.
//
// Route registration:
//
//	r.New("/services/cluster/shutdown", cluster.ClusterShutdownHandler, http.MethodPost).
//	    Authentication(false, false)
func ClusterShutdownHandler(session *router.Session, w http.ResponseWriter, r *http.Request) int {
	if !ValidateClusterToken(r) && !session.Admin {
		return util.ErrorResponse(w, session.ID, i18n.Text(session.Language, "error.cluster.auth.invalid"), http.StatusForbidden)
	}

	ui.Log(ui.ServerLogger, "cluster.shutdown", ui.A{
		"session": session.ID,
		"peer":    r.Header.Get("X-Cluster-Node"),
	})

	// Mark this node as removed in the cluster table before we go down.
	Shutdown()

	// Trigger server shutdown using the existing down handler. Lots a shutdown
	// by admin function, and tells the router to stop accepting new requests immediately.
	return router.DownHandler(session, w, r)
}

// ClusterRemoveHandler handles POST /services/cluster/remove. An operator uses
// this to forcibly evict a non-responsive peer from the cluster membership table
// without sending a shutdown request to that peer (which would fail anyway if
// the peer is unreachable). The health checker would eventually evict it too,
// but this provides an immediate operator override.
//
// The node to evict is specified via the "node_id" query parameter.
//
// Authentication: cluster HMAC token only.
//
// Route registration:
//
//	r.New("/services/cluster/remove", cluster.ClusterRemoveHandler, http.MethodPost).
//	    Authentication(false, false)
func ClusterRemoveHandler(session *router.Session, w http.ResponseWriter, r *http.Request) int {
	if !ValidateClusterToken(r) && !session.Admin {
		return util.ErrorResponse(w, session.ID, i18n.Text(session.Language, "error.cluster.auth.invalid"), http.StatusForbidden)
	}

	if systemDB == nil {
		return util.ErrorResponse(w, session.ID, i18n.Text(session.Language, "error.cluster.not.running"), http.StatusNotFound)
	}

	nodeID := r.URL.Query().Get("node_id")
	if nodeID == "" {
		return util.ErrorResponse(w, session.ID, i18n.Text(session.Language, "error.cluster.node.id.required"), http.StatusBadRequest)
	}

	if err := RemoveMember(systemDB, nodeID); err != nil {
		return util.ErrorResponse(w, session.ID, errors.Localize(err, session.Language), http.StatusInternalServerError)
	}

	ui.Log(ui.ServerLogger, "cluster.evict", ui.A{
		"session": session.ID,
		"id":      nodeID,
		"name":    ClusterName,
		"timeout": "manual",
	})

	response := struct {
		defs.ServerInfo `json:"server"`
		Status          int    `json:"status"`
		Message         string `json:"msg"`
		NodeID          string `json:"node_id"`
	}{
		ServerInfo: util.MakeServerInfo(session.ID),
		Status:     http.StatusOK,
		NodeID:     nodeID,
	}

	w.Header().Add(defs.ContentTypeHeader, defs.JSONMediaType)
	util.WriteJSON(w, response, &session.ResponseLength)

	return http.StatusOK
}
