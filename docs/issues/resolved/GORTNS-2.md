# GORTNS-2 — Expired debug sessions leaked their goroutine; `debugger.Close` had no caller

**Affected functions:** debug session lifecycle in `admin/run.go`, `Delete` and
`expire` in the `caches` package
**Files:** `server/admin/run.go`, `caches/cache.go`, `caches/delete.go`
**New API:** `caches.SetOnEvict`
**Risk:** Medium — one leaked goroutine (plus its bytecode and symbol table) per
abandoned debug session, accumulating without bound
**Status: RESOLVED**

## GORTNS-2: Description

An API-mode debug session is not plain data. `debugger.Resume` starts a goroutine
on the first call and leaves it parked on a channel waiting for the next command
from the REST client:

```go
go func() {
    err := runWithSession(c, sessionContext)
    ...
}()
```

The session is tracked in a cache with a 15-minute inactivity expiry:

```go
caches.SetExpiration(caches.DebugSessionCache, "15m")
...
caches.Add(caches.DebugSessionCache, uuid, &debugSession{owner: user, ctx: ctx})
```

When a client abandoned a session — disconnected, closed the browser tab, or just
stopped sending commands — the cache expiry dropped the map entry and nothing
else happened. The goroutine stayed blocked on its input channel forever, holding
its `*Context`, compiled bytecode, symbol table, and captured program output.

`debugger.Close` exists for precisely this teardown, and its doc comment even
names the scenario:

```go
// Close tears down an API-mode debug session that is no longer needed — for
// example, when a REST client disconnects mid-session.
```

It had **no callers anywhere in the tree**.

The `maxDebugSessions = 20` cap did not contain this. The cap counts entries in
the cache, and expiry *frees* an entry, so an admin could keep starting new
sessions indefinitely — each expiring out of the cache after 15 minutes and
leaving a permanently parked goroutine behind — without ever hitting the limit.

## GORTNS-2: Fix

The root cause is structural: the cache had no way to tell an owner that a value
was being discarded. Purge had a hook (`OnPurge`) but individual item removal did
not, so a value holding a resource had no opportunity to release it.

A per-item eviction callback was added, registered through a setter:

```go
func SetOnEvict(handler func(id int, key any, value any))
```

It fires for **both** removal paths — expiry (`sweepExpired`) and explicit
`Delete` — with two guarantees the documentation states and the tests enforce:

- The callback runs **after** the cache lock is released, so an implementation may
  call back into the cache package without deadlocking. Both call sites collect
  the evicted entries while locked and deliver the notifications afterwards.
- It runs synchronously on whichever goroutine performed the eviction, so an
  implementation must return promptly.

The admin package registers a handler that closes debug sessions:

```go
caches.SetOnEvict(releaseEvictedCacheItem)
```

`releaseEvictedCacheItem` filters for `DebugSessionCache`, uses the two-value form
of the type assertion (the handler sees evictions from every cache, so an
unexpected type is a normal condition rather than a reason to panic), and calls
`debugger.Close`.

### Why a setter rather than an exported variable

The first version of this fix exposed a plain `var OnEvict func(...)`, matching the
existing `OnPurge`. The race detector rejected it, correctly: the callback is read
by the background expiration goroutines, so it is a global that two goroutines
touch. In production the handler is installed from `init` long before any request
arrives, so an unsynchronized read would happen to work — but the compiler is
entitled to assume no other goroutine writes it, and "happens to work" is not a
guarantee. `SetOnEvict` and an internal `evictHandler()` accessor put an `RWMutex`
around both sides.

Note that the pre-existing `OnPurge` variable has the same shape and was left
alone; it is written once at startup and reading it is not part of this issue.

### Refactor

The body of the expiration loop was extracted from `expire` into `sweepExpired`,
which returns false when the cache no longer exists. This keeps the lock handling
readable in one place and lets the tests drive a single deterministic sweep instead
of waiting out a real scan interval.

## GORTNS-2: Tests

`caches/evict_test.go`:

- `TestDeleteNotifiesOnEvict_GORTNS2` — the explicit-delete path, including that
  deleting a key that is not present reports nothing.
- `TestExpirationNotifiesOnEvict_GORTNS2` — the path that caused the leak. It
  back-dates an item's expiration and drives one sweep directly.
- `TestOnEvictRunsUnlocked_GORTNS2` — calls back into the cache from inside the
  callback. If the lock were still held this would deadlock and the test would
  time out rather than fail, which is the point.

All three pass under `go test -race`.
