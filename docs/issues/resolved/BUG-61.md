# BUG-61 — `break`/`continue` skips scope cleanup for blocks it jumps out of, leaving the runtime scope one level too deep — **Resolved**

**Severity:** MEDIUM

**Description:**  
Found while writing Go unit tests for the BUG-30 fix. When a program with no enclosing
`func main()` — the form used internally by `compiler.RunString()`/`compiler.Run()`, e.g. for
the interactive REPL, evaluated one statement at a time — is instead compiled and run as a
*single* multi-statement unit containing a `for` loop with a `break` or `continue` nested
inside an `if` block, any variable declared by a statement *after* the loop ends up
inaccessible from outside that compile-and-run call, because the runtime's "current scope"
pointer is left one level deeper than it should be.

Originally this looked like a narrow, Go-harness-only issue (see the first reproducer and the
original Notes below), because in the common case of a `break`/`continue` nested only inside
plain `{ }` blocks, the leaked scope level happens to be silently absorbed by the loop's own
trailing cleanup code (see "Reproducer 2" below for the mechanism and a directly
`ego run`-reproducible case, found while implementing PERFORMANCE.md Finding 8). That
absorption is coincidental, not a real fix, and breaks down as soon as something *other* than
a plain block sits between the `break`/`continue` and the loop — most notably a `switch`
statement with a named init clause (`switch v := expr; v { ... }`), where a `continue` inside
any `case`/`default` body leaks the switch's own init-variable scope every time it fires.

**Reproducer 1** (Go, using the compiler package directly — see Notes for why this does not
reproduce through the ordinary `ego` CLI):

```go
s := symbols.NewRootSymbolTable("repro")
compiler.AddStandard(s)

err := compiler.RunString("repro", s, `
    total := 0
    for i := 0; i < 5; i++ {
        if i == 3 {
            break
        }
        total = total + i
    }
    result := total == 3
`)

v, found := s.Get("result")
fmt.Println(err, v, found)
```

**Actual output:**

```text
<nil> <nil> false
```

`result` was never actually declared as far as the caller's symbol table (`s`) is concerned,
even though the program compiled and ran with no error, and `total` (declared *before* the
loop) is present and correct (`total, _ := s.Get("total")` returns `3`).

**Expected output:**

```text
<nil> true true
```

**Post-Finding-8 note:** this exact reproducer, as written, stopped demonstrating the bug once
PERFORMANCE.md Finding 8 (a later, unrelated change) shipped: an `if` body that declares
nothing of its own now has its scope elided entirely, so there is no scope left for `break` to
skip over in the first place in this specific example. The underlying defect was never
specific to `if` — it is any real (non-elided) scope sitting between the `break`/`continue`
and its target loop — so the reproducer still works exactly as shown by declaring a harmless
throwaway local inside the `if` body (forcing a real scope even under Finding 8), e.g.
`if i == 3 { marker := true; _ = marker; break }`. See
`internal/language/compiler/for_bug61_test.go` for the regression tests built on this
adjusted form.

**Reproducer 2** (plain `ego run`, no Go harness needed — found while implementing
PERFORMANCE.md Finding 8):

```go
package main

import "fmt"

func main() {
    count := 0
    for i := 0; i < 6; i++ {
        switch v := i; v {
        case 2, 4:
            continue
        }
        count++
    }

    v := "after the loop"
    fmt.Println("count:", count, "v:", v)
}
```

**Actual output:**

```text
Error: at main(line 15), symbol already exists: v
Error: terminated with errors
```

**Expected output:**

```text
count: 4 v: after the loop
```

