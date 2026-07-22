# BUG-66 — `(e error) In(name)` and `(e error) At(line)` mutate the receiver in place instead of returning a clone, unlike `Context()` — **Resolved**

**Severity:** MEDIUM

**Description:**  
Found immediately after investigating BUG-65, while double-checking the Ego-exposed error
methods documented in `docs/LANGUAGE.md`. `Context()` (`internal/errors/context.go`) clones
the receiver (`e = e.Clone()`) before modifying it, so calling `e.Context(v)` leaves `e` itself
untouched and returns an independent error value — matching its documented behavior ("returns
a new error"). `In()` and `At()` (`internal/errors/location.go`) did not clone: they mutated
the receiver's `location` field directly and returned the *same* pointer. Any Ego (or Go)
code holding two references to the same underlying error value — most easily reproduced by
calling `.In()` or `.At()` through one variable and reading a second variable that was set
from the same original expression — would see the second variable's formatted message change
retroactively.

**Reproducer:**

```go
package main

func main() {
    base := errors.New("not found")

    e1 := base.In("readFile")
    fmt.Println("e1 right after:", e1.Error())

    e2 := base.In("writeFile")   // intended to be independent of e1
    fmt.Println("e1 after a second, unrelated .In() call:", e1.Error())
    fmt.Println("e2:", e2.Error())
}
```

**Actual output:**

```text
e1 right after: in readFile, not found
e1 after a second, unrelated .In() call: in writeFile, not found
e2: in writeFile, not found
```

**Expected output:**

```text
e1 right after: in readFile, not found
e1 after a second, unrelated .In() call: in readFile, not found
e2: in writeFile, not found
```

**Notes:**  
Grepping the codebase for existing callers of `.In(`/`.At(` found exactly two call sites that
relied on the old mutate-in-place behavior by discarding or ignoring the return value:
`internal/language/compiler/errors.go`'s `compileError` (`e.In(c.activePackageName)` as a bare
statement, used to attach package/file context to essentially every compiler-generated error
message) and `internal/runtime/errors/new.go`'s `newError` (the Ego-visible `errors.New()`
builtin's verbose-mode path, `_ = result.In(...)` / `_ = result.At(...)`). Both had to be
updated to capture the return value (`e = e.In(...)`) as part of this fix, or the package name
and source line/column would have silently stopped being attached to every compiler error and
every verbose `errors.New()` error, respectively.

**Resolution (July 2026):**

- `internal/errors/location.go` — `In()` and `At()` now call `e = e.Clone()` before mutating
  the (now-local) copy's `location` field and returning it, exactly mirroring `Context()`.
- `internal/language/compiler/errors.go` — `compileError` changed from `e.In(...)` (statement)
  to `e = e.In(...)` (assignment) at both call sites.
- `internal/runtime/errors/new.go` — `newError` changed from `_ = result.In(...)` /
  `_ = result.At(...)` to `result = result.In(...)` / `result = result.At(...)`.

Regression tests were added at two levels:

- `internal/errors/location_test.go` (new) — `TestError_In_DoesNotMutateReceiver`,
  `TestError_At_DoesNotMutateReceiver`, and
  `TestError_In_SharedValue_TwoCallersDoNotInterfere` (the exact two-variable scenario above).
- `tests/errors/in_at_clone.ego` — four `ego test` cases covering both methods individually,
  the two-caller interference scenario, and confirming the existing chained-call idiom
  (`errors.New(...).In(...).At(...)`) still works correctly now that each step returns a new
  value rather than the same mutated one.

Verified against `go test ./...`, `go test -race ./...`, and both `ego test tests/` and
`ego test --types strict tests/` (1229 `@test` blocks each, including a manual check that
compiler error messages still correctly show the offending package name and source
line/column after the `compileError` fix) with no regressions.

