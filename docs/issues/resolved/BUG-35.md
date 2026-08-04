# BUG-35 — An error raised inside a `catch` block escapes all enclosing `try` blocks

**Severity:** HIGH

**Description:**  
When a `try`/`catch` is nested inside another `try`, and the *inner* `catch` block itself
raises a new error, that error is not caught by the *outer* `try`'s `catch`, even though the
inner catch block is lexically inside the outer `try{}`'s braces. The error escapes the
entire construct uncaught, aborting the program. This is analogous to Go's
`defer`+`recover()`, where a panic raised while a recover-handler itself is executing still
unwinds into the next outer recover — Ego's `try`/`catch` should behave the same way per
`docs/LANGUAGE.md` lines 2211-2214.

**Reproducer:**

```go
func main() {
    try {
        try {
            x := 0
            _ = 5 / x
        } catch {
            y := 0
            _ = 10 / y
        }
    } catch (outer) {
        fmt.Println("outer caught:", outer)
    }
    fmt.Println("done")
}
```

**Actual output:**

```text
Error: at main(line 8), division by zero
Error: terminated with errors
```

(exit code 1; neither "outer caught" nor "done" is ever printed)

**Expected output:**

```text
outer caught: division by zero
done
```

**Notes:**  
Root cause: `internal/language/bytecode/catch.go:100` (`handleCatch`) only ever inspects
`c.tryStack[len(c.tryStack)-1]`. On catching an error it zeroes that top entry's `.addr`
("so recursive errors don't occur") but does not pop it off the stack — `TryPop` is emitted
by the compiler only once, after the *entire* catch block finishes
(`internal/language/compiler/try.go:66`). So while the inner catch block executes, the
top-of-stack entry is still the same try's now-inert (`addr == 0`) entry, and `handleCatch`
never falls through to check the entry below it (the outer try). The condition only ever
looks at index `len-1`, so a live outer entry further down the stack is invisible to any
error thrown during the inner catch block's execution.

**Resolution (July 2026):**  
Fixed in `internal/language/bytecode/catch.go` (`handleCatch`). The single check against
`c.tryStack[len(c.tryStack)-1]` was replaced with a search from the top of `c.tryStack`
downward for the nearest frame that can actually catch the error, for two related reasons
uncovered during the fix (the second broadens the original bug report slightly, since it is
the same root defect wearing a different hat):

1. **(the reported bug)** A frame with `addr == 0` means that level's catch block is
   already executing and cannot be re-entered — but it must not be treated as "nothing can
   catch this." The search now skips it and keeps looking further down for a live
   (`addr > 0`) enclosing frame.
2. **(selective-catch escalation — same defect, different trigger)** A *live* frame
   (`addr > 0`) can still fail to catch a given error if it was armed with a selective
   `catches` list — the mechanism used both by a named `catch(SomeSpecificError)` clause
   and, notably, by the `?expr : fallback` optional operator (compiled with
   `bytecode.OptionalCatchSet`, `internal/language/compiler/expr_atom.go:848`). Before the
   fix, a non-matching selective frame caused the same premature "uncaught" result as case 1,
   even when a live, catch-all try/catch was available further out. For example, `?failer() :
   -1` nested inside a user `try { ... } catch(e) { ... }` would lose an error raised by
   `failer()` that isn't one of the small set of errors the optional operator itself
   recognizes (nil pointer, invalid type, unknown member, divide-by-zero, array index, type
   mismatch, invalid value, invalid integer/float) — exactly the same symptom as the
   original report, just reached through the `?:` construct instead of literal nested
   `try`/`catch`.

Once the correct catching frame is identified, two additional details had to be handled so
neither `c.tryStack` nor the runtime execution stack are left in a corrupted state:

- **Orphaned `tryInfo` frames.** Any frame(s) skipped over above the one that actually
  catches will never reach their own `TryPop` instruction (control jumps directly into the
  outer catch, past the remaining bytecode of the skipped frames). `c.tryStack` is now
  truncated down to (and including) the matched frame immediately, discarding those orphans
  instead of leaking them for the life of the context.
- **Orphaned `"try"` `StackMarker`s on the execution stack.** A frame skipped because it was
  already spent (case 1) had its marker consumed the first time its own catch was entered,
  so it contributes nothing further. A frame skipped because of a selective-catch mismatch
  (case 2) was never entered at all, so its `"try"` marker is still sitting on the execution
  stack above the marker for the frame we are unwinding to. The unwind loop that pops values
  until it finds the `"try"` marker was changed to count how many markers must be consumed
  (one for the matched frame, plus one for each live-but-skipped frame) rather than stopping
  at the first one it finds — otherwise it would stop at an inner, bypassed frame's marker
  and leave the outer frame's own marker (and the outer try-block's now-stale contents)
  behind on the stack.

**Tests added:**

- `internal/language/bytecode/try_catch_test.go` (Section 7, "BUG-35"):
  `Test_handleCatch_BUG35_ErrorDuringCatchBlockEscalatesToOuterTry`,
  `Test_handleCatch_BUG35_NoEnclosingTry_ErrorPassesThrough`,
  `Test_handleCatch_BUG35_DiscardsMultipleOrphanedFrames`, and
  `Test_handleCatch_BUG35_SelectiveCatchMismatchEscalatesToOuterTry` (the selective-catch /
  `?:`-flavored variant, including verifying the execution stack ends up empty rather than
  leaving a stray marker behind).
- `tests/errors/try_catch.ego`: `"errors: BUG-35, error in catch block escalates to outer
  try"` (the original two-level reproducer), `"errors: BUG-35, error escalates through three
  nested levels"`, and `"errors: BUG-35, no enclosing try means error still
  fatal-catchable-only-once"`.
- `tests/errors/optional.ego`: `"errors: BUG-35, optional operator escalates unlisted error
  to enclosing try"` and `"errors: BUG-35, optional operator nested inside a catch block"`
  (the `?:`-specific variant of the fix, combining both escalation cases in one test).

