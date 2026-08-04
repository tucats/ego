# BUG-84 — `util` package: `setLogger`, `getLogContents`, and `getPackage` violate the `(value, error)` return convention

**Severity:** MEDIUM

**Description:**  
Found during a documentation review of the `util` package (an Ego-specific package with
no Go corrolary). Three functions were declared with a non-error-typed single (or
error-omitting) `Returns`, but returned a bare `nil, err` on their error paths instead
of `data.NewList(nil, err), err`:

- `setLogger` (`util.SetLogger()`) was declared with only `[]*data.Type{data.BoolType}`
  -- no error -- despite genuinely failing on an unrecognized logger name or a
  non-bool `active` argument.
- `getLogContents` (`util.Log()`) was declared with only
  `[]*data.Type{data.ArrayType(data.StringType)}` -- no error -- despite genuinely
  failing on a non-integer `count`/`session` argument or an underlying `ui.Tail` error.
- `getPackage` (`util.Package()`) was declared with only the result map type -- no
  error -- despite genuinely failing on an unknown package name.

Since dispatch only routes a `*data.List` result through the `(value, error)`
convention (see `callRuntimeFunction` dispatch mechanics in `CLAUDE.md`), a bare
non-list error on a non-error-typed return becomes an uncatchable-except-try/catch
runtime abort instead of a normal returned error -- e.g. `_, err :=
util.SetLogger("bogus", true)` crashed the whole program instead of setting `err`.

Separately, `util.SymbolTables()`'s declaration claimed a single `UtilSymbolTableType`
return, but its implementation (`formatTables`) always returns a `*data.Array` of
them -- see CALL-14, which fixes this alongside the missing `Scope: true` on the same
declaration.

**Fix:**  
All three declarations now include `data.ErrorType` in `Returns`, and all three
implementations wrap every return path (success and error) in `data.NewList`.
`getLogContents`'s previous "empty buffer" success path (a bare `[]any{}`) was also
changed to `data.NewArray(data.StringType, 0)`, consistent with its declared element
type.

Regression tests added to `internal/runtime/util/util_test.go`: existing
`TestSetLogger`/`TestGetLogContents` subtests were updated to unwrap the now-`data.List`
results via a new `unwrapValue` helper; a new `TestGetPackage` asserts both the success
and error paths of `getPackage` return a `data.List` (registering a synthetic package via
`packages.Save` rather than depending on a real package like `math` already being
registered in the test's process).

