# BUG-28 — Double-`Unlock()` of a `sync.Mutex` crashes the entire process

**Severity:** CRITICAL

**Description:**  
Calling `Unlock()` on a `sync.Mutex` that is not currently locked triggers Go's
`internal/sync.fatal()` path (`fatal error: sync: unlock of unlocked mutex`), which is not
interceptable even by a Go-level `recover()`, let alone Ego's `try`/`catch`. The entire
`ego` process crashes.

**Reproducer:**

```go
import "sync"

func main() {
    try {
        var mu sync.Mutex
        mu.Lock()
        mu.Unlock()
        mu.Unlock()
    } catch (e) {
        fmt.Println("caught:", e)
    }
    fmt.Println("survived")
}
```

**Actual output:**

```text
fatal error: sync: unlock of unlocked mutex

goroutine 1 [running]:
internal/sync.fatal(...)
    .../runtime/panic.go:1191 +0x20
internal/sync.(*Mutex).unlockSlow(...)
    .../internal/sync/mutex.go:204 +0x38
```

(exit code 2; neither "caught" nor "survived" is ever printed)

**Expected output:**

```text
caught: <some Ego error describing the misuse>
survived
```

**Notes:**  
Same root-cause class as BUG-27 (unguarded native pass-through). This case is strictly
worse: Go's `sync.Mutex.Unlock` calls `fatal()` directly rather than `panic()`, so no amount
of wrapping inside Ego (nor even a Go-level `defer recover()`) can intercept it — the
runtime function must guard against the misuse explicitly (e.g. tracking lock state) before
ever calling into the native `sync.Mutex`.

**Resolution (July 2026):**  
As predicted in the Notes above, BUG-27's general `recover()`-based safety net (see its own
resolution) cannot help here, because Go's `fatal()` path bypasses `recover()` entirely by
design. The fix instead prevents the risky call from ever happening:

- **`internal/language/bytecode/callNative.go` — new `mutexLockState` + `callMutexMethod`.**
  Ego represents a `sync.Mutex` variable as a bare `*sync.Mutex` with no extra field where an
  "am I locked?" flag could live (see `SetNew` in `internal/runtime/sync/types.go`), so a
  package-level `sync.Map` (`mutexLockState`) supplies that missing bookkeeping externally,
  keyed by the `*sync.Mutex` pointer itself. `callMutexMethod` intercepts `Lock`, `Unlock`,
  and `TryLock` calls on a `*sync.Mutex` receiver *before* `CallWithReceiver`'s generic
  reflection-based dispatch: `Lock`/`TryLock` update the tracked state as usual, and `Unlock`
  checks the tracked state first — if the mutex isn't marked locked, the real `Unlock()` is
  never called at all, so the fatal error can never be triggered.
- **New error `ErrMutexNotLocked`** (key `mutex.not.locked`), added to
  `internal/errors/messages.go` and localized in all three `messages_*.txt` files, is
  returned instead — an ordinary, catchable Ego error.
- `sync.RWMutex` is registered internally (`SyncRWMutexType` in
  `internal/runtime/sync/types.go`) but is not actually exposed to Ego code (it's absent from
  `SyncPackage`'s member map) — this fix is therefore scoped to `sync.Mutex`, the only type
  reachable from Ego code that has this failure mode today.

**Tests added:**

- `internal/language/bytecode/callNative_test.go` (Section 11): six tests covering normal
  Lock/Unlock, TryLock while unlocked/locked, an unhandled-method-name fallback, and two
  direct regression tests — `Test_callMutexMethod_UnlockWithoutLock` and
  `Test_callMutexMethod_DoubleUnlockReturnsErrorNotFatal` (the exact repro from this issue,
  exercised through `callMutexMethod` and again through the public `CallWithReceiver` entry
  point in `Test_CallWithReceiver_MutexDoubleUnlock`).
- `tests/packages/sync.ego` — `"sync: Mutex double Unlock is catchable, not a crash"` is the
  end-to-end Ego-level regression test; `"sync: Mutex normal lock and unlock cycle"`,
  `"sync: Mutex TryLock reports availability correctly"`, and `"sync: independent Mutex
  values track lock state separately"` guard against regressions in ordinary usage and in
  the per-instance bookkeeping.

