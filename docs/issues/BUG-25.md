# BUG-25 — `fallthrough` into a `switch`'s `default`/terminal clause causes an infinite loop

**Severity:** CRITICAL  
**Status:** Fixed

**Description:**  
`fallthrough` from a `case` clause into the `default` clause — a perfectly legal, idiomatic
Go construct — causes the program to loop forever instead of executing the `default` block
once. The same happens when `fallthrough` is used as the last statement of a switch's only/
last `case` with no following clause at all (which real Go rejects as a compile-time error,
"cannot fallthrough final case in switch").

**Reproducer:**

```go
func main() {
    x := 2
    switch x {
    case 1:
        fmt.Println("one")
    case 2:
        fmt.Println("two")
        fallthrough
    default:
        fmt.Println("default")
    }
}
```

**Actual output:**

```text
two
two
two
two
... (repeats forever until killed; ./ego --timeout 3s run reports
     "INTERNAL: Ego execution time exceeded 3s; terminating program execution")
```

**Expected output:**

```text
two
default
```

**Notes:**  
Root cause in `internal/language/compiler/switch.go`: `compileSwitchCase` records the
`fallthrough` branch's fixup address and only patches it when the *next* clause is itself
compiled by `compileSwitchCase` (i.e. another `case`). `compileSwitchDefaultBlock` never
patches this address, and if no clause follows at all, `compileSwitch`'s main loop exits
without patching it either. The resulting branch instruction is left targeting address `0`,
so execution jumps to the start of the function's bytecode and re-enters the switch,
looping forever. `fallthrough` into a following `case` works correctly because that path is
patched; only fallthrough-into-`default` and fallthrough-as-terminal-statement are affected.

**Fix:**  
`compileSwitch()` in `internal/language/compiler/switch.go` now tracks a pending
`fallthrough` branch address across the whole switch instead of only inside
`compileSwitchCase()`:

1. When the main clause loop encounters a `default:` token while a `fallthrough` from the
   previous `case` is still pending, the pending address is moved into a new
   `fallThroughToDefault` variable instead of being dropped. Because `compileSwitch` always
   compiles the `default` body into a separate buffer and appends it to the very end of the
   switch's bytecode (after every `case`, regardless of where `default:` appears in the
   source), the branch target isn't known until that append happens — so the patch is applied
   at that point, immediately before `c.b.Append(defaultBlock)`, using `SetAddressHere`.
2. If the main clause loop instead runs out of clauses (hits the switch's closing `}`) while a
   `fallthrough` is still pending and was never claimed by a `default:`, `compileSwitch` now
   returns a new compile-time error, `errors.ErrInvalidFallthrough` ("cannot fallthrough final
   case in switch") — matching real Go's rejection of the same construct — instead of silently
   emitting a branch to address `0`. This also correctly covers the case where a `default:`
   clause exists elsewhere in the switch but does not immediately follow the `fallthrough` in
   source order (e.g. `default:` declared *before* the final `case`): since nothing follows the
   `fallthrough` textually, it is still an error, exactly as in Go.
3. `fallthrough` into a following `case` is unaffected — that path was already correct and
   continues to be patched inside `compileSwitchCase()` itself.

**New error added:**  
`errors.ErrInvalidFallthrough` (`internal/errors/messages.go`, key `invalid.fallthrough`),
with entries added to all three localization files
(`messages_en.txt`, `messages_fr.txt`, `messages_es.txt`).

**Incidental fix (found while adding tests for this bug):**  
`compileBlockDirective()` (`internal/language/compiler/directives.go:757`), which implements
the `@compile` test directive, read and restored the wrong settings key when computing the
default "unused variable" enforcement for a `@compile` block — it used
`defs.UnusedVarLoggingSetting` (a verbose-logging toggle) instead of `defs.UnusedVarsSetting`
(the actual error-enforcement flag). This is one of the two root causes tracked as
[BUG-54](#BUG-54) (now fully fixed; see that entry). Fixing the key mismatch here was
necessary to write reliable `@compile`-based regression tests for BUG-25 without them
tripping over unrelated stale global settings.

**Files changed:**

- `internal/language/compiler/switch.go` — track and patch (or reject) a pending
  `fallthrough` that targets a `default` clause or has no following clause at all
- `internal/errors/messages.go` — added `ErrInvalidFallthrough`
- `internal/i18n/languages/messages_en.txt`, `messages_fr.txt`, `messages_es.txt` — added the
  `invalid.fallthrough` localization key
- `internal/language/compiler/directives.go` — fixed `compileBlockDirective()` to read/restore
  `defs.UnusedVarsSetting` instead of `defs.UnusedVarLoggingSetting`
- `internal/language/compiler/switch_test.go` — new Go unit tests: fallthrough into `default`
  for both value and conditional switches, fallthrough as a terminal statement (with and
  without an earlier `default:`) now producing `ErrInvalidFallthrough`, and a non-regression
  check that fallthrough into a following `case` still works
- `internal/language/compiler/directives_test.go` — new Go unit test verifying `@compile` no
  longer leaks/corrupts `defs.UnusedVarsSetting` via the wrong settings key
- `tests/flow/switch_advanced.ego` — new Ego-level regression tests covering the same
  scenarios end-to-end, including two `@compile`-based tests that assert the new
  `invalid.fallthrough` compile error is produced

