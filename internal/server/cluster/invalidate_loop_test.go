package cluster

// Regression tests for the CLUSTER-1 fix: applying a cache invalidation that
// arrived from a peer must not send another invalidation back out.
//
// The loop these guard against was:
//
//	Node A: caches.Purge(X) -> OnPurge -> POST /services/cluster/flush to peers
//	Node B: flush handler   -> caches.Purge(X) -> OnPurge -> POST back to A
//	Node A: flush handler   -> caches.Purge(X) -> ... forever
//
// Two nodes traded flushes indefinitely; with three or more, the traffic
// multiplied by (peers-1) every round.

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tucats/ego/internal/caches"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/router"
)

// purgeHookRecorder counts OnPurge invocations. The hook is dispatched on its own
// goroutine by the caches package ("go OnPurge(id)"), so the counter is only read
// through settled(), which gives that goroutine time to run first.
type purgeHookRecorder struct {
	calls chan int
}

func newPurgeHookRecorder(t *testing.T) *purgeHookRecorder {
	t.Helper()

	// A buffered channel is used instead of an int counter so the test goroutine
	// and the hook goroutine never touch the same variable. That keeps the test
	// clean under "go test -race", which would flag an unsynchronized counter.
	recorder := &purgeHookRecorder{calls: make(chan int, 64)}

	saved := caches.OnPurge
	caches.OnPurge = func(id int) { recorder.calls <- id }

	t.Cleanup(func() { caches.OnPurge = saved })

	return recorder
}

// settled returns the number of hook calls seen once no new call has arrived for
// a short grace period. The grace period is what makes a "zero calls" assertion
// meaningful: without it the test could pass simply by reading the counter before
// the hook goroutine had a chance to increment it.
func (r *purgeHookRecorder) settled() int {
	count := 0

	for {
		select {
		case <-r.calls:
			count++

		case <-time.After(250 * time.Millisecond):
			return count
		}
	}
}

// clusterTestMode puts the package into cluster mode for the duration of a test
// so that ValidateClusterToken accepts the request, and restores the previous
// values afterwards.
func clusterTestMode(t *testing.T) {
	t.Helper()

	savedName := ClusterName
	savedNode := NodeID

	ClusterName = "cluster-1-test"
	NodeID = "this-node"

	t.Cleanup(func() {
		ClusterName = savedName
		NodeID = savedNode
	})

	caches.Active(true)
}

