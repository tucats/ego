# BUG-29 — Closing an already-closed channel crashes the entire process

**Severity:** CRITICAL

**Description:**  
Calling `close()` a second time on the same channel panics with Go's native
`"panic: close of closed channel"` and crashes the whole `ego` process; the panic is not
caught by `try`/`catch`. This is inconsistent with `Send()` on a closed channel, which is
already handled gracefully and returns a catchable "channel not open" error.

**Reproducer:**

```go
func main() {
    ch := make(chan, 2)
    close(ch)
    try {
        close(ch)
        fmt.Println("double close did not error")
    } catch (e) {
        fmt.Println("double close caught:", e)
    }
    fmt.Println("done")
}
```

**Actual output:**

```text
panic: close of closed channel

goroutine 1 [running]:
github.com/tucats/ego/internal/language/data.(*Channel).Close(...)
    /Users/tom/go/src/github.com/tucats/ego/internal/language/data/channel.go:184 +0xf4
github.com/tucats/ego/internal/builtins.Close(...)
    /Users/tom/go/src/github.com/tucats/ego/internal/builtins/close.go:16 +0x78
...
```

(exit code 2; nothing after the first `close(ch)` ever prints)

**Expected output:**

```text
double close caught: <catchable "channel not open"-style error>
done
```

**Notes:**  
Root cause: `internal/language/data/channel.go`, `Channel.Close()` (~line 168-187)
unconditionally calls Go's native `close(c.channel)` before checking `c.isOpen`; it only
computes `wasActive` *after* the unconditional close. `Send()` in the same file is protected
by `defer recover()` (per CLAUDE.md's documented note on `data.Channel`), but `Close()` has
no equivalent guard.

**Resolution (July 2026):**  
Applied the same fix pattern the Notes above point at for `Send()`: check the channel's
state before ever touching the native channel, rather than trying to recover afterward.

- **`internal/language/data/channel.go` — `Channel.Close()` signature changed** from
  `func() bool` to `func() (wasOpen bool, err error)`. The method now checks `c.isOpen`
  *before* calling the native `close()` (both under the same exclusive `c.mutex.Lock()` that
  was already held for the whole method, so there's no race window between the check and the
  close). If the channel is already closed, it returns `false, errors.ErrChannelNotOpen`
  (the same error `Send()` already used for a closed channel) instead of calling `close()`
  again — so the panic can no longer happen at all.
- **`internal/builtins/close.go` — `Close()`** now builds a `data.NewList(wasOpen, err)` for
  the channel case, following the documented multi-return convention (CLAUDE.md's
  `callRuntimeFunction` dispatch mechanics section), which is what makes the error catchable
  via `try`/`catch` rather than an uncatchable abort.
- **`internal/builtins/functions.go`** — the `"close"` builtin's `Declaration.Returns` was
  updated to `[]*data.Type{data.BoolType, data.ErrorType}` to match the new two-value
  contract. This is backward compatible: every existing call site in the codebase and test
  suite uses `close(ch)` as a bare statement or inside `defer`, both of which are unaffected
  by adding return values; the two-value form `wasOpen, err := close(ch)` now also works.

**Tests added:**

- `internal/language/data/channel_test.go`: `Test_Channel_Close_FirstCallSucceeds`,
  `Test_Channel_Close_NilReceiver`, and the direct regression test
  `Test_Channel_Close_DoubleCloseDoesNotPanic` (wraps the second `Close()` call in its own
  `recover()` so a regression fails the test cleanly instead of crashing the whole `go test`
  run).
- `internal/builtins/close_test.go`: updated the existing tests for the new `data.List`
  return shape and added `Test_Close_DoubleCloseReturnsError`, the same regression test one
  layer up through the `close()` builtin wrapper.
- `tests/defer/channel.ego` — `"defer: double close is catchable, not a crash"` is the
  end-to-end Ego-level regression test for the original repro above;
  `"defer: closing a channel returns wasOpen and no error"` guards the new two-value return
  form.

