package cluster

import (
	"encoding/json"
	"net/http"

	"github.com/tucats/ego/internal/caches"
	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/i18n"
	"github.com/tucats/ego/internal/language/data"
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
//	    Permissions(defs.RootPermission)
func ClusterStatusHandler(session *router.Session, w http.ResponseWriter, r *http.Request) int {
	sysDb, err := openSystemDB(nil)
	if err != nil {
		return util.ErrorResponse(w, session.ID, errors.Localize(err, session.Language), http.StatusBadRequest)
	}

	name := ""
	if n, found := session.URLParts[name]; found {
		name = data.String(n)
	}

	members, err := ListAllActiveMembers(sysDb, name)
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
	b := util.WriteJSON(w, session.Response(), http.StatusOK, response)

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
	// This route has no session-based authentication at all (registered
	// Authentication(false, false)) -- the cluster token IS the only
	// credential this endpoint recognizes, so a missing or invalid one is
	// "not authenticated" (401), not "authenticated but forbidden" (403).
	// RFC 7235 §3.1: 401 covers a request that lacks valid credentials,
	// whether none were sent or the ones sent don't check out.
	if !ValidateClusterToken(r) {
		w.Header().Set(defs.AuthenticateHeader, `Bearer realm="cluster"`)

		return util.ErrorResponse(w, session.ID, i18n.Text(session.Language, "error.cluster.token.invalid"), http.StatusUnauthorized)
	}

	// Decode the cache flush request from the JSON body.
	var req defs.ClusterFlushRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return util.ErrorResponse(w, session.ID, "invalid request body: "+errors.Localize(err, session.Language), http.StatusBadRequest)
	}

	cacheID := req.CacheID

	// CLUSTER-1: reject a notification that has been relayed too many times.
	//
	// This should never fire. With the PurgeLocal call below, a received
	// notification is never re-broadcast, so hop counts do not grow and every
	// legitimate request arrives with a count of 1. The check is a circuit breaker
	// against a future change that reintroduces forwarding: rather than letting a
	// storm run unbounded, the cluster would go quiet after maxFlushHops rounds
	// and leave a clear trail of warnings in the log explaining why.
	//
	// A request from an older build carries no hops field, which decodes as 0 and
	// so passes this check as a first-hop message.
	if req.Hops > maxFlushHops {
		ui.Log(ui.ServerLogger, "cluster.flush.hops", ui.A{
			"session": session.ID,
			"cache":   cacheName(cacheID),
			"peer":    req.SenderID,
			"hops":    req.Hops,
			"limit":   maxFlushHops,
		})

		// Answer 200 rather than an error status. The sender cannot do anything
		// useful with a failure here, and returning an error would only add a
		// second stream of log noise on its side while a loop is being contained.
		return writeFlushResponse(session, w, cacheID)
	}

	// PurgeLocal, NOT Purge. This is the single line that breaks the notification
	// loop described in CLUSTER-1: Purge fires the OnPurge hook, which broadcasts
	// the invalidation to every peer, so calling it here would make this node
	// answer every notification it receives with a notification of its own. Two
	// nodes did that to each other indefinitely. PurgeLocal discards the cache
	// without notifying anyone, which is exactly right for a purge that arrived
	// from somewhere else -- the originating node has already told the other peers.
	caches.PurgeLocal(cacheID)

	ui.Log(ui.ServerLogger, "cluster.flush", ui.A{
		"session": session.ID,
		"cache":   cacheName(cacheID),
		"peer":    req.SenderID,
	})

	return writeFlushResponse(session, w, cacheID)
}

// writeFlushResponse sends the JSON reply for a cache-flush request. It is a
// separate function because FlushCacheHandler has two exits that both need to
// answer 200 with the same body: the normal one after the cache is purged, and
// the CLUSTER-1 hop-limit path that deliberately does no work.
func writeFlushResponse(session *router.Session, w http.ResponseWriter, cacheID int) int {
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
	util.WriteJSON(w, session.Response(), http.StatusOK, response)

	return http.StatusOK
}

// authorizeClusterOrAdmin checks the two credentials ClusterShutdownHandler
// and ClusterRemoveHandler each accept -- a valid cluster HMAC token, or an
// authenticated admin session -- and returns 0 if either one is present.
//
// A failure is 401 if the caller proved no Ego identity at all
// (session.Authenticated is false: no valid Ego credential was presented,
// whether or not a cluster token was attempted), and 403 if the caller *is*
// an authenticated Ego user who simply isn't an admin. Before this, both
// cases were folded into one check and one status, so "no credentials at
// all" and "valid credentials, wrong role" were indistinguishable from the
// response, and the former was misreported as 403 (RFC 7235 §3.1: 401 is
// for a request that lacks valid credentials).
func authorizeClusterOrAdmin(session *router.Session, w http.ResponseWriter, r *http.Request) int {
	if ValidateClusterToken(r) || session.Admin {
		return 0
	}

	if !session.Authenticated {
		w.Header().Set(defs.AuthenticateHeader, `Bearer realm="cluster"`)

		return util.ErrorResponse(w, session.ID, i18n.Text(session.Language, "error.cluster.auth.invalid"), http.StatusUnauthorized)
	}

	return util.ErrorResponse(w, session.ID, i18n.Text(session.Language, "error.cluster.auth.invalid"), http.StatusForbidden)
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
// Authentication: cluster HMAC token, or an authenticated admin session.
//
// Route registration:
//
//	r.New("/services/cluster/shutdown", cluster.ClusterShutdownHandler, http.MethodPost).
//	    Authentication(false, false)
func ClusterShutdownHandler(session *router.Session, w http.ResponseWriter, r *http.Request) int {
	if status := authorizeClusterOrAdmin(session, w, r); status != 0 {
		return status
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
// Authentication: cluster HMAC token, or an authenticated admin session.
//
// Route registration:
//
//	r.New("/services/cluster/remove", cluster.ClusterRemoveHandler, http.MethodPost).
//	    Authentication(false, false)
func ClusterRemoveHandler(session *router.Session, w http.ResponseWriter, r *http.Request) int {
	if status := authorizeClusterOrAdmin(session, w, r); status != 0 {
		return status
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
	util.WriteJSON(w, session.Response(), http.StatusOK, response)

	return http.StatusOK
}
