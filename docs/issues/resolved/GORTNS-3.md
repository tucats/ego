# GORTNS-3 — REST progress goroutine outlived its request and could print stale messages

**Affected function:** `Exchange`
**File:** `runtime/rest/exchange.go`
**Risk:** Low — a goroutine lingering up to three seconds past its request, and an
occasional misleading "still waiting" message
**Status: RESOLVED**

## GORTNS-3: Description

`Exchange` starts a helper goroutine that reports progress if a REST call is slow,
and stops it by clearing a boolean:

```go
stillWaiting.Store(true)

if v, found := symbols.RootSymbolTable.Get(defs.UserCodeRunningVariable); found && !data.BoolOrFalse(v) {
    go func() {
        time.Sleep(1 * time.Second)

        for stillWaiting.Load() {
            ui.Say(i18n.M("rest.waiting", map[string]any{"URL": url}))
            time.Sleep(3 * time.Second)
        }
    }()
}

defer func() {
    stillWaiting.Store(false)
}()
```

Two problems, both caused by polling a flag from inside `time.Sleep`.

**A sleeping goroutine cannot notice anything.** When the request finished and the
flag was cleared, this goroutine was parked in the middle of a three-second sleep
and could not react until it woke up on its own. So it outlived the function that
started it by up to three seconds on every slow call.

**Checking a flag and then printing are two separate steps.** If the request
completed in the window between `stillWaiting.Load()` returning true and `ui.Say`
running, the user was told the client was still waiting for a request that had
already come back.

Neither is a permanent leak — the goroutine always terminated, and its lifetime
was bounded — which is why this is rated low.

### Correction to the original audit

The audit that produced this issue reported that `stillWaiting` was a
package-level variable shared across concurrent REST calls, so that the first call
to finish would silence the progress messages of all the others. **That was
wrong.** `stillWaiting` is declared inside `Exchange`:

```go
func Exchange(...) error {
    var (
        restResponse *resty.Response
        err          error
        stillWaiting atomic.Bool
    )
```

Each call therefore has its own flag and there is no cross-call interference. The
real defects are the two above, and the severity is correspondingly lower than
first reported.

## GORTNS-3: Fix

The flag is replaced with a done channel, and both waits become a `select` over
that channel and a timer:

```go
done := make(chan struct{})

defer close(done)

if ... {
    go func() {
        // Stay quiet for the first second.
        select {
        case <-done:
            return
        case <-time.After(1 * time.Second):
        }

        for {
            ui.Say(i18n.M("rest.waiting", map[string]any{"URL": url}))

            select {
            case <-done:
                return
            case <-time.After(3 * time.Second):
            }
        }
    }()
}
```

`select` over a stop channel and a timer channel is the Go idiom for "sleep, but
wake up early if something happens": whichever case becomes ready first is chosen.
The goroutine now stops the moment the request finishes rather than noticing later,
and it cannot print after that point, because the `done` case returns instead of
looping back to the message.

As elsewhere in this set of fixes, `done` is closed rather than written to, because
closing a channel releases every receiver immediately and does not require the
sender to know whether anyone is currently listening.

The now-unused `stillWaiting` variable and the `sync/atomic` import were removed.
