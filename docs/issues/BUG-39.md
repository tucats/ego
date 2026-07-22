# BUG-39 — `@compile block` corrupts parsing when the block body contains any nested `{ }`

**Severity:** HIGH

**Description:**  
An `@compile block { ... }` directive whose body contains any nested brace pair (an `if`,
`for`, or `func` block) fails to parse — the compiler stops collecting tokens for the block
too early, desynchronizing the surrounding token stream so that a subsequent, syntactically
valid `catch(e)` clause is rejected as an unexpected token. The non-`block` (full-program)
form of `@compile` has the same underlying defect, surfacing once the program contains two
or more top-level braced constructs (e.g. two `func` declarations).

**Reproducer:**

```go
func main() {
    @compile block {
        x := 1
        if true {
            x = 2
        }
        fmt.Println(x)
    } catch(e) {
        fmt.Println("compile error: ", e)
    }
    fmt.Println("done")
}
```

**Actual output:**

```text
Error: at line 8:7, unexpected token: Reserved "catch"
Error: terminated with errors
```

**Expected output:**

The block should compile as ordinary Ego code (per `docs/LANGUAGE.md`'s `@compile`
section), with any resulting compile error caught by the trailing `catch(e)`. Since the
block here is entirely valid Ego, the program should print `2` then `done`.

**Notes:**  
Root cause in `internal/language/compiler/directives.go` (`compileBlockDirective`,
token-collection loop, ~line 897-920): the brace-depth counter starts at 1 and stops
collecting as soon as it returns to `<= 1`:

```go
} else if t.Is(tokenizer.BlockEndToken) {
    braces--
    if braces <= 1 {   // BUG: should be braces == 0
        break
    }
}
```

Any nested `{ }` that returns depth to 1 (i.e. the closing brace of the `if` block)
terminates collection early, silently discarding the rest of the intended block. For the
non-`block` form only, a compensating single `tokens.Append(tokenizer.BlockEndToken)`
happens to paper over exactly one dropped brace, which is why single-top-level-construct
full-program examples happen to work while anything with a second top-level braced
construct fails the same way.

**Resolution (July 2026):**  
`internal/language/compiler/directives.go` (`compileBlockDirective`, brace-counting loop):
changed the break condition from `braces <= 1` to `braces == 0`, exactly as suggested in the
root-cause notes above. `braces` starts at 1 because the directive's own opening `{` was
already consumed before the loop begins; the loop must keep collecting tokens through any
number of nested `{ }` pairs and only stop once the count returns all the way to 0 — i.e.
the token that matches the directive's *own* opening brace, not merely the closing brace of
the first nested construct it happens to encounter.

With that one-line fix in place, the previously-necessary compensating logic immediately
below it became actively harmful rather than merely redundant, and was removed:

```go
// removed:
if !blockMode && !c.t.IsNext(tokenizer.BlockEndToken) {
    return c.compileError(errors.ErrMissingStatement)
}

if !blockMode {
    tokens.Append(tokenizer.BlockEndToken)
}
```

This block existed only to patch back the single closing brace that the `braces <= 1` bug
used to drop from full-program (non-`block`) mode. Once the loop itself stops at the
correct place, there is no dropped brace left to patch, and no second closing brace left
in the source to look for — keeping this code would have made the fixed loop's cleanly
collected tokens incorrect again (an extra, unbalanced closing brace appended to `tokens`)
and would have raised a spurious `ErrMissingStatement` looking for a brace that no longer
exists in the stream.

Both the `block` form (a nested `if`, matching the bug's reproducer exactly) and the
full-program form (two sibling top-level function declarations) now compile and run
correctly; the previously swallowed `catch(e)` clause, and any code after it, parses as
ordinary code again. Verified with new Go tests in
`internal/language/compiler/directives_test.go`
(`TestCompileBlockDirectiveNestedBraceInBlockMode`,
`TestCompileBlockDirectiveTwoTopLevelConstructsFullProgramMode` — both confirmed to fail
against the pre-fix code and pass against the fix), two new Ego tests in
`tests/directives/compile.ego`, and a full run of `tools/gotests.sh` plus `ego test` under
all three typing modes (strict/relaxed/dynamic).

