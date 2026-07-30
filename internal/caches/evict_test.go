package caches

// Regression tests for the GORTNS-2 fix: the caches package now notifies an
// optional OnEvict callback when an individual item leaves a cache, so an owner
// whose cached value holds a resource (a parked goroutine, an open handle) can
// release it instead of leaking it.

import (
	"sync"
	"testing"
	"time"
)

// evictionRecorder collects the evictions reported to OnEvict. A mutex protects
// it because the expiration scan runs on its own goroutine, so the test
// goroutine and the scan goroutine can touch it at the same time -- and the Go
// race detector will (correctly) fail the test if that access is unsynchronized.
type evictionRecorder struct {
	mutex  sync.Mutex
	events []string
}

func (r *evictionRecorder) record(key any, value any) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.events = append(r.events, key.(string)+"="+value.(string))
}

func (r *evictionRecorder) count() int {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	return len(r.events)
}

func (r *evictionRecorder) all() []string {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	return append([]string{}, r.events...)
}

// withOnEvict installs an eviction handler for the duration of a test and puts
// the previous value back afterwards, so tests cannot leak global state into
// each other.
func withOnEvict(t *testing.T, handler func(id int, key any, value any)) {
	t.Helper()

	saved := evictHandler()

	SetOnEvict(handler)

	t.Cleanup(func() {
		SetOnEvict(saved)
	})
}

// TestDeleteNotifiesOnEvict covers the explicit-delete path.
func TestDeleteNotifiesOnEvict_GORTNS2(t *testing.T) {
	Active(true)

	recorder := &evictionRecorder{}

	withOnEvict(t, func(id int, key any, value any) {
		if id == AuthCache {
			recorder.record(key, value)
		}
	})

	Add(AuthCache, "key-1", "value-1")

	if !Delete(AuthCache, "key-1") {
		t.Fatal("Delete reported the item was not present")
	}

	if got := recorder.all(); len(got) != 1 || got[0] != "key-1=value-1" {
		t.Errorf("OnEvict events = %v, want [key-1=value-1]", got)
	}

	// Deleting something that is not there must not report an eviction.
	Delete(AuthCache, "key-1")

	if recorder.count() != 1 {
		t.Errorf("OnEvict fired %d times, want 1; a missing key should not report an eviction", recorder.count())
	}
}

// TestExpirationNotifiesOnEvict covers the path that actually caused the leak:
// an item aging out via the background expiration scan. Before the fix the map
// entry was simply dropped and nobody was told, so an abandoned debug session's
// goroutine stayed parked forever.
func TestExpirationNotifiesOnEvict_GORTNS2(t *testing.T) {
	Active(true)

	recorder := &evictionRecorder{}

	withOnEvict(t, func(id int, key any, value any) {
		if id == AuthCache {
			recorder.record(key, value)
		}
	})

	// Add the item, then force it to be already expired by rewriting its
	// expiration to a time in the past. This avoids having to wait out a real
	// expiration interval.
	Add(AuthCache, "stale", "payload")

	cacheLock.Lock()

	if cache, found := cacheList[AuthCache]; found {
		if item, found := cache.Items["stale"]; found {
			item.Expires = time.Now().Add(-time.Hour)
			cache.Items["stale"] = item
		}
	}

	cacheLock.Unlock()

	// Drive one expiration sweep directly rather than waiting for the scan
	// goroutine's timer, so the test is fast and deterministic.
	sweepExpired(AuthCache)

	if got := recorder.all(); len(got) != 1 || got[0] != "stale=payload" {
		t.Errorf("OnEvict events = %v, want [stale=payload]", got)
	}
}

// TestOnEvictRunsUnlocked verifies the guarantee OnEvict documents: the callback
// is invoked with no cache lock held, so an implementation may call back into
// this package without deadlocking. If the callback ran under the lock, the
// Find call below would block forever and the test would time out.
func TestOnEvictRunsUnlocked_GORTNS2(t *testing.T) {
	Active(true)

	reentered := false

	withOnEvict(t, func(id int, key any, value any) {
		if id != AuthCache {
			return
		}

		// Calling back into the cache from inside the callback is the operation
		// that would deadlock if the lock were still held.
		_, _ = Find(AuthCache, "other")

		reentered = true
	})

	Add(AuthCache, "reentry", "value")
	Delete(AuthCache, "reentry")

	if !reentered {
		t.Error("eviction callback did not run")
	}
}
