# BUG-59 — `@compile ... optimize=N` silently has no effect

**Severity:** LOW

**Description:**  
`docs/LANGUAGE.md` documents `@compile`'s `optimize=N` option as overriding the optimizer
level "for just this block of code." In practice, the override is never actually applied —
it is written to a settings key that nothing reads.

**Reproducer:**

```go
func main() {
    before := profile.Get("ego.compiler.optimize")
    fmt.Println("before:", before)
    @compile block optimize=2 {
        x := 1
        fmt.Println(x)
    }
    after := profile.Get("ego.compiler.optimize")
    fmt.Println("after:", after)
}
```

**Actual output:**

```text
before: false
1
after: false
```

**Expected output:**

The `optimize=2` override should have some observable effect while the block compiles
(e.g. on the setting read by the optimizer, or on the resulting bytecode), rather than none
at all.

**Notes:**  
Root cause: `internal/language/compiler/directives.go` (~lines 759, 870, 876) — the
save/restore logic uses `defs.OptimizerSetting` (`"ego.compiler.optimize"`, the real key
read by `bytecode.go:332`) to *capture* the saved value, but *writes* the override to
`defs.OptimizerOption` (`"optimize"` — an unrelated CLI-grammar-option constant from
`internal/defs/constants.go`, never read anywhere for this purpose). The override is dead
code.

**Fix:**  
`compileBlockDirective` in `internal/language/compiler/directives.go` now writes the
`optimize=N` override (and its restore-on-exit) to `defs.OptimizerSetting` instead of
`defs.OptimizerOption`, so the value the sub-compiler's bytecode actually reads via
`settings.GetInt(defs.OptimizerSetting)` in `bytecode.go:332` is the one the directive sets.
Verified with `--log optimizer`: a block compiled with `optimize=2` now shows the optimizer
finding and applying patches for that block specifically (e.g. "Found 4 optimizations for a
net change of 6 instructions"), while a block compiled with `optimize=0` shows "Optimizations
disabled by configuration setting" and no patches, and the setting is correctly restored to
its prior value once the block finishes compiling. `defs.OptimizerOption` remains in use
elsewhere (the `--optimize` CLI flag in `internal/grammar/traditional.go`), so it was not
removed — only the directive's misuse of that constant was corrected.

