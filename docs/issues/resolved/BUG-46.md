# BUG-46 — A typed array silently degrades to `[]interface{}` on an out-of-type index assignment

**Severity:** MEDIUM

**Description:**  
In dynamic mode, assigning a value of the wrong type to an element of a typed array (e.g.
storing a `string` into an `[]int`) succeeds silently and converts the array's declared
type from `[]int` to `[]interface{}`, rather than erroring — even though `append()` (after
the BUG-15 fix) and map value assignment both correctly error for the analogous case in the
same mode. `strict`/`relaxed` type modes correctly reject this; only `dynamic` mode has the
gap.

**Reproducer:**

```go
func main() {
    a := []int{1, 2, 3}
    fmt.Println("before:", typeof(a))
    a[1] = "hello"
    fmt.Println("after:", typeof(a), a)
}
```

**Actual output:**

```text
before: []int
after: []interface{} [int(1), "hello", int(3)]
```

**Expected output:**

Consistent with map value assignment (`docs/LANGUAGE.md` line 372: assigning a wrong-typed
value to a typed map produces `Error: wrong map value type: ...`), a typed array should
either reject the assignment or coerce the value, not silently widen the whole array's
declared type.

**Notes:**  
`docs/LANGUAGE.md` lines 246-256 describe per-mode behavior only for *array-literal
initialization*, not for later index assignment, so this is an undocumented inconsistency
between two structurally similar constructs (typed arrays vs. typed maps) rather than a
contradiction of an explicit doc statement.

**Resolution (July 2026):**

**Root cause:** `storeInArray` (`internal/language/bytecode/structs.go`) compares the Go
type of the existing element at the target index against the new value's type, and only
took action on a mismatch via a three-way `switch` on the type-checking mode:

- `strict`: rejected immediately with `ErrInvalidVarType`.
- `relaxed`: attempted `data.Coerce(v, vv)` and returned its error if coercion failed.
- `dynamic` (the default, `NoTypeEnforcement`): called `array.MakeAny()`, converting the
  array's declared element type to `interface{}` and always allowing the write — the exact
  behavior BUG-15 had already fixed for `append()`.

**Fix:** dynamic mode now shares the relaxed-mode code path: it attempts to coerce the
value to the array's existing element type and returns the coercion error if that fails,
instead of ever calling `array.MakeAny()`. This mirrors the BUG-15 fix in
`internal/builtins/append.go` (`strict: reject`, `relaxed`/`dynamic`: `coerce or error`) and
is consistent with `data.Map.Set`, which already rejects a wrong-typed value in every mode.
An array declared as `[]interface{}` is excluded from the check entirely (in every mode),
also matching `append()` — this exclusion was **not** present before the fix (the pre-fix
`dynamic` path's `MakeAny()` call happened to be a no-op for an already-interface array, so
the gap was latent rather than user-visible; `strict`/`relaxed` mode assigning a new type
into an existing element of a `[]interface{}` array would previously have been incorrectly
rejected/coerced too).

**Scope creep (explicitly approved):** writing Ego-language tests for this fix needed to
branch on the ambient type-checking mode from within a test body, by reading the invisible
`__type_checking` runtime global (see `internal/language/bytecode/types.go`,
`staticTypingByteCode`). A single, one-time read of that global failed to compile with
`"variable created but never used: __type_checking"`. Root cause:
`resolveExternalSymbol` (`internal/language/compiler/symbols.go`) is reached whenever a
symbol reference can't be resolved in any compiler scope, for two different reasons that
both leave its `mustExist` parameter `false`:

1. `ReferenceOrDefineSymbol` (only caller: the simple-lvalue path in `lvalue.go`, e.g.
   compiling `neverUsed := 42`) starts `mustExist` as `false` deliberately — the name may
   not exist yet, and this call *is* the declaration.
2. `validateSymbol` forces `mustExist` to `false` for names the compiler can never see
   declared at all: generated (`$`-prefixed) names, invisible (`__`-prefixed) runtime
   globals such as `__type_checking`, trial compilations, and server mode. This call *is* a
   read of something already known to exist at runtime even though no compile-time scope
   ever declared it.

Both cases funneled into the same unconditional `c.DefineSymbol(name)` call, which always
registers a fresh "declared but not yet used" sentinel, expecting a later reference to the
same name to clear it. That is correct for case 1, but wrong for case 2: a case-2 name read
exactly once in the whole compilation unit has no second reference to clear the sentinel, so
it was always reported unused. Fixed by having `validateSymbol` capture
`allowImplicitDefine` (whether its own `mustExist` argument was already `false` on entry,
i.e. case 1) before applying the case-2 exemptions, and threading it through to
`resolveExternalSymbol`, which now calls `c.DefineSymbol` (fresh declaration) only when
`allowImplicitDefine` is true, and a new `c.markSymbolAsUsed` helper (marks the name used
immediately, no sentinel) otherwise.

**Tests:** Go unit tests in `internal/language/bytecode/structs_test.go` (`storeInArray`,
covering dynamic/relaxed/strict mode across `int`, `float64`, `bool`, `string`, `byte`, and
`interface{}` element types) and `internal/language/compiler/symbols_test.go` (the
`__type_checking` unused-variable fix, plus a regression guard confirming a genuinely
unused `:=` local is still reported). Ego-language tests in
`tests/types/array_index_type_check.ego`, run and passing under `ego test`, `--types
strict`, and `--types relaxed`, alongside the full `tests/` and `go test ./...` suites in
all three modes.

