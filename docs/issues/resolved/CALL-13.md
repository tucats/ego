# CALL-13 — `tables.Find()`'s callback can't see enclosing-scope symbols (missing `Scope: true`)

**Affected function:** `findRows` (declaration in `internal/runtime/tables/types.go`)  
**Files:** `internal/runtime/tables/types.go`, `internal/runtime/tables/find.go`  
**Risk:** High when triggered — any package reference (e.g. `strconv.Atoi`) inside a
`Find()` comparator closure fails outright with `"unknown identifier: strconv"`, but only
when the `Find()` call itself is made at a shallow enough scope depth (a bare top-level
statement, as used by `ego run < file.ego`'s stdin path) — the identical code works fine
one scope level deeper (inside a function body, or an Ego test's own `@test{}` block),
which is what let this ship unnoticed  
**Discovered by:** manual testing while writing `tables` package documentation examples
(the `Find()` example from `docs/LANGUAGE.md`, using `strconv.Atoi` inside the closure,
silently returned an empty result with no error visible via single-value assignment)  
**Status: RESOLVED**

## CALL-13: Description

Every other runtime function that invokes a user-supplied Ego callback — `sort.Slice`,
`sort.SliceStable`, `sort.Search`, `os.Expand`, `strings.Template` — sets `Scope: true`
on its `data.Declaration`, per the documented convention: "The function's declaration
entry in `types.go` must set `Scope: true` so the Ego closure can access variables from
its enclosing scope." `tables.Find`'s declaration was missing this flag.

`callRuntimeFunction` computes the parent for a call's own symbol table as follows:

```go
if fullScope {
    parentTable = c.symbols
} else {
    parentTable = c.symbols.FindNextScope()
}
```

`fullScope` is set from `definition.FullScope`, which is populated from the
`Declaration.Scope` field. Without it, `findRows`'s own symbol table (and the child table
it builds for the callback's `bytecode.Context`) is parented via `FindNextScope()` instead
of `c.symbols` directly — the same scope-skipping mechanism responsible for CALL-12,
this time affecting a normal symbol lookup (an imported package) rather than the sandbox
flags. At a shallow enough call depth, this skips right past the table where the
`strconv` package import is registered, so any reference to it inside the closure fails:

```go
t, _ := tables.New("Name", "Age")
t.AddRow("Tom", 55)

rows, err := t.Find(func(name string, age string) bool {
    i, e := strconv.Atoi(age)
    return e == nil && i > 50
})

fmt.Println(rows, err)
// []  at <anon>(line 4), unknown identifier: strconv   -- when Find() is called
//                                                          at bare top-level scope
```

The single-value form used in `docs/LANGUAGE.md`'s original example (`retirees :=
t.Find(...)`, discarding the error) made this look like a silent, wrong *result* (an
empty array) rather than a visible error, which is how it was first noticed.

## CALL-13: Fix

Added `Scope: true` to `Find`'s `data.Declaration` in `types.go`, matching the other
five callback-invoking functions listed above.

**Regression test:** reproducing the exact scope-depth condition isn't possible through
the normal Ego test harness — `ego test`'s `@test{}` block wrapping (and any function
body) is already one scope level deeper than the bug requires, so a `tests/tables/*.ego`
test can't fail on this even with the flag removed (confirmed by testing). Added
`TestFindDeclarationHasScopeTrue` (`internal/runtime/tables/tables_test.go`) as a direct
metadata check instead, asserting `Declaration.Scope == true` — verified it fails when
the flag is manually removed and passes with it restored.
