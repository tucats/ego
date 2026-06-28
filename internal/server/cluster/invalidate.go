package cluster

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
)

// BroadcastCacheFlush notifies every active peer in the cluster that a
// particular in-memory cache has become stale and must be purged. It is a
// no-op when the server is running in standalone mode (ClusterName == "").
//
// Errors sending to individual peers are logged but do not abort the broadcast;
// a peer that is temporarily unreachable will be evicted by the health checker
// within 90 seconds anyway, so a missed flush is self-correcting.
//
// The cacheID parameter must be one of the integer constants defined in the
// caches package (e.g. caches.UserCache, caches.DSNCache). The mapping from
// integer to human-readable name is in defs.ClusterCacheNames.
func BroadcastCacheFlush(cacheID int) {
	if ClusterName == "" || systemDB == nil {
		return
	}

	peers, err := ListActiveMembers(systemDB)
	if err != nil {
		ui.Log(ui.ServerLogger, "cluster.broadcast.error", ui.A{
			"error": err.Error(),
		})

		return
	}

	for _, peer := range peers {
		if sendErr := SendCacheFlush(peer, cacheID); sendErr != nil {
			ui.Log(ui.ServerLogger, "cluster.flush.error", ui.A{
				"id":    peer.NodeID,
				"host":  peer.Host,
				"port":  peer.Port,
				"cache": cacheName(cacheID),
				"error": sendErr.Error(),
			})
		}
	}
}

// SendCacheFlush sends a cache-invalidation POST request to a single peer.
// It sets a short timeout so that a slow or unreachable peer does not hold
// up the caller's request handler.
//
// A non-nil error is returned when the HTTP request fails or the peer returns
// a non-2xx status. The caller (BroadcastCacheFlush) logs the error and moves
// on to the next peer.
func SendCacheFlush(peer defs.ClusterMember, cacheID int) error {
	payload := defs.ClusterFlushRequest{
		CacheID:  cacheID,
		SenderID: NodeID,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s://%s:%d/services/cluster/flush",
		peer.Scheme, peer.Host, peer.Port)

	client := &http.Client{Timeout: 5 * time.Second}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", ClusterAuthHeader())

	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return errors.New(errors.ErrClusterPeerHTTPStatus).Context(fmt.Sprintf("%s: HTTP %d", peer.NodeID, resp.StatusCode))
	}

	ui.Log(ui.ServerLogger, "cluster.flush.sent", ui.A{
		"id":    peer.NodeID,
		"cache": cacheName(cacheID),
	})

	return nil
}

// cacheName returns the human-readable label for a cache class integer.
// It uses the mapping in defs.ClusterCacheNames and falls back to the raw
// integer string for unknown cache IDs.
func cacheName(id int) string {
	if name, ok := defs.ClusterCacheNames[id]; ok {
		return name
	}

	return fmt.Sprintf("cache(%d)", id)
}
