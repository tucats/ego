# BUG-31 — A bare `break` inside a `switch` incorrectly targets the enclosing `for` loop

**Severity:** HIGH

**Description:**  
In Go and every C-family language, a bare `break` inside a `switch` exits only the
`switch`, not any enclosing loop. In Ego, `break` inside a `switch` exits the nearest
enclosing `for` loop instead — silently wrong control flow. Additionally, `break` inside a
`switch` with *no* enclosing loop at all produces a spurious compile error, even though
that is perfectly legal Go.

**Reproducer:**

```go
func main() {
    for i := 0; i < 5; i++ {
        switch i {
        case 3:
            break
        }
        fmt.Println(i)
    }
}
```

**Actual output:**

```text
0
1
2
```

**Expected output** (verified against real Go):

```text
0
1
2
3
4
```

**Second reproducer** (no enclosing loop):

```go
func main() {
    x := 5
    switch x {
    case 5:
        break
    }
    fmt.Println("after switch")
}
```

**Actual output:**

```text
Error: at line 5:1, loop control statement outside of for-loop
```

**Expected output:**

```text
after switch
```

**Notes:**  
Root cause: `internal/language/compiler/switch.go` never pushes anything onto the
compiler's loop stack (`c.loops`) for a `switch` construct. `compileBreak`
(`internal/language/compiler/for.go`) only knows about `c.loops`, populated exclusively by
`for`-loop compilation, so a bare `break` always resolves to the innermost `for` loop if one
exists, or errors if none does. `continue` is unaffected by this bug, since Go has no
"continue the switch" concept and continuing the enclosing loop is the correct target.

**Resolution (July 2026):**  
`internal/language/compiler/for.go` and `internal/language/compiler/switch.go`:

- Added a new `switchLoopType` value to the `runtimeLoopType` enum in `for.go`. This
  is not a real loop type (nothing ever iterates) — it exists purely so a `switch`
  construct has a place of its own on the compiler's loop stack (`c.loops`) for `break`
  statements to attach to.
- `compileSwitch` (`switch.go`) now calls `c.loopStackPush(switchLoopType)` right before
  compiling the switch's case/default bodies, and patches every `break` collected on that
  entry to land at the switch's normal exit point (the same address a case miss already
  branches to), then calls `c.loopStackPop()`. This is the exact same push/pop pattern
  `for`-loop compilation already used, just applied to `switch`.
- `compileBreak` needed no changes: it already resolves an unlabeled `break` to the
  top of `c.loops`, which is now correctly the switch (if one is the innermost
  enclosing construct) rather than skipping past it to the next `for` loop out.
- `compileContinue` was changed to walk past any `switchLoopType` entries — both when
  picking the default (innermost) target and when a label resolves to one — since real Go
  has no "continue the switch" concept; a bare or labeled `continue` must always land on a
  genuine `for` loop. If no such loop remains after skipping switch entries, compilation
  still fails with the same `errors.ErrInvalidLoopControl` as before.

As a side effect, a `switch` with no enclosing loop at all no longer needs a loop to exist
merely to give `break` somewhere to go — `c.loops` is non-nil for the duration of compiling
the switch body regardless of what (if anything) encloses it, so the bug report's second
reproducer (`break` inside a switch with no loop around it) now compiles and runs instead of
failing with `ErrInvalidLoopControl` ("loop control statement outside of for-loop").

New tests:

- `internal/language/compiler/switch_test.go` — six new `TestBUG31*` unit tests: a `break`
  inside a `switch` inside a `for` loop completing every iteration;  a `break` inside a
  `switch` with no enclosing loop at all; a `continue` inside a `switch` correctly skipping
  an iteration of the enclosing `for` loop; a labeled `break outer` reaching through a
  nested `switch` to a labeled loop; a bare `break` in a `switch` nested inside another
  `switch` exiting only the inner one; and a bare `continue` inside a `switch` with no
  enclosing loop still failing to compile with `ErrInvalidLoopControl`.
- `tests/flow/switch_advanced.ego` — six matching `@test` blocks (suffixed `(BUG-31)`)
  covering the same scenarios at the Ego-language level, including a `@compile`/`catch`
  regression check that `continue` outside of any loop remains a compile error.

