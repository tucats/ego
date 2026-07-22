# BUG-86 — Loop closure-capture (BUG-30) silently breaks whenever the bytecode optimizer is active

**Severity:** HIGH

**Description:**  
Found while profiling `examples/mandelbrot3.ego` for `docs/PERFORMANCE.md` and
re-running the existing regression suite with the optimizer forced on
(`ego test tests/flow/for_loopvar.ego -o=2`). A classic `for i := 0; i < n; i++`
loop whose body captures `i` in a closure must give each iteration its own copy of
`i` (BUG-30, Go 1.22+ semantics); the fix for that lives in
`iterationFor` (`internal/language/compiler/for.go`), which splices a per-iteration
copy-in/copy-out prologue/epilogue around the loop body — but only when it detects
the loop counter is a "simple" named variable, via:

```go
if firstInstr := indexStore.Instruction(0); firstInstr != nil {
    switch firstInstr.Operation {
    case bytecode.Store:
        isSimpleIndex = true
    case bytecode.SymbolCreate:
        if second := indexStore.Instruction(1); second != nil && second.Operation == bytecode.Store {
            isSimpleIndex = true
        }
    }
}
```

`indexStore` (the loop counter's own `i := 0` initializer) is built by
`assignmentTarget` (`lvalue.go`), which calls `bc.Seal()` on it before returning —
sealing runs the peephole optimizer immediately, on that small buffer alone, whenever
`ego.compiler.optimize` is `2` (always) or `1` with bytecode large enough to cross the
conditional threshold. The optimizer's "Create and store" rule collapses
`SymbolCreate "i"; Store "i"` into a single `CreateAndStore "i"` instruction *before*
`isSimpleIndex`'s switch ever runs. Since neither case in the switch recognizes
`bytecode.CreateAndStore`, an entirely ordinary loop counter was silently classified
as "not simple," and the entire prologue/epilogue was skipped — every closure created
in the loop body then captured the loop's single, shared, post-loop value of `i`
instead of its own iteration's value, exactly the pre-Go-1.22 behavior BUG-30 exists
to prevent:

```ego
var funcs []any
for i := 0; i < 3; i++ {
    funcs = append(funcs, func() int { return i })
}
funcs[0](), funcs[1](), funcs[2]()
// want: 0, 1, 2   (BUG-30's own guarantee)
// got with -o 2:  3, 3, 3
```

**Fix:**  
`isSimpleIndex`'s switch also recognizes `bytecode.CreateAndStore` as a simple index
(same case as `bytecode.Store`) — a `CreateAndStore` at `indexStore.Instruction(0)` can
only arise from the optimizer having already collapsed the `":="` shape, since a plain
`"for i = 0; ..."` (pre-declared variable, `"="` not `":="`) initializer is just a lone
`Store` with nothing preceding it to collapse.

Regression coverage: `internal/language/compiler/for_optimizer_regression_test.go`'s
`TestForLoopClosureCapture_SurvivesOptimizer` forces
`ego.compiler.optimize = 2` and asserts three closures created in a loop each capture
a distinct value — confirmed failing (`3, 3, 3`) with the fix reverted. The full
existing Ego-level suite, `tests/flow/for_loopvar.ego`, now also passes at `-o 1` and
`-o 2`, not just the default `-o 0` it was previously only ever run at.

