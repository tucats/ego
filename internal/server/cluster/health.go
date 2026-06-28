package cluster

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	"github.com/tucats/ego/internal/cli/settings"
	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
)

// maxConsecutiveFailures is the number of back-to-back failed health-check
// pings that must occur before a peer is evicted from the cluster. With the
// default 30-second ping interval this gives a 90-second grace period.
const maxConsecutiveFailures = 3

// StartHealthChecker runs an infinite loop that pings every active peer in
// the cluster at regular intervals. It must be launched as a goroutine from
// RunServer immediately after cluster.Initialize returns.
//
// Each iteration:
//  1. Reads the current active peer list from the shared system database.
//  2. Pings each peer via HTTP GET /services/up.
//  3. Tracks consecutive failures per peer in a local map.
//  4. Evicts any peer that has failed maxConsecutiveFailures times in a row.
//  5. Updates this node's own last_seen timestamp to signal liveness to peers.
//
// The function returns immediately if ClusterName is empty (standalone mode),
// so the caller can always launch it as a goroutine without a conditional.
func StartHealthChecker() {
	if ClusterName == "" || systemDB == nil {
		return
	}

	// failures maps nodeID → consecutive ping failures for each known peer.
	failures := make(map[string]int)

	interval := pingInterval()

	for {
		time.Sleep(interval)

		peers, err := ListActiveMembers(systemDB)
		if err != nil {
			ui.Log(ui.ServerLogger, "cluster.health.list.error", ui.A{
				"error": err.Error(),
			})

			continue
		}

		for _, peer := range peers {
			if pingErr := pingPeer(peer); pingErr != nil {
				failures[peer.NodeID]++
				count := failures[peer.NodeID]

				ui.Log(ui.ServerLogger, "cluster.ping.fail", ui.A{
					"id":    peer.NodeID,
					"host":  peer.Host,
					"port":  peer.Port,
					"count": count,
					"max":   maxConsecutiveFailures,
				})

				if count >= maxConsecutiveFailures {
					evictPeer(peer)
					delete(failures, peer.NodeID)
				}
			} else {
				// Successful ping: reset the failure counter for this peer.
				failures[peer.NodeID] = 0
			}
		}

		// Update our own last_seen so that peers do not evict us.
		if err := UpdateLastSeen(systemDB, NodeID); err != nil {
			ui.Log(ui.ServerLogger, "cluster.health.self.update.error", ui.A{
				"error": err.Error(),
			})
		}
	}
}

// pingPeer sends a GET request to the peer's /services/up endpoint. This is
// the same liveness probe used by the apitest suite. A timeout of 5 seconds
// (configurable via ego.cluster.ping.timeout) prevents a slow peer from
// blocking the whole health-check round.
func pingPeer(peer defs.ClusterMember) error {
	timeout := pingTimeout()
	url := fmt.Sprintf("%s://%s:%d/services/up", peer.Scheme, peer.Host, peer.Port)

	client := &http.Client{
		Timeout: timeout,
		// Skip TLS verification when pinging peers on the same private network.
		// In production, all nodes share the same certificate authority, so
		// strict verification would work, but this keeps setup simple.
		Transport: &http.Transport{
			TLSClientConfig: tlsClientConfig(),
		},
	}

	resp, err := client.Get(url)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return errors.New(errors.ErrClusterPeerHTTPStatus).Context(fmt.Sprintf("%s: HTTP %d", peer.NodeID, resp.StatusCode))
	}

	return nil
}

// evictPeer removes a non-responsive peer from the active cluster membership
// and logs the eviction event.
func evictPeer(peer defs.ClusterMember) {
	if err := RemoveMember(systemDB, peer.NodeID); err != nil {
		ui.Log(ui.ServerLogger, "cluster.evict.error", ui.A{
			"id":    peer.NodeID,
			"error": err.Error(),
		})

		return
	}

	ui.Log(ui.ServerLogger, "cluster.evict", ui.A{
		"id":      peer.NodeID,
		"name":    ClusterName,
		"timeout": fmt.Sprintf("%ds", int(pingInterval().Seconds())*maxConsecutiveFailures),
	})
}

// pingInterval returns the duration between health-check rounds. It reads
// ego.cluster.ping.interval from the configuration and defaults to 30 seconds.
func pingInterval() time.Duration {
	s := settings.Get(defs.ClusterPingIntervalSetting)
	if d, err := time.ParseDuration(s); err == nil {
		return d
	}

	return 30 * time.Second
}

// tlsClientConfig returns a TLS configuration that skips certificate
// verification. Cluster peers on a private network share the same CA, but
// requiring strict verification would complicate setup. Production deployments
// that need strict verification can extend this function to load a shared cert.
func tlsClientConfig() *tls.Config {
	return &tls.Config{InsecureSkipVerify: true} //nolint:gosec
}

// pingTimeout returns the per-request timeout for a single health-check ping.
// It reads ego.cluster.ping.timeout from the configuration and defaults to 5 s.
func pingTimeout() time.Duration {
	s := settings.Get(defs.ClusterPingTimeoutSetting)
	if d, err := time.ParseDuration(s); err == nil {
		return d
	}

	return 5 * time.Second
}
