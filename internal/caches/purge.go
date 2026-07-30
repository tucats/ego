package caches

import (
	"sort"

	"github.com/tucats/ego/internal/cli/ui"
)

// Purge is used to discard all elements of a given cache, identified by an integer key. If
// there is no such cache, no action is taken.
//
// Purge is the entry point for a purge that ORIGINATES on this node: in addition
// to discarding the local cache it fires the OnPurge hook, which in cluster mode
// broadcasts the invalidation to every peer.
//
// CLUSTER-1: use PurgeLocal instead when applying an invalidation that arrived
// FROM a peer. Calling Purge on that path makes each node re-broadcast every
// notification it receives, which is an unbounded feedback loop -- see the
// comment on PurgeLocal.
func Purge(id int) {
	purge(id, true)
}

// PurgeLocal discards all elements of a given cache exactly as Purge does, but
// does NOT fire the OnPurge hook, so it never notifies cluster peers.
//
// CLUSTER-1: this exists to break a notification loop. The chain was:
//
//	Node A: caches.Purge(X) -> OnPurge -> POST /services/cluster/flush to peers
//	Node B: flush handler   -> caches.Purge(X) -> OnPurge -> POST back to A
//	Node A: flush handler   -> caches.Purge(X) -> ... forever
//
// Because the inbound-flush handler called the same public Purge that triggers
// the broadcast, every node that received a notification sent one back out.
// Two nodes traded flushes indefinitely; with three or more the traffic
// multiplied by (peers-1) on every round, so it grew exponentially.
//
// Nothing stopped it on its own: the SenderID in the request was only logged,
// and the hook fires even when there was no such cache locally, so "nothing left
// to purge" did not end the cascade either.
//
// The rule to remember: a purge that originates here should broadcast, and a
// purge that arrived from somewhere else must not. Making that the difference
// between two functions means the terminating case is chosen at the call site
// rather than depending on a runtime check that a future edit could drop.
func PurgeLocal(id int) {
	purge(id, false)
}

// purge holds the shared implementation. notify controls whether the OnPurge
// hook is fired after the local cache is discarded.
func purge(id int, notify bool) {
	cacheLock.Lock()
	defer cacheLock.Unlock()

	if !active {
		return
	}

	if cache, found := cacheList[id]; found {
		ui.Log(ui.CacheLogger, "cache.purge", ui.A{
			"name":  class(id),
			"id":    cache.ID,
			"count": len(cache.Items)})

		delete(cacheList, id)
	} else {
		ui.Log(ui.CacheLogger, "cache.purge", ui.A{
			"name":  class(id),
			"id":    int32(0),
			"count": 0})
	}

	if !notify {
		return
	}

	// If a cluster broadcast hook is registered, notify peers that this cache
	// is now stale. The hook is set by server/cluster at startup and is nil
	// (no-op) when running in standalone mode.
	//
	// Note that this fires even when the cache did not exist locally, and that is
	// deliberate: a peer may well have that cache populated even though this node
	// never built it, so the peer still needs to be told. Restricting the
	// broadcast to the "found" case would silently skip invalidations that other
	// nodes need.
	if OnPurge != nil {
		go OnPurge(id)
	}
}

// PurgeAll purges all defined caches. It uses the map of cache ID to name to get the
// list of cache ID values.
func PurgeAll() {
	ui.Log(ui.CacheLogger, "cache.purge.all", nil)

	// Get the list of cache ID values and sort them in ascending order. This ensures
	// that the order they are printed in the log is consistent across different runs
	// of the application.
	keys := make([]int, 0, len(cacheList))
	for id := range cacheList {
		keys = append(keys, id)
	}

	sort.Ints(keys)

	// Now that we know the range of all ID values, purge them in ascending order.
	for _, id := range keys {
		Purge(id)
	}
}