**Notes:**  
Reproducer 1 does **not** reproduce through `ego run` (a `.ego` file compiled that way
requires a `func main()`, and returning from a function unconditionally discards all of that
function's scopes regardless of exactly how many `PopScope` instructions actually ran), the
interactive REPL (each line is compiled and run as its own separate call, so a scope left
over from one line cannot affect the next), or `ego test` (each `@test { ... }` block's own
brace-delimited block scope appears to absorb the discrepancy). It was only found by calling
`compiler.RunString()` directly with a multi-statement program containing no function
wrapper at all — which is exactly the shape of several existing Go-level compiler tests, and
is why the BUG-30 unit tests in `internal/language/compiler/for_loopvar_test.go` that need
`break`/`continue` wrap the relevant code in a small named function rather than using it at
the bare top level.

Reproducer 2, by contrast, reproduces with a completely ordinary `func main()` program run
through the normal `ego run` CLI path — no special harness, REPL, or test mode involved. The
difference is *what* sits between the `continue` and the loop's own boundary. A `switch`
statement with a named init clause (`switch v := expr; v { ... }`) pushes its own extra scope
to hold `v`, on top of (and structurally unrelated to) the `for` loop's own scope(s); see
`compileSwitchAssignedValue` in `internal/language/compiler/switch.go`. When a plain `{ }`
block is what's skipped, the leaked level happens to line up with a `PopScope` the loop's own
trailing code already needed to execute for its own purposes, so it is masked — a coincidence
of the specific bytecode layout involved, not a real fix, and not something later code should
ever rely on. The switch's extra scope has no such lucky counterpart waiting downstream, so it
is never popped: `v` stays permanently declared in whatever scope the loop body shares, and
the very next attempt to declare an unrelated `v` in that same scope (here, the line right
after the loop) fails with `symbol already exists`. The same leak also affects a `continue`
inside a `case`/`default` body of an *anonymous* `switch expr { ... }` (no named init): there,
instead of leaking a named variable, it skips the `SymbolDelete` that normally removes the
switch's internal synthetic test-value symbol at the end of the statement (see
`compileSwitch`), leaking that placeholder into the enclosing scope on every iteration where
`continue` fires. This was found to matter in practice while investigating whether a switch
`case`/`default` body could safely skip its own scope when it declares nothing of its own
(PERFORMANCE.md Finding 8): doing so removes the coincidental masking for the *plain-block*
case without fixing the underlying issue, making the pre-existing bug immediately and
reliably reproducible (`ego test` runs every test file against one shared, persistent root
symbol table, so the leaked synthetic name from an early test collided with an unrelated
variable in a completely different, later test file). Scope elision was therefore **not**
applied to switch `case`/`default` bodies at all, keeping the coincidental masking intact
there until this bug has a real fix.

Root cause (both reproducers): `compileBreak`/`compileContinue`
(`internal/language/compiler/for.go`) emit a bare, unconditional `Branch` with no
accompanying `PopScope`, regardless of how many scopes (each pushed by `compileBlock` for its
own `{ }`, by `compileSwitchAssignedValue` for a switch's named init clause, or by the
anonymous-switch synthetic-symbol path) lie between the `break`/`continue` statement and the
loop's own boundary. `popScopeByteCode` (`internal/language/bytecode/symbols.go`) walks up
exactly one parent per `PopScope` executed (or exactly `N` when given an explicit
`PopScope, N` count, which nothing currently supplies for this case), so a `break`/`continue`
nested one or more scopes deep leaves that many scopes un-popped (or, for the anonymous-switch
case, one synthetic symbol un-deleted) when it lands at the loop's exit/continue point, which
only accounts for the loop's own scope(s).

**Resolution (July 2026):**  
Fixed by giving the compiler an accurate, always-up-to-date count of how many *runtime* scopes
are actually open at any point in the bytecode being generated, and having
`compileBreak`/`compileContinue` emit a `PopScope, N` for the difference before their `Branch`.

**Why `blockDepth` couldn't be reused.** The original root-cause note above suggested using
`compiler.go`'s existing `blockDepth` field. That turned out not to work: PERFORMANCE.md
Finding 8 (implemented in a later session) taught several call sites to *conditionally* skip
emitting `PushScope`/`PopScope` for a block that declares nothing, but `blockDepth` itself is
still incremented unconditionally at every one of those call sites (it also drives compile-time
unused-variable-scope tracking and an unrelated "am I lexically inside a block" check in
`import.go`, both of which need to keep counting elided blocks as if they were real). Using
`blockDepth` to compute a `PopScope, N` count would therefore have popped too many scopes
through any function/loop/switch containing an elided block.

