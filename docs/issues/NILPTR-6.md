# NILPTR-6 — Background goroutines had no panic recovery, so one bad iteration killed the process

**Affected functions:** `startRateLimitScan`, the transaction cleanup loop in
`Begin`, the OAuth state-purge loop in `Initialize`
**Files:** `router/ratelimit.go`, `server/tables/transactions.go`,
`server/oauth/oauth.go`
**New helper:** `util.SafeCall` in `util/safecall.go`
**Risk:** High — an unrecovered panic in any of these terminates the entire
server process
**Status: RESOLVED**

## NILPTR-6: Description

The server starts three long-lived goroutines that run periodic maintenance:

```go
// router/ratelimit.go
go func() {
    for {
        time.Sleep(rateLimitScanInterval)
        pruneLoginAttempts()
    }
}()

// server/tables/transactions.go
go func() {
    for {
        time.Sleep(time.Second * 60)
        cleanupExpiredTransactions()
    }
}()

// server/oauth/oauth.go
go func() {
    ticker := time.NewTicker(statePurgeInterval)
    defer ticker.Stop()

    for range ticker.C {
        purgeExpiredStates()
    }
}()
```

None had a `recover()`.

This is the most dangerous place in the server to panic, and the reason is a Go
detail that is easy to miss. `net/http` recovers panics — but only in the
goroutine it created to serve a request. A panic in any *other* goroutine has
nothing above it to catch it, and Go's rule for an unrecovered panic is to
terminate the whole program.

So the practical severity is inverted from intuition. A nil dereference in a
request handler costs one request (and, after NILPTR-1, produces a clean 500). The
same nil dereference in a once-a-minute cleanup task kills the server outright,
dropping every healthy connection, with no response to anyone and no chance to
retry.

These particular tasks are plausible places for it to happen: the transaction
cleanup walks a map of transactions whose database handles may already have been
closed (see NILPTR-4), and the rate-limit scan walks a map of login records.

## NILPTR-6: Fix

A shared helper, `util.SafeCall`, wraps one call and converts a panic into a log
entry, returning whether the call completed:

```go
func SafeCall(name string, fn func()) (completed bool) {
    defer func() {
        panicValue := recover()
        if panicValue == nil {
            return
        }

        completed = false

        if !PanicRecoveryEnabled() {
            panic(panicValue)
        }

        ui.Log(ui.ServerLogger, "server.panic.task", ...)
        ui.Log(ui.InternalLogger, "server.panic.stack", ...)
    }()

    fn()

    return true
}
```

It wraps the **body** of each loop, not the loop itself, so a bad iteration is
logged and skipped and the loop lives to run again:

```go
for {
    time.Sleep(rateLimitScanInterval)
    util.SafeCall("prune login attempts", pruneLoginAttempts)
}
```

Note that `recover()` is called directly by the deferred closure here. That is
required — Go ignores `recover()` when it is reached through an extra call frame.
See NILPTR-1, where the first attempt at the request handler got this wrong.

The four other `go func()` blocks in the server were reviewed and left alone: each
one sleeps, logs, and calls `os.Exit(0)` as part of a deliberate shutdown, so
there is nothing to protect.

## NILPTR-6: Configuration

`util.PanicRecoveryEnabled()` centralizes the reading of
`ego.server.panic.recovery` so the request path and the background tasks cannot
disagree. It deliberately does **not** use `settings.GetBool` directly:

```go
func PanicRecoveryEnabled() bool {
    if !settings.Exists(defs.ServerPanicRecoverySetting) {
        return true
    }

    return settings.GetBool(defs.ServerPanicRecoverySetting)
}
```

`GetBool` returns false for a key that was never configured, because an unset key
reads as an empty string. Using it directly would mean every configuration file
written before this setting existed — which is all of them — reported false and
ran with recovery switched **off**, silently doing the opposite of the documented
default. Checking `Exists` first distinguishes "explicitly set to false" from "not
mentioned at all".

## NILPTR-6: Tests

In `util/safecall_test.go`:

- `TestPanicRecoveryEnabledDefaultsOn_NILPTR6` — the absent-key case above.
- `TestPanicRecoveryEnabledRespectsExplicitFalse_NILPTR6`
- `TestSafeCallRecoversPanic_NILPTR6` / `TestSafeCallReportsSuccess_NILPTR6`
- `TestSafeCallLoopSurvivesPanic_NILPTR6` — the behavior the goroutines depend on:
  a loop that panics on alternating iterations still completes all four.
- `TestSafeCallPropagatesWhenDisabled_NILPTR6`
