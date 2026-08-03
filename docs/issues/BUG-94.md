# BUG-94 — A `go` statement's captured-scope sharing is established too late, racing against the launching goroutine's own continued execution

**Severity:** MEDIUM

**Discovered by:** `go test -race` during work on PERFORMANCE.md Finding 17 (the global-reference cache, `docs/internals/GLOBALS.md`); confirmed present on `master` before that work began (byte-for-byte-identical stack trace), so this is a pre-existing bug, not a regression from it. Investigated and fixed at the user's request in a dedicated follow-up pass.

**Status: FIXED**

**Description:**  
Running the full `ego test tests/` suite under `go test -race` (or an `-race`-instrumented `ego`
binary) reliably surfaced exactly one data race, isolated down to a single existing test file
(`tests/flow/go_func_literal.ego`) run entirely on its own:

```text
WARNING: DATA RACE
Read at 0x... by goroutine N:
  runtime.mapaccess1_faststr()
  symbols.(*SymbolTable).GetAnyScope()          get.go:23
  symbols.(*SymbolTable).GetAnyScope()          get.go:33   (2 levels of recursion)
  bytecode.argCheckByteCode()                   argcheck.go:79
  bytecode.(*Context).RunFromAddress()
  bytecode.(*Context).Run()
  bytecode.GoRoutine()                          goroutine.go:170
  bytecode.goByteCode.gowrap1()                 goroutine.go:63

Previous write at 0x... by main goroutine:
  runtime.mapaccess2_faststr()
  symbols.(*SymbolTable).Create()               create.go:43
  bytecode.(*Context).create()
  bytecode.storeChanByteCode()                  store.go:293
  ...
```

The minimal reproducer is the first `@test` block in that file:

```ego
@test "flow: function literal as go-routine"
{
    var ch chan

    go func (c chan, str string) {
        c <- strings.ToLower(str)
    }(ch, "HellO")

    x <- ch                       // auto-creates "x" via storeChanByteCode
    @assert T.Equal(x, "hello")
}
```

Both sides of the race are entirely ordinary, already-existing code: launching a closure as a
goroutine, and receiving from a channel into a not-yet-declared variable (which
`storeChanByteCode` creates automatically). The test's own assertions always passed — this was
purely a synchronization bug invisible without a race detector, not a functional/correctness bug.

**Root cause:** launching a goroutine for a closure (`go func() {...}()`) requires marking the
closure's *captured* scope (the enclosing local scope it reads/writes) as shared, so that both the
new goroutine and the thread that launched it access that scope's symbol table safely
(`SymbolTable.Shared(true)`, which causes `Get`/`Set`/`Create` to take a read/write lock). Before
this fix, that marking happened as the very first thing `GoRoutine` did — but `GoRoutine` only
starts running *inside the new goroutine*, launched via a bare `go GoRoutine(...)` in `goByteCode`
(the code that runs the `go` statement in the launching thread). `goByteCode` does not wait for
`GoRoutine` to reach that marking step before returning — it launches the goroutine and
immediately continues to whatever statement comes next.

Go gives no ordering guarantee between "a newly spawned goroutine actually starts running" and
"the goroutine that spawned it continues past the `go` statement." So there was a genuine race
window: the launching thread could reach a *later* statement that touches the very same captured
scope (here, `x <- ch`, which auto-creates `x` in that same local scope) *before* the new goroutine
got around to marking that scope shared. Both sides individually check the shared flag correctly
before deciding whether to lock — but the write in the launching thread ran while the flag was
still `false` (unlocked), while the read in the new goroutine, moments later, saw the
now-`true` flag and took a lock. One side's access was never actually guarded, which is exactly
what a data race is, regardless of the other side behaving correctly from its own point of view.

Named-function goroutines (`go someFunc(args)`, as opposed to a closure literal) were not affected
by this specific race: they don't have a captured scope to mark at all, and instead rely on
whichever ancestor scope is already marked shared from program start (ultimately the process-wide
root symbol table, always shared since an `init()` function marks it so). There is no "newly
becomes shared" transition for that path, so no equivalent race window exists there.

**Fix:** moved the captured-scope-sharing step out of `GoRoutine` (which runs inside the new
goroutine, after the race window has already opened) and into `goByteCode` (which runs in the
launching thread, *before* the `go GoRoutine(...)` statement that starts the new goroutine at
all):

```go
if fx, err := c.Pop(); err != nil {
    return err
} else {
    if bc, ok := fx.(*ByteCode); ok && bc.IsLiteral() {
        if captured := bc.GetCapturedScope(); captured != nil {
            captured.Shared(true)
        }
    }

    goRoutineCompletion.Add(1)
    go GoRoutine(fx, c, data.NewList(args...))

    return nil
}
```

Since this now runs synchronously, before any concurrency exists for this specific closure/scope,
no additional locking is needed for the marking step itself — by the time either the launching
thread continues or the new goroutine starts, the scope is already correctly marked shared, with
no window in between.

**Files modified:**

- `internal/language/bytecode/goroutine.go` — moved the closure-captured-scope `Shared(true)` call
  from `GoRoutine` to `goByteCode`; updated both functions' doc comments to describe the new
  timing and why the old placement was racy.

**Tests added:**

- `internal/language/bytecode/goroutine_test.go` —
  `Test_GoByteCode_MarksCapturedScopeSharedBeforeLaunching` calls `goByteCode` directly (the real
  entry point for a `go` statement) and asserts the captured scope is already marked shared the
  instant `goByteCode` returns, before the spawned goroutine has necessarily run at all. Verified
  this test fails against the pre-fix code (confirmed by temporarily reverting the fix and
  re-running it) and passes against the fix.

**Verification:** Bisected the full `ego test tests/` suite under a race-instrumented binary down
to the single responsible file, confirmed the race reproduced reliably (5/5 runs) before the fix
and did not reproduce at all (0/5+ runs, including several full-suite runs) after it. `go build
./...`, `go vet ./...`, `go test ./...` (including `go test -race ./...` across the whole
repository) all clean. The full `ego test tests/` suite (1,709 cases, unchanged count — this was a
synchronization fix, not a behavior change) passes, run repeatedly under `-race` with zero data
races reported, down from a consistent one data race before the fix.
