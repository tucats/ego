# BUG-82 — Several `tables` package methods violate the `(value, error)` return convention

**Severity:** MEDIUM

**Description:**  
Found during a documentation review of the `tables` package. Several methods either
declared a return signature that didn't match their actual behavior, or returned a
success-path value that broke normal `if err != nil` error checking:

- `tables.New()` (`newTable` in `table.go`) was declared with only a single
  `TablesTableType` return, no error — but its underlying `tables.New(headings)` call can
  genuinely fail (terminal-size detection). A bare non-list error on a non-error-typed
  single return becomes an uncatchable-except-try/catch runtime abort instead of a normal
  returned error.
- `Close()` (`closeTable`) and `Pagination()` (`setPagination`) are both declared with a
  single `ErrorType` return -- the "value-slot" convention documented at the top of
  `types.go` (`return err, err`) -- but each returned the literal boolean `true` on
  success instead of `nil`. Since the single-ErrorType dispatch path pushes the success
  value verbatim, `if err := t.Close(); err != nil` saw the non-nil `true` and treated
  every successful close as a failure.
- `String()` (`toString`) is declared to return `(string, error)` but its implementation
  returned Go's bare two-value tuple (`return t.String(fmt)`) instead of wrapping it in
  `data.NewList`. A two-value assignment (`s, err := t.String(...)`) failed outright with
  "incorrect number of return values"; only single-value usage happened to work.
  `Get()` (`getTableElement`) and `GetRow()` (`getRow`) each had one early-return path (an
  invalid, non-coercible `rowIndex` argument) that returned a bare `nil, err` instead of
  the `data.NewList(nil, err), err` pattern used by every other path in the same function
  -- an inconsistency that made that one specific error path uncatchable via a normal
  `v, err := ...` assignment even though the function's other error paths worked fine.
- `Len()` (`lenTable`) and `Width()` (`widthTable`) were declared with only a single
  `IntType` return, no error -- but both call `getTable(s)`, which fails on a closed or
  invalid table receiver, hitting the same bare-non-list-error-on-non-error-return bug as
  `tables.New()` above.

**Fix:**  
`New()`, `Len()`, and `Width()` now declare `(value, error)` and wrap both paths in
`data.NewList`. `Close()` and `Pagination()` now return `nil` (via `err`, which is `nil` at
that point) instead of `true` on success. `String()`, `Get()`, and `GetRow()` now wrap
every return path in `data.NewList` consistently. Updated `internal/runtime/tables/tables_test.go`
accordingly (it directly asserted the old `got != true` behavior for `Close()`/`Pagination()`,
which had to be corrected to `got != nil` alongside the fix, and several tests needed a new
`unwrapValue` helper to unwrap the now-`data.List` results of `newTable`/`lenTable`/`widthTable`/`toString`).

