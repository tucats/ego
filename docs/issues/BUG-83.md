# BUG-83 — `time` package: `durationString` return-convention violation and a shared-singleton base type

**Severity:** MEDIUM

**Description:**  
Found while adding `time.Time.SleepUntil()` (see MEMBERS-8 and MATH-12 for the two
dispatch-layer bugs that had to be fixed first for the new method to be callable at
all).

- `durationString` (`Duration.String()`'s Ego implementation, in
  `internal/runtime/time/duration.go`) is declared to return `(string, error)`, but
  returned a bare `nil, err` on its two error paths (a non-bool `extendedFormat`
  argument; no receiver bound) instead of `data.NewList(nil, err), err`. Since dispatch
  only routes a `*data.List` result through the `(value, error)` convention, this meant
  `s, err := d.String(...)` without a surrounding `try`/`catch` crashed the whole
  program on a bad argument instead of returning a normal catchable error.
- `TimeType` (`internal/runtime/time/types.go`) was constructed as
  `data.TypeDefinition("Time", data.StructType)` — using `data.StructType`, a
  process-wide **shared singleton** `*Type` value (`internal/language/data/constructors.go`),
  as its base — instead of a fresh `data.StructureType()` instance the way
  `TimeDurationType`/`TimeLocationType`/`TimeMonthType` in the same file correctly do.
  `DefineFunction`/`DefineNativeFunction` mutate the *wrapper* type's own `.functions`
  map (not the shared base), so this did not turn out to be the root cause of the
  `SleepUntil` dispatch failure (see MEMBERS-8) — but it remains a latent hazard flagged
  during review: `data.StructType` is referenced from many unrelated places in the
  codebase, and any future code path that reads through a `TypeKind` wrapper's
  `valueType` fields/functions directly (rather than the wrapper's own) would silently
  cross-contaminate every other type built on the same shared base.

**Fix:**  
`durationString` now wraps both the success and error paths in `data.NewList`, matching
its declared `(string, error)` signature. `TimeType` now uses `data.StructureType()` like
its sibling types in the same file — confirmed via grep that no other `TypeDefinition`
call in the codebase uses the shared `data.StructType` singleton as a base.

Regression test `TestDurationString_ReturnsDataList`
(`internal/runtime/time/time_test.go`) asserts both the success and error paths return a
`data.List`. Existing `TestDurationString_*` tests were updated to unwrap the now-`data.List`
result via a new `unwrapValue` test helper.