// flushRequest builds an inbound cache-flush request the way SendCacheFlush does.
func flushRequest(t *testing.T, cacheID int, sender string, hops int) *http.Request {
	t.Helper()

	payload, err := json.Marshal(defs.ClusterFlushRequest{
		CacheID:  cacheID,
		SenderID: sender,
		Hops:     hops,
	})
	if err != nil {
		t.Fatalf("marshal flush payload: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/services/cluster/flush", bytes.NewReader(payload))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", ClusterAuthHeader())

	return request
}

// TestInboundFlushDoesNotRebroadcast is the test that matters. Before the fix the
// handler called caches.Purge, so handling one inbound notification produced
// another outbound one and the cluster never went quiet.
func TestInboundFlushDoesNotRebroadcast_CLUSTER1(t *testing.T) {
	clusterTestMode(t)

	recorder := newPurgeHookRecorder(t)

	// Seed the cache so the purge has something real to discard, proving the
	// handler still does its job while staying silent.
	caches.Add(caches.AuthCache, "seeded-key", "seeded-value")

	response := httptest.NewRecorder()
	session := &router.Session{ID: 1, Language: "en"}

	if status := FlushCacheHandler(session, response, flushRequest(t, caches.AuthCache, "peer-a", 1)); status != http.StatusOK {
		t.Fatalf("FlushCacheHandler returned %d, want 200; body: %s", status, response.Body.String())
	}

	if calls := recorder.settled(); calls != 0 {
		t.Errorf("handling an inbound flush fired the broadcast hook %d time(s), want 0; the notification loop is still present", calls)
	}

	// The purge itself must still have happened -- breaking the loop must not
	// break invalidation.
	if _, found := caches.Find(caches.AuthCache, "seeded-key"); found {
		t.Error("inbound flush did not actually purge the cache")
	}
}

// TestLocallyOriginatedPurgeStillBroadcasts is the other half of the contract. It
// would be easy to "fix" the loop by suppressing the broadcast everywhere, which
// would silently stop cluster invalidation from working at all.
func TestLocallyOriginatedPurgeStillBroadcasts_CLUSTER1(t *testing.T) {
	clusterTestMode(t)

	recorder := newPurgeHookRecorder(t)

	caches.Purge(caches.UserCache)

	if calls := recorder.settled(); calls != 1 {
		t.Errorf("a locally originated Purge fired the broadcast hook %d time(s), want 1", calls)
	}
}

// TestPurgeLocalNeverBroadcasts covers the new function directly.
func TestPurgeLocalNeverBroadcasts_CLUSTER1(t *testing.T) {
	clusterTestMode(t)

	recorder := newPurgeHookRecorder(t)

	caches.Add(caches.DSNCache, "key", "value")
	caches.PurgeLocal(caches.DSNCache)

	if calls := recorder.settled(); calls != 0 {
		t.Errorf("PurgeLocal fired the broadcast hook %d time(s), want 0", calls)
	}

	if _, found := caches.Find(caches.DSNCache, "key"); found {
		t.Error("PurgeLocal did not discard the cache")
	}
}

// TestFlushBeyondHopLimitIsDropped covers the CLUSTER-1 circuit breaker. A
// notification that has been relayed more than maxFlushHops times is ignored
// rather than acted on, so a reintroduced forwarding bug would burn out after a
// bounded number of rounds.
func TestFlushBeyondHopLimitIsDropped_CLUSTER1(t *testing.T) {
	clusterTestMode(t)

	newPurgeHookRecorder(t)

	caches.Add(caches.SchemaCache, "survivor", "value")

	response := httptest.NewRecorder()
	session := &router.Session{ID: 2, Language: "en"}

	request := flushRequest(t, caches.SchemaCache, "peer-b", maxFlushHops+1)

	if status := FlushCacheHandler(session, response, request); status != http.StatusOK {
		t.Fatalf("FlushCacheHandler returned %d, want 200", status)
	}

	// Over the limit means the flush is not applied at all.
	if _, found := caches.Find(caches.SchemaCache, "survivor"); !found {
		t.Error("a flush over the hop limit was applied; it should have been dropped")
	}
}

// TestFlushAtHopLimitIsAccepted pins down the boundary so the comparison cannot
// drift into an off-by-one that silently discards legitimate notifications.
func TestFlushAtHopLimitIsAccepted_CLUSTER1(t *testing.T) {
	clusterTestMode(t)

	newPurgeHookRecorder(t)

	caches.Add(caches.TokenCache, "doomed", "value")

	response := httptest.NewRecorder()
	session := &router.Session{ID: 3, Language: "en"}

	if status := FlushCacheHandler(session, response, flushRequest(t, caches.TokenCache, "peer-c", maxFlushHops)); status != http.StatusOK {
		t.Fatalf("FlushCacheHandler returned %d, want 200", status)
	}

	if _, found := caches.Find(caches.TokenCache, "doomed"); found {
		t.Error("a flush exactly at the hop limit was dropped; it should have been applied")
	}
}

// TestFlushWithNoHopsFieldIsAccepted covers compatibility with a peer running an
// older build, which sends no hops field at all. That decodes as zero and must be
// treated as a first-hop message rather than rejected.
func TestFlushWithNoHopsFieldIsAccepted_CLUSTER1(t *testing.T) {
	clusterTestMode(t)

	newPurgeHookRecorder(t)

	caches.Add(caches.BlacklistCache, "doomed", "value")

	// Hand-built body with no "hops" key, as an older node would send.
	body := []byte(`{"cache_id":` + itoa(caches.BlacklistCache) + `,"sender_id":"old-peer"}`)

	request := httptest.NewRequest(http.MethodPost, "/services/cluster/flush", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", ClusterAuthHeader())

	response := httptest.NewRecorder()
	session := &router.Session{ID: 4, Language: "en"}

	if status := FlushCacheHandler(session, response, request); status != http.StatusOK {
		t.Fatalf("FlushCacheHandler returned %d, want 200", status)
	}

	if _, found := caches.Find(caches.BlacklistCache, "doomed"); found {
		t.Error("a flush from an older peer (no hops field) was dropped; it should have been applied")
	}
}

// itoa avoids pulling strconv in just for the one hand-built request body above.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}

	digits := ""

	for n > 0 {
		digits = string(rune('0'+n%10)) + digits
		n /= 10
	}

	return digits
}
