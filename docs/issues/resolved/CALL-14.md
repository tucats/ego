# CALL-14 — `util.Symbols()`/`util.SymbolTables()` can't see the caller's own local variables (missing `Scope: true`)

**Affected functions:** `formatSymbols`, `formatTables` (declarations in
`internal/runtime/util/types.go`)  
**Files:** `internal/runtime/util/types.go`, `internal/runtime/util/symbols.go`  
**Risk:** High when triggered — the caller's own local variables (and, for
`SymbolTables()`, the caller's own table) are silently missing from the report, with
no error of any kind; only manifests at a specific shallow call depth (a bare
top-level statement, as used by `ego run < file.ego`'s stdin path), which is why it
shipped unnoticed — the identical call one scope level deeper (inside a function
body) is unaffected  
**Discovered by:** manual testing while writing `util` package documentation examples
(`x := 5; y := "hello"; fmt.Println(util.Symbols())` never showed `x` or `y` at any
scope level in the report)  
**Status: RESOLVED**

## CALL-14: Description

Same root cause and mechanism as CALL-13, this time affecting the two functions whose
entire purpose is to introspect the caller's own live scope chain rather than a nested
callback closure. Neither `Symbols` nor `SymbolTables`'s `data.Declaration` set
`Scope: true`, so `callRuntimeFunction` parented their call's own symbol table via:

```go
if fullScope {
    parentTable = c.symbols
} else {
    parentTable = c.symbols.FindNextScope()
}
```

`fullScope` comes from `Declaration.Scope`; without it, `FindNextScope()` returns the
*parent* of `c.symbols` for a non-boundary scope, i.e. it always skips exactly one
level -- the caller's own immediate block scope, which is precisely the scope holding
the caller's own local variables (and, for `SymbolTables()`, the caller's own table
entry). `formatSymbols` and `formatTables` both walk outward from the symbol table
they're given (`s.Parent()` for `formatTables`; `s` itself, then `.Parent()`
repeatedly, for `formatSymbols`), so once that one level is skipped it's gone from
the walk for good -- not merely reordered.

**Reproducer (before the fix):**

```go
x := 5
y := "hello"
fmt.Println(util.Symbols())
// "x" and "y" never appear in the report, at any scope level
```

```go
tbls := util.SymbolTables()
fmt.Println(tbls[0].name)
// omits the caller's own immediate table entirely -- the walk starts one level
// too high, e.g. skipping straight to a *file table when it should start there
```

## CALL-14: Fix

Added `Scope: true` to both `Symbols` and `SymbolTables`'s `data.Declaration` in
`types.go`, matching the convention already used by `sort.Slice`, `os.Expand`,
`tables.Find` (CALL-13), and the other callback-invoking functions. While auditing
`SymbolTables`'s declaration, also found and fixed a second, unrelated inaccuracy: its
`Returns` claimed a single `UtilSymbolTableType`, but the implementation
(`formatTables`) always returns a `*data.Array` of them -- corrected to
`data.ArrayType(UtilSymbolTableType)`.

**Regression test:** same harness limitation as CALL-13 -- `ego test`'s `@test{}`
wrapping (and any additional nesting, e.g. `@capture`) is already deep enough that the
one skipped level either doesn't matter or lands past where the variable of interest
lives, so a `tests/util/*.ego` test cannot be made to fail on this even with the flag
removed (confirmed by testing). Added `TestSymbolsAndSymbolTablesDeclareScopeTrue`
(`internal/runtime/util/util_test.go`) as a direct metadata check instead, asserting
`Declaration.Scope == true` for both functions -- verified it fails when either flag is
manually removed and passes with both restored. `tests/util/util.ego` still exercises
both functions' actual reporting behavior (at the scope depth `ego test` provides) as
general behavioral coverage, alongside the rest of the `util` package.
