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

// maxFlushHops bounds how many times a cache-invalidation notification may be
// relayed between nodes before a receiver refuses to act on it.
//
// CLUSTER-1: this is a safety net, not the mechanism that prevents the
// notification loop. The loop is prevented structurally, by having the inbound
// flush handler call caches.PurgeLocal (which does not broadcast) rather than
// caches.Purge (which does). With that in place a notification is never relayed,
// so a well-behaved cluster only ever sends hop count 1 and this limit is never
// reached.
//
// It exists because the structural fix is one call site away from being undone.
// If a future change made the receive path broadcast again, the hop count would
// climb and every node would start rejecting the notification after this many
// rounds -- turning an unbounded storm into a brief, bounded, and very visible
// burst of log warnings. For that to work, any code that forwards a notification
// it received MUST pass the incoming hop count plus one; see SendCacheFlush.
const maxFlushHops = 4

// originHopCount is the hop count used for a notification that starts on this
// node, as opposed to one being relayed. It is 1 rather than 0 so that a
// hand-written or older-build request carrying no hops field (which decodes as 0)
// is still accepted as a first-hop message.
const originHopCount = 1

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

	peers, err := ListActiveMembers(systemDB, ClusterName)
	if err != nil {
		ui.Log(ui.ServerLogger, "cluster.broadcast.error", ui.A{
			"error": err.Error(),
		})

		return
	}

	// A purge that originates on this node starts at the first hop.
	for _, peer := range peers {
		if sendErr := SendCacheFlush(peer, cacheID, originHopCount); sendErr != nil {
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
//
// CLUSTER-1: hops is the hop count to place in the request. Pass
// originHopCount for a notification that starts on this node. A caller that is
// forwarding a notification it received must pass the incoming count plus one,
// or the receiving side's maxFlushHops circuit breaker cannot do its job.
func SendCacheFlush(peer defs.ClusterMember, cacheID int, hops int) error {
	payload := defs.ClusterFlushRequest{
		CacheID:  cacheID,
		SenderID: NodeID,
		Hops:     hops,
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
