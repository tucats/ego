# BUG-77 — `type` declarations inside a block leaked into every later, unrelated block

**Severity:** MEDIUM

**Description:**  
Found while writing regression tests for the BUG-73 fix, and reported directly by the user as a
recurring annoyance. A `type X ...` declaration made inside any block — an `if` body, a function
body, or (most visibly) one `@test { }` block in an Ego test file — remained permanently visible
to every block compiled afterward, for the rest of the compilation unit. Worse, if a *later*,
completely unrelated block declared another type with the same name, compilation failed with
`"duplicate type name: X"`, even though the two types had nothing to do with each other (different
fields, different blocks, no shared scope in the Go sense).

This was especially disruptive for `ego test`: `internal/commands/test.go`'s `TestAction()`
compiles an entire test *file* — which may contain many `@test { }` blocks — as a single
`comp.Compile(name, t)` call, i.e. one compilation unit. Every `@test` block in the file therefore
shared the same, unscoped type namespace. Test authors were forced to invent artificially unique
type names per test (`box1`, `box2`, `box3`, `senderBox`, `ptrBox`, `multiBox`, ...) purely to
avoid an unrelated `@test` block elsewhere in the same file tripping the duplicate-name check —
exactly the "box1/box2/box3" pattern the user called out.

**Root cause:**  
`Compiler.types` (`map[string]*data.Type`) is a single flat map that spans the *entire*
compilation, with no scoping mechanism of its own. This is inconsistent with how the compiler
already treats ordinary variable declarations: `Compiler.scopes` is a proper stack, pushed by
`PushSymbolScope()` and popped by `PopSymbolScope()` around every block (see
`internal/language/compiler/block.go`'s `compileBlock()`, the single function that compiles every
kind of `{ }` block — `if`/`for`/`func`/`switch`/`try`/bare blocks/`@test` blocks all funnel
through it). `type` declarations were simply never given the same treatment, so once a name was
added to `c.types` it stayed there for the rest of the file, regardless of which block declared it.

**Reproducer:**

```go
package main

func main() {
    if true {
        type box struct {
            ch chan
        }
        b := box{ch: make(chan, 1)}
        _ = b
    }

    if true {
        type box struct { // different type, same name, different (later) block
            n int
        }
        b := box{n: 1}
        _ = b
    }
}
```

**Actual output (before fix):**

```text
Error: duplicate type name: box
```

**Expected output:**

No error. The second `box` is a distinct type scoped to its own `if` block and has no relationship
to the first.

**Resolution:**  
`compileBlock()` in `internal/language/compiler/block.go` now snapshots `c.types` (a shallow copy
— only the map's name→`*data.Type` associations are duplicated; the `*data.Type` values themselves
are treated as immutable once created) immediately after `PushSymbolScope()`, and restores that
snapshot immediately before `PopSymbolScope()` at the end of the block. This gives `type`
declarations exactly the same block-scoping behavior Go gives them, and that Ego already gave
plain variable declarations: a type declared inside a block is visible for the rest of that block
and any nested blocks, but is discarded — along with any collision it might otherwise have caused
— once the block ends.

Because `compileBlock()` is the single chokepoint for every kind of `{ }` block, this fix applies
uniformly to `if`/`for`/`func`/`switch`/`try`/bare blocks and `@test` blocks alike, with no
special-casing needed for test mode.

This incidentally also avoids BUG-76-shaped collisions in the *cross-block* case (a type declared
in an earlier block no longer exists by the time a later block declares a field with the same
name), but does **not** fix BUG-76 itself: a field name colliding with a type name that is still
live *within the same block* is unaffected, since block-scoping restores the snapshot only at the
end of the block, not partway through it. BUG-76 is a genuinely separate bug in how struct field
declarations are parsed, not a scoping issue, and was root-caused and fixed independently — see
[BUG-76](#BUG-76).

As a direct demonstration of the fix, `tests/flow/channel_compound_lvalue.ego` (originally written
for BUG-73, before this fix existed) was simplified to drop its artificially unique per-test type
names (`senderBox`/`ptrBox`/`multiBox` → `box`, `bumpPoint` → `point`) back to natural, reused
names, since every `@test` block in the file now has its own independent type namespace.

Verified with `go build ./...`, `go vet ./...`, `go test ./...` (all clean) and `ego test tests/`
under `--types dynamic`, `--types strict`, and `--types relaxed` (1361 tests passing in all three
modes, matching the pre-fix count exactly — zero regressions from making type scoping stricter).

