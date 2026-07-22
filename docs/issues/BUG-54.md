# BUG-54 — `@compile ... unused=false` does not suppress "unused variable" errors

**Severity:** MEDIUM  **Status:** Fixed

**Description:**  
`docs/LANGUAGE.md` documents that `@compile`'s `unused=` flag, when set, overrides the
default unused-variable-checking setting for that compilation — `unused=false` should mean
an unused local variable inside the compiled block does not produce a compile error. In
practice, the override has no effect; the error is still reported.

**Reproducer:**

```go
@compile block unused=false {
    neverUsed := 42
} catch(e) {
    fmt.Println("BUG: got compile error even with unused=false: ", e)
}
fmt.Println("done")
```

**Actual output:**

```text
BUG: got compile error even with unused=false:  at ...(line ...), variable created but never used: neverUsed
done
```

**Expected output:**

```text
done
```

(no error caught)

**Notes:**  
Root cause: `Compiler.Errors()` (`internal/language/compiler/compiler.go:358-386`) sweeps
any still-open scope's usage errors into `c.symbolErrors` unconditionally, without checking
`c.flags.unusedVars`. That flag is only honored inside `PopSymbolScope`, for scopes popped
before the compile unit finishes — the block's own top-level scope is swept by `Errors()`
regardless of the `unused=` override. Separately, the *default* value used when `unused=` is
omitted is seeded from the wrong setting key (`UnusedVarLoggingSetting`, a logging toggle,
rather than `UnusedVarsSetting`) at `directives.go:757`.

**Update (found and fixed while working on [BUG-25](#BUG-25)):**  
The second root cause above — `directives.go:757` reading `defs.UnusedVarLoggingSetting`
instead of `defs.UnusedVarsSetting` — has been fixed. This mismatch was not just cosmetic:
because the same (wrong) value was also written back to `defs.UnusedVarsSetting` when the
`@compile` directive finished (its "restore the setting to how we found it" step), running a
single `@compile` statement with no explicit `unused=` flag could permanently overwrite the
real `defs.UnusedVarsSetting` for the rest of the process with whatever
`defs.UnusedVarLoggingSetting` happened to be — silently turning "unused variable" error
enforcement on or off process-wide. A regression test,
`TestCompileBlockDirectiveDoesNotLeakUnusedVarsSetting` in
`internal/language/compiler/directives_test.go`, sets the two settings to different values,
runs a `@compile` block, and confirms `defs.UnusedVarsSetting` is unchanged afterward.

The primary reported symptom — `@compile ... unused=false` still reporting the error — was
**not** fixed by the above change, since it is caused by the first root cause
(`Compiler.Errors()` ignoring `c.flags.unusedVars` for the block's own top-level scope).

**Fix (primary symptom):**  
`Compiler.Errors()` (`internal/language/compiler/compiler.go`) now gates its sweep of
still-open scopes on `c.flags.unusedVars`, the same flag `PopSymbolScope` already checks
for every other scope:

```go
if c.flags.unusedVars {
    for _, scope := range c.scopes {
        for v, e := range scope.usage {
            ...
        }
    }
}
```

The reason the bug only affected the compiled block's own top-level scope: nested scopes
(if/else arms, loop bodies, function bodies, ...) are closed by explicit `PopSymbolScope`
calls as compilation proceeds, and that function already honored the flag correctly. The
block's own outermost scope, however, is lazily created by the first `DefineSymbol` call
and is never popped by anything — it is only ever swept up when the sub-compiler closes and
`Errors()` runs, which is exactly the code path that was ignoring `c.flags.unusedVars`.
Since `@compile`'s `unused=` flag is written directly
into the sub-compiler's `c.flags.unusedVars` before compilation starts, this single check
fixes the override for both the explicit `unused=false` case and the default (no `unused=`
given, falling back to whatever `defs.UnusedVarsSetting` already was) case alike.

Verified against the documented reproducer (now prints only `done`, with no error caught)
and the inverse case (`unused=true`, and the default with no flag at all, both still catch
the error correctly). Regression tests: `TestCompileBlockDirectiveUnusedFalseSuppressesError`
and `TestCompileBlockDirectiveUnusedTrueStillReportsError` in
`internal/language/compiler/directives_test.go`, plus two new `@test` cases in
`tests/directives/compile.ego`.

