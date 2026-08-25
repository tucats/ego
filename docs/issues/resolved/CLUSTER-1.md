# CLUSTER-1 — Cache-invalidation broadcast fed back on itself without bound

**Affected functions:** `FlushCacheHandler`, `BroadcastCacheFlush`,
`SendCacheFlush`, `caches.Purge`
**Files:** `server/cluster/handlers.go`, `server/cluster/invalidate.go`,
`caches/purge.go`, `defs/cluster.go`
**New API:** `caches.PurgeLocal`
**Risk:** High in any cluster of two or more active nodes — an unbounded request
storm plus permanently empty caches. No effect on standalone servers.
**Status: RESOLVED**

## CLUSTER-1: Description

Cluster nodes tell each other to drop a stale cache through a hook the `caches`
package fires whenever a cache is purged:

```go
// caches/purge.go
if OnPurge != nil {
    go OnPurge(id)
}
```

`server/cluster` installs `BroadcastCacheFlush` as that hook when a node joins a
cluster, which POSTs `/services/cluster/flush` to every active peer. The receiving
handler then purged the named cache:

```go
// server/cluster/handlers.go
cacheID := req.CacheID
caches.Purge(cacheID)
```

`caches.Purge` is the same public function that fires `OnPurge`. So handling an
inbound notification produced a new outbound one:

```text
Node A: caches.Purge(X)
     -> OnPurge -> POST /services/cluster/flush to every peer
Node B: FlushCacheHandler
     -> caches.Purge(X)          <-- fires OnPurge again
     -> POST back to A (and to C, D, ...)
Node A: FlushCacheHandler -> caches.Purge(X) -> ... forever
```

### Nothing damped it

Every candidate for a brake was checked and none existed:

- **`SenderID` was decoded but never acted on.** It went only into a log line;
  its own doc comment in `defs/cluster.go` said "Used for logging only."
- **No self-damping.** `OnPurge` fires outside the `if cache, found := ...` check,
  so purging an already-deleted cache broadcasts identically. "Nothing left to
  purge" did not end the cascade.
- **No hop count, TTL, or de-duplication nonce** in `ClusterFlushRequest`.
- **No replay protection** in `ValidateClusterToken` — a static HMAC compare.
- **No backpressure**: `go OnPurge(id)` is fire-and-forget, so a node did not even
  wait for its own sends to complete.

### What limited the blast radius

- `ListActiveMembers` excludes the local node (`m.NodeID != NodeID`), so a
  single-node cluster had no peer to notify and could not loop.
- The hook is only installed inside cluster `Join`, and `BroadcastCacheFlush`
  early-returns when `ClusterName` is empty, so standalone servers were unaffected.

### Severity scaled with node count

Each received flush triggered (peers-1) new sends, so traffic multiplied by
(peers-1) every round:

| Nodes | Behavior |
| :---- | :------- |
| 1 | No loop; no peers to notify |
| 2 | Steady ping-pong at HTTP speed, indefinitely |
| 3 | ×2 per round — exponential |
| 4 | ×3 per round |

Every round also purged the cache on every node, so beyond the request storm the
affected caches stayed permanently empty and every lookup fell through to the
system database.

The trigger surface was ordinary operation, not an edge case: `caches.Purge` is
called by the admin cache-flush endpoint, by token blacklist add/remove, and by
DDL through `@sql` and the scripting endpoint. `DELETE /admin/caches` with no
`class` parameter was the worst starter, because `PurgeAll` walks every cache class
and so kicked off one independent cascade per class at once.

## CLUSTER-1: Fix

### 1. A non-broadcasting apply path (the actual fix)

`caches` now has two entry points, and the difference is which one the caller
picks:

```go
// Purge discards the cache AND fires OnPurge. For a purge that originates here.
func Purge(id int) { purge(id, true) }

// PurgeLocal discards the cache and notifies nobody. For a purge that arrived
// from a peer.
func PurgeLocal(id int) { purge(id, false) }
```

