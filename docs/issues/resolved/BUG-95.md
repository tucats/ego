# BUG-95 — A multi-target `=` assignment silently loses its effect when it runs in a block scope nested below where the variables were declared

**Severity:** HIGH

**Discovered by:** The user, debugging a pi-digit-generator program (`test.ego`, a Rabinowitz–Wagon
spigot algorithm) that always printed `0` for every digit instead of real output — and reproduced
identically as plain Go, ruling out the algorithm itself before this was investigated as an Ego
compiler bug.

**Status:** FIXED

**Description:**

A comma-separated multi-target *assignment* (not declaration — `a, b = ...`, not `a, b := ...`) to
already-declared variables silently did nothing whenever it executed inside a block scope nested
below wherever those variables were originally declared — an `if` body, a `for` body, a `switch`
case, or any combination of these nested inside each other. The write appeared to vanish the moment
the enclosing block's scope was popped, leaving the outer variables completely unchanged, with no
error of any kind:

```go
predigit, nines := 5, 1

if true {
    for k := 0; k < nines; k++ {
    }

    predigit, nines = 0, 0   // intended to reset both — silently had no effect
}

fmt.Printf("predigit=%d nines=%d\n", predigit, nines)   // "predigit=5 nines=1", not "predigit=0 nines=0"
```

In the original pi-digit program, this exact pattern (a `switch` state machine that periodically
resets two accumulator variables with `predigit, nines = 0, 0`) meant the reset silently failed
every time it ran with the block nested one or more scopes below the declaration — which, in that
program, was every time — so the accumulator state was never actually cleared, corrupting every
subsequently computed digit.

**Root cause:**

`assignmentTargetList` (`internal/language/compiler/lvalue.go`) compiles a comma-separated target
list for BOTH forms — `a, b := ...` (declare) and `a, b = ...` (assign) — through the same code
path. For each simple (non-compound) target, it unconditionally emitted a `SymbolOptCreate`
instruction ahead of the `Store`.

`SymbolOptCreate`'s runtime processor (`symbolCreateIfByteCode`,
`internal/language/bytecode/symbols.go`) decides whether to create a new local by checking `GetLocal`
— the *current* symbol table only, never the enclosing chain. That is exactly correct for `:=`: Go's
own shadowing rule says a multi-target declaration in a nested block always creates new locals, even
when a same-named variable exists further out (verified this remains correct after the fix — see
tests below). It is wrong for `=`, which must find and update whatever the name *already* resolves
to, however far out that is — the same job a plain `Store` already does correctly via a full
scope-chain walk (`c.set`), and the same job the *single*-target assignment path
(`assignmentTarget`, same file) already got right by only ever emitting a create opcode for `:=`.

For `=`, finding nothing in the immediate (inner) scope, `SymbolOptCreate` silently created a
brand-new local *there* instead of updating the outer variable — shadowing it, not writing to it —
and that shadow, along with the write it held, was discarded the instant the block's own scope was
popped.

Confirmed with `--log symbols`: the assignment's `Store "predigit"` ran against a freshly created
table (`block 2`, distinct from the `block 1` table the variable was declared in and still lived in),
and the final read after the block closed found the original, untouched value back in `block 1`.

A `for`-loop's own body wasn't itself implicated — earlier debugging suspected the loop's per-
iteration scope handling, and traced through its `PushScope`/`PopScope` bytecode pairing in detail
before ruling it out; the loop only mattered here because it was *any* nested block sitting between
the declaration and the reset assignment. A block with no locals of its own can be elided entirely
(PERFORMANCE.md Finding 8), which is why a bare `if true { a, b = ... }` with nothing else in it does
not reproduce this — there is no separate scope for the bug to manifest in.

**Fix:**

- `internal/language/compiler/lvalue.go` — added `multiTargetIsDeclaration`, a bounded,
  non-consuming lookahead (mirroring the existing style of `isAssignmentTarget` in the same file)
  that scans forward from the start of a candidate target list, tracking `[...]`/`(...)` nesting, to
  find the operator (`:=` vs `=` vs `<-`) that terminates it — the operator itself only appears
  *after* the whole comma-separated name list, but bytecode for each name is emitted one at a time
  as the list is parsed, before that operator has been seen, so this can't be decided by looking at
  what's already been consumed.
- `assignmentTargetList` now calls this once up front and only emits `SymbolOptCreate` for a simple
  target when the list is a genuine declaration (`:=`). The compile-time `ReferenceOrDefineSymbol`
  call stays unconditional for both forms: it also marks the name as "used" for unused-variable
  tracking, and `assignmentTargetList` is tried *speculatively* for every assignment — including a
  plain single-name `x = y` that turns out not to be a list at all once the whole statement is seen
  (no comma) — so that side effect is relied on even outside the declare/assign distinction this fix
  is about. An earlier version of this fix gated that call on `isDeclaration` too, which regressed 8
  existing tests (`err := nil; ...; err = e` patterns being flagged "declared but never used", and
  goroutine-closure tests with the same shape) by silently losing that marking for `=`; keeping it
  unconditional and gating only the bytecode emission fixed those without reopening the original bug.

**Tests added:**

- `tests/datamodel/parallel_assignment.ego` — six new `@test` cases: a multi-target `=` inside a
  nested `if` updating the outer variables; the same inside a `for`-loop body across several
  iterations; the exact two-scopes-deep (`for` nested in `if`, reset assignment after the loop)
  shape that exposed the bug originally, both with a loop that runs and one that never executes its
  body (both reproduced identically pre-fix); a regression guard confirming multi-target `:=` in a
  nested block still shadows correctly (unaffected by this fix); and a regression guard for the
  unused-variable-tracking side effect described above. Three of the six fail against the pre-fix
  code (confirmed by temporarily reverting the fix and re-running); the other three don't exercise a
  nested scope at all (by design, as regression/documentation guards) so pass either way.

**Verification:** `go build ./...`, `go vet` clean, `go test ./...` (full repository) passes with no
regressions. The full `ego test tests/` suite (1,732 cases, up from 1,726 — six new, zero removed)
passes. The original `test.ego` reproducer was cross-checked digit-for-digit against a known-correct
compiled Go build of the same corrected algorithm for output lengths from 5 up to 150 digits, with
particular attention to the range (32 digits and up) that first exposed the bug — all match exactly
after the fix.