**The new counter.** A separate field, `c.scopeDepth` (`compiler.go`), counts only *actual*
`PushScope` instructions currently open in the bytecode stream. Two new helper methods,
`emitPushScope`/`emitPopScope` (`internal/language/compiler/block.go`), are now the *only*
places in the compiler allowed to emit a `bytecode.PushScope`/`PopScope` for a block, switch,
or try/catch scope — every direct `c.b.Emit(bytecode.PushScope, ...)`/`c.b.Emit(bytecode.PopScope)`
call site in `block.go`, `for.go`, and `switch.go` was converted to go through them, keeping
`c.scopeDepth` in lockstep automatically. The one exception is `loopVariableEpilogue`'s
embedded `PopScope` (it lives inside a separately-built `*bytecode.ByteCode` fragment, not a
direct `c.b.Emit` call) — `compileForBody` decrements `c.scopeDepth` by hand at the point it
splices that fragment in, since the effect on the runtime is identical either way.

**Capturing the target depth.** The `loop` struct (`compiler.go`) gained its own `scopeDepth`
field, a snapshot of `c.scopeDepth` captured once, at the exact point each loop form's (or
switch's) own persistent wrapper scope(s) have all been pushed — e.g. right where a classic
`for` loop marks the address its condition test branches back to (`b1`). This is provably the
same scope depth that both `break`'s and `continue`'s branch targets land at for every one of
the four loop forms and for a `switch`'s own `break` target (traced by hand for each form; see
the inline comments at each capture site in `for.go`/`switch.go`).

**The fix itself**, `emitScopeUnwindTo` (`for.go`): `compileBreak`/`compileContinue` now call it
before emitting their `Branch` — it computes `c.scopeDepth - targetLoop.scopeDepth` and, if
positive, emits `PopScope, N` for that many scopes. This never touches `c.scopeDepth` itself:
the compiler's own bookkeeping must keep reflecting the lexical nesting it is still compiling
through for whatever (reachable or not) code comes textually after the `break`/`continue` —
only the bytecode emitted along that one branch's path needs the correction. This one change
transparently fixes labeled `break`/`continue` targeting an *outer* loop through several levels
of intervening blocks/switches/loops too, since `c.scopeDepth` is a simple running total
unaffected by which construct pushed which scope.

**The anonymous-switch case was unified, not special-cased.** Rather than teach
`compileBreak`/`compileContinue` about the anonymous switch's `SymbolDelete`-based cleanup as a
second, parallel mechanism, `compileSwitchAssignedValue` (`switch.go`) now pushes a real scope
for *every* switch value form — named-init, semicolon-separated init, and anonymous alike —
and `compileSwitch`'s cleanup always pops it. The old `SymbolDelete` path is gone entirely. This
means the anonymous form is now covered by the exact same `PopScope, N` mechanism as the
named-init form, with no separate code path to keep in sync.

**Switch `case`/`default` bodies are elidable again.** PERFORMANCE.md Finding 8 had explicitly
excluded switch `case`/`default` bodies from scope elision specifically *because of* this bug
(eliding a declaration-free case body removed the coincidental masking without fixing the
underlying issue — see that Finding's own write-up). With `compileBreak`/`compileContinue` now
correctly unwinding any scope they jump over, that masking is no longer needed, so
`compileSwitchCase`/`compileSwitchDefaultBlock` now use the same `blockBodyNeedsOwnScope`-style
predicate (renamed `switchCaseBodyNeedsOwnScope`, still in `block.go`) that ordinary blocks use.

**`try`/`catch` was deliberately left untouched, and investigated separately.** `try.go` was not
modified by this fix. Initial design work assumed a `break`/`continue` inside a `catch` body
would need a manual `c.scopeDepth` adjustment to account for the try body's scope surviving
into the catch handler (per BUG-64's fix note: it is deliberately left open on the error path
so the `catch` clause can store its error variable into it). Investigating that assumption
before implementing it surfaced a more fundamental reason not to: *how deep* the runtime scope
actually is when a catch handler starts is not a fixed, compile-time-knowable quantity at
all — it depends on exactly where inside the try body the error originated (e.g., a division
by zero three `if`-blocks deep inside the try body leaves three real scopes open, not one), so
a single static adjustment could never be correct in general. A `c.scopeDepth`-based fix
therefore cannot solve this the way it solves the block/switch/loop cases, where a
`break`/`continue`'s target depth is always fixed by the surrounding *lexical* structure alone.

Extensive testing (see `TestBUG61...` cases and `tests/flow/scope_unwind.ego`'s catch-body
tests, plus additional adversarial manual reproducers with errors originating both inside a
called function and directly inside deeply-nested `if` blocks within the try body) found
`break`/`continue` inside a `catch` body continuing to compute correct results in every case
tried, via two effects that are independent of this fix: (1) when the error originates inside
a function called from the try body, `handleCatch`'s existing unwind loop
(`internal/language/bytecode/catch.go`) already pops that function's own call frame while
searching for the enclosing "try" stack marker, which resets the runtime's current-scope
pointer via the same `callFramePop` mechanism BUG-64 relies on elsewhere, restoring it to
exactly the scope active when the call was made; and (2) even when scope depth is left
genuinely too deep after a `catch` body's `break`/`continue` (an error raised directly inside a
nested block within the try body, no function call involved), the operations these test
programs perform afterward — writing to an already-existing outer variable, a loop's own
increment/condition clauses — are exactly the kind of drift-tolerant operations documented
throughout this fix (they resolve names by walking up the parent-table chain, which still finds
the right variable regardless of the extra depth). No case was found where this combination
produces an incorrect result, but this is not a proof of correctness for every possible
program shape, and is called out here explicitly rather than left for a future investigator to
rediscover. A fully general fix, if one is ever needed, would most likely belong in
`handleCatch` itself (resetting the runtime scope pointer to whatever was active when the
matching `Try` instruction executed, regardless of how deep the error occurred), not in the
compiler's static `c.scopeDepth` bookkeeping.

