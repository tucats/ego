# GORTNS-4 — Long-lived server tasks had no way to stop and no guard against duplicate launches

**Affected functions:** `LogMemoryStatistics`, `LogRequestCounts`,
`StartHealthChecker`, `OpenLogFile`
**Files:** `router/logging.go`, `router/stats.go`, `server/cluster/health.go`,
`cli/ui/logfile.go`, `commands/server.go`
**Risk:** Low — no live defect; abrupt shutdown, and an unguarded path to
duplicate tasks
**Status: RESOLVED**

## GORTNS-4: Description

Four background tasks were bare infinite loops with no exit:

```go
// router/logging.go
for {
    ...
    time.Sleep(duration)          // default 5 minutes
}

// router/stats.go
for {
    time.Sleep(logRequestCounterDuration * time.Second)   // 60 seconds
    ...
}

// server/cluster/health.go
for {
    time.Sleep(interval)          // default 30 seconds
    ...
}

// cli/ui/logfile.go
for {
    ...
    time.Sleep(sleepUntil)        // until just after midnight
}
```

None of them leaked in the sense GORTNS-1 did — each is launched exactly once per
process, so nothing accumulates, and `ego restart server` forks a fresh process
rather than restarting in place. Two things were nonetheless wrong.

**Shutdown was abrupt.** The SIGINT handler in `commands/server.go` calls
`os.Exit(0)`, so these tasks were terminated mid-sleep by the operating system.
That works, but it means a task could be pinging cluster peers on behalf of a
server that is on its way out, or logging statistics for it, with no chance to
stop first.

**Nothing prevented a duplicate launch.** The three tasks in `commands/server.go`
have no guard whatsoever, and neither did the log rollover task:

```go
if withTimeStamp {
    PurgeLogs()

    go rollOverTask()
}
```

A second call to `OpenLogFile` with `withTimeStamp` set — from a test, or from any
future code that reopens the log — would start a second identical rollover task,
and both would try to roll the same log over at midnight.

## GORTNS-4: Fix

### Stop channels

The three server tasks now take a stop channel and wait on it rather than sleeping
blindly. A shared helper in `router/logging.go` expresses the pattern once:

```go
func sleepOrStop(duration time.Duration, stop <-chan struct{}) bool {
    timer := time.NewTimer(duration)
    defer timer.Stop()

    select {
    case <-stop:
        return false
    case <-timer.C:
        return true
    }
}
```

It returns true when the interval elapsed (keep looping) and false when the stop
signal arrived (return). `time.NewTimer` is used rather than `time.After` so the
timer is released on the stop path instead of being left to fire into a channel
nobody reads.

Passing a nil stop channel is legal and means "never stop", because a receive on a
nil channel blocks forever and so can never be selected — the timer case always
wins.

`StartHealthChecker` uses the same construction inline. It benefits most: with a
30-second default ping interval, a plain `Sleep` could keep the process alive for
up to half a minute after shutdown began.

### A real caller for the stop signal

`commands/server.go` owns one channel for all three tasks and closes it from the
SIGINT handler, before the cluster membership teardown:

```go
serverTasksStopped := make(chan struct{})

go router.LogMemoryStatistics(serverTasksStopped)
go router.LogRequestCounts(serverTasksStopped)
go cluster.StartHealthChecker(serverTasksStopped)
...
// in the SIGINT handler:
close(serverTasksStopped)
```

Closing releases all three at once. This matters: without a caller the stop
channel would be dead code exercised only by tests.

### Duplicate-launch guard

The rollover task is now started through a `sync.Once`:

```go
rollOverTaskOnce.Do(func() {
    go rollOverTask()
})
```

`sync.Once` is the Go tool for "at most once per process, no matter how many
callers ask or how many ask concurrently". `Do` runs the function the first time
and is a no-op afterwards, and it is safe under concurrent callers, so no separate
mutex is needed.

## GORTNS-4: Tests

`router/background_task_test.go`. The failure mode being tested is "never returns",
so each test asserts on *termination*, observed through a channel the launched
goroutine closes on its way out, with a timeout to catch a hang:

- `TestLogMemoryStatisticsStops_GORTNS4` and `TestLogRequestCountsStops_GORTNS4` —
  launch the task, close the stop channel, require a return within five seconds.
  Against the unfixed code the first fails with
  `LogMemoryStatistics did not return after its stop channel was closed`.
- `TestSleepOrStopReportsFullInterval_GORTNS4` — the keep-looping result. Getting
  this backwards would make every task exit after its first tick.
- `TestSleepOrStopReportsStop_GORTNS4` — the cancel result, using a one-hour
  interval so a `time.Sleep` implementation would time out rather than merely fail,
  and asserting the return is prompt.
- `TestSleepOrStopWithNilChannel_GORTNS4` — documents the "never cancel" case.
