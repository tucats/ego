package caches

// Tests for the MaxSize/HasLogged behavior in Add(): once a cache reaches its
// MaxSize, further Add() calls must be rejected rather than growing the cache
// past its limit; the "cache is full" condition must be logged only once per
// over-capacity episode, not on every rejected Add(); and the "already
// logged" flag must reset once the cache's size drops back below 95% of
// MaxSize, so a later full episode logs again instead of being permanently
// silenced by the first one.

import (
	"testing"
)

// TestAddRejectsWhenFullAndLogsOnce covers all three behaviors above.
//
// UserCache is used as the cache class under test. It is a general-purpose
// cache with no production Add()/Find() call sites anywhere in the server --
// nothing else in the test binary touches it, so this test cannot collide
// with any other test's cache state, and there is no need to save/restore
// anything about it beyond purging it when done.
//
// Rather than going through newCache() (which sizes a new cache from the
// package-level MaxCacheSize var, itself possibly overridden by whatever
// ego.server.cache.maxsize happens to be configured in the environment
// running the test), this test fabricates the cache entry directly with a
// small MaxSize of 10. That keeps the test fully deterministic regardless of
// any settings the machine running it might have -- the "artificially set
// the cache size small for testing" the feature calls for, scoped to only
// this one cache instance rather than the shared package-level default.
func TestAddRejectsWhenFullAndLogsOnce(t *testing.T) {
	Active(true)

	PurgeLocal(UserCache) // start from a clean slate, in case an earlier run left state behind

	t.Cleanup(func() {
		PurgeLocal(UserCache)
	})

	cacheLock.Lock()
	cacheList[UserCache] = Cache{
		ID:      sequenceNumber.Add(1),
		Items:   map[any]Item{},
		MaxSize: 10,
	}
	cacheLock.Unlock()

	// Fill the cache to exactly its MaxSize.
	for i := 0; i < 10; i++ {
		Add(UserCache, i, i)
	}

	if got := Size(UserCache); got != 10 {
		t.Fatalf("Size(UserCache) after filling = %d, want 10", got)
	}

	// a) An 11th item must be rejected: the count must not grow past MaxSize,
	// and the rejected item must not be found in the cache afterward.
	Add(UserCache, "overflow-1", "x")

	if got := Size(UserCache); got != 10 {
		t.Fatalf("Size(UserCache) after an overflow Add = %d, want 10 (the item should have been rejected)", got)
	}

	if _, found := Find(UserCache, "overflow-1"); found {
		t.Error("a rejected item was found in the cache")
	}

	if !cacheHasLogged(t, UserCache) {
		t.Fatal("HasLogged was not set to true after the cache became full")
	}

	// b) Repeated overflow attempts must not log again. Add() only calls
	// ui.Log when HasLogged transitions from false to true (see the "if
	// !cache.HasLogged" guard), so HasLogged staying true across every one
	// of these confirms the log line was skipped each time -- logged exactly
	// once for this whole episode, not once per rejected Add().
	for i := 0; i < 5; i++ {
		Add(UserCache, "overflow-more", i)

		if !cacheHasLogged(t, UserCache) {
			t.Fatalf("HasLogged was cleared by a rejected Add() on iteration %d; it should stay true while the cache is still full", i)
		}
	}

	if got := Size(UserCache); got != 10 {
		t.Fatalf("Size(UserCache) after repeated overflow Adds = %d, want 10", got)
	}

	// c) Dropping the cache below 95% of MaxSize (9.5, so 9 items or fewer
	// out of a MaxSize of 10) must reset HasLogged, so a later full episode
	// logs again. The reset happens lazily inside the next Add() call (the
	// "else" branch beside the MaxSize check), not proactively from Delete,
	// so the delete alone is not enough to observe it -- the Add() right
	// after is what actually notices the smaller size and clears the flag.
	Delete(UserCache, 0)

	if got := Size(UserCache); got != 9 {
		t.Fatalf("Size(UserCache) after one delete = %d, want 9", got)
	}

	Add(UserCache, "refill", "y") // brings the count back to 10; this call performs the reset

	if cacheHasLogged(t, UserCache) {
		t.Fatal("HasLogged did not reset once the cache dropped below 95% of MaxSize")
	}

	if got := Size(UserCache); got != 10 {
		t.Fatalf("Size(UserCache) after the refill Add = %d, want 10", got)
	}

	// A fresh overflow must log again now that HasLogged has been reset,
	// proving the first full episode did not permanently silence logging.
	Add(UserCache, "overflow-2", "z")

	if !cacheHasLogged(t, UserCache) {
		t.Error("HasLogged did not become true again for a second full episode")
	}
}

// cacheHasLogged reads the HasLogged flag for a cache class under the
// package's own lock, exactly as production code must.
func cacheHasLogged(t *testing.T, id int) bool {
	t.Helper()

	cacheLock.RLock()
	defer cacheLock.RUnlock()

	return cacheList[id].HasLogged
}