**A second, independent bug was found and fixed alongside this one:** `simpleFor` (a bare
`for {}` loop, `for.go`) never popped `compileFor`'s own outer "ForScope" at all — unlike the
other three loop forms, which all do. This permanently leaked one scope level every time a bare
`for {}` loop ran, confirmed via `--disassemble` (the `PushScope 2` for `ForScope` had no
matching `PopScope` anywhere in the function) and via a runtime reproducer. It was not visible
as an ordinary user-facing bug in most programs (reads/writes to already-existing names survive
scope drift by walking the parent chain, and a leak entirely inside one function call is erased
by that function's own `Return`), but it would have made `c.scopeDepth` permanently wrong for
any code following such a loop, defeating the rest of this fix — so it had to be corrected
alongside `compileBreak`/`compileContinue`. Fixed by adding the missing `emitPopScope()` call.

**Files modified:**

- `internal/language/compiler/compiler.go` — new `c.scopeDepth` field on `Compiler`; new
  `scopeDepth` field on `loop`.
- `internal/language/compiler/block.go` — new `emitPushScope`/`emitPopScope` helpers;
  `compileBlock` converted to use them; `switchCaseBodyNeedsOwnScope` (renamed/restored from
  Finding 8's `blockBodyNeedsOwnScope`, generalized to share one scan with a pluggable
  terminator for switch case bodies, which are not brace-delimited).
- `internal/language/compiler/for.go` — `emitScopeUnwindTo`; `compileBreak`/`compileContinue`
  call it; every loop form converted to `emitPushScope`/`emitPopScope` and captures
  `loop.scopeDepth` at the right point; `simpleFor`'s missing ForScope pop added.
- `internal/language/compiler/switch.go` — `compileSwitchAssignedValue` always pushes a scope
  (the `hasScope` return value was removed — no longer needed by any caller);
  `compileSwitch`'s cleanup always pops it; `compileSwitchCase`/`compileSwitchDefaultBlock` use
  `switchCaseBodyNeedsOwnScope` and the new helpers; `switchLoopType`'s `loop.scopeDepth`
  captured right after `loopStackPush`.
- `internal/language/compiler/try.go` — **not modified**; see the try/catch discussion above
  for why, and for what was verified instead.
- `internal/language/compiler/for_bug61_test.go` (new) — Go-level regression tests: both
  reproducers (adjusted for the Finding-8 note above), the anonymous-switch form, the
  `simpleFor` ForScope leak (loops run directly at the top level, not each wrapped in its own
  function, so a leak that stayed entirely within one function call — masked by `Return`
  resetting scope state from the saved call frame — would not have been caught), a
  labeled-break/continue stress test through mixed `if`/`switch`/`try`/nested-`for` nesting, and
  the try/catch investigation above.
- `tests/flow/scope_unwind.ego` (new) — nine `@test` blocks covering the same scenarios
  end-to-end through the real `ego test`/`ego run` path, in both `dynamic` and `--types strict`
  modes.

**Correctness verification:** `go test ./...`, `go test -race ./...`, and both
`ego test tests/` and `ego test --types strict tests/` (1193 `@test` blocks each) pass with no
regressions.