`FlushCacheHandler` calls `PurgeLocal`. That single line breaks the cycle
structurally: a received notification is terminal, so there is no path from
"inbound flush" back to "outbound flush" for a loop to travel.

Encoding the rule as two functions rather than a runtime check means the
terminating case is chosen at the call site, where it is visible, instead of
depending on a condition a later edit could quietly drop.

**Deliberately not changed:** `OnPurge` still fires even when the cache did not
exist locally. An earlier draft of this fix proposed moving it inside the `found`
check, which would have been wrong — a peer may have a cache populated that this
node never built, so it still needs to be told. Restricting the broadcast would
have traded a loop for silently missed invalidations.

### 2. A hop-count circuit breaker (defense in depth)

`ClusterFlushRequest` gained a `Hops` field, and the cluster package a limit:

```go
const maxFlushHops = 4
const originHopCount = 1
```

`BroadcastCacheFlush` originates notifications at hop 1, `SendCacheFlush` takes
the count as a parameter, and `FlushCacheHandler` drops any request whose count
exceeds the limit.

This is explicitly **not** the mechanism that prevents the loop, and with fix 1 in
place it never fires — a notification is never relayed, so every legitimate request
arrives at hop 1. It exists because fix 1 is one call site away from being undone:
if a future change made the receive path broadcast again, hop counts would climb
and the cluster would go quiet after `maxFlushHops` rounds, leaving a clear trail
of log warnings, instead of storming without limit.

The contract this depends on is documented on `SendCacheFlush`: any code that
forwards a notification it received **must** pass the incoming count plus one.

The field is `omitempty`, so a peer running an older build sends no `hops` key,
which decodes as 0 and is accepted as a first-hop message rather than rejected.

## CLUSTER-1: Tests

`server/cluster/invalidate_loop_test.go`, driving the real `FlushCacheHandler`:

- `TestInboundFlushDoesNotRebroadcast_CLUSTER1` — the test that matters. Handling
  an inbound flush must fire the broadcast hook zero times. Reverting the handler
  to `caches.Purge` fails it with `handling an inbound flush fired the broadcast
  hook 1 time(s), want 0; the notification loop is still present`. It also asserts
  the cache really was purged, so breaking the loop cannot be mistaken for
  breaking invalidation.
- `TestLocallyOriginatedPurgeStillBroadcasts_CLUSTER1` — the other half of the
  contract. It would be easy to "fix" the loop by suppressing the broadcast
  everywhere, which would disable cluster invalidation entirely.
- `TestPurgeLocalNeverBroadcasts_CLUSTER1` — covers the new function directly.
- `TestFlushBeyondHopLimitIsDropped_CLUSTER1` and
  `TestFlushAtHopLimitIsAccepted_CLUSTER1` — the circuit breaker and its exact
  boundary, so the comparison cannot drift into an off-by-one that discards
  legitimate notifications.
- `TestFlushWithNoHopsFieldIsAccepted_CLUSTER1` — compatibility with an older peer
  that sends no `hops` field.

Asserting "the hook fired zero times" needs care, because the hook runs on its own
goroutine (`go OnPurge(id)`): a test that read a counter immediately could pass
just by racing ahead of it. The tests use a buffered channel drained until it has
been quiet for 250ms, which both gives the hook time to appear and keeps the test
clean under `go test -race`.

## CLUSTER-1: Remaining recommendation

`caches.OnPurge` is still an exported plain `var` read by the goroutine that
`Purge` spawns. That is the same unsynchronized-global shape that `OnEvict` had
before GORTNS-2 replaced it with `SetOnEvict` and an `RWMutex`. It works today
because the hook is assigned once during cluster join, before request traffic
begins, but it is not a guarantee the compiler owes us. Giving `OnPurge` the same
setter treatment is a small, self-contained follow-up, left out of this change to
keep it focused on the loop.
