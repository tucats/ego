# BUG-81 — `NewStructFromMap`/`NewStructOfTypeFromMap` produce non-deterministic field order

**Severity:** MEDIUM

**Description:**  
Reported as a flaky test: `ego test tests/packages/tokenizer.ego` randomly failed its BUG-80
regression test (added moments earlier in the same session) on some invocations but not others,
with no source change in between. The user correctly suspected non-deterministic map traversal.

`data.NewStructFromMap` (`internal/language/data/structs.go`) built its type's field list with:

```go
for k, v := range m {
    t.DefineField(k, TypeOf(v))
}
```

`DefineField` appends to `t.fieldOrder` in call order (`t.fieldOrder = append(t.fieldOrder,
name)`), and this loop's call order came directly from ranging over `m`. Go deliberately
randomizes map iteration order on every single `range` statement execution — so `t.fieldOrder`
(and, since `NewStructFromMap` copies `fieldOrder: t.fieldOrder` onto the returned `*Struct`, the
struct's own field order too) ended up in a different, arbitrary order on every call. Anything
that displays a struct/type by field order — `reflect.Type()`, the type's own `String()`
(`structTypeString` in `types.go`) — showed a different field ordering depending on that call's
random iteration outcome.

This is more than a display quirk: `structTypeString`'s own fallback logic explicitly says "use
`fieldOrder` if set, otherwise sort alphabetically for determinism" — but because `DefineField`
unconditionally appends to `fieldOrder` on every call, `fieldOrder` was *never* actually empty for
a struct built this way, so the alphabetical fallback path was dead code for every single caller
of `NewStructFromMap`/`NewStructOfTypeFromMap` that didn't separately call `.SetFieldOrder()`
afterward (many callers throughout the runtime packages do call `.SetFieldOrder()` explicitly on
the resulting struct *value*, which masked the bug for those instances — but package-level `*Type`
variables built once via `data.TypeOf(data.NewStructFromMap(...))`, like
`strings.StringsTokenArrayType`, have no such per-call correction and kept whatever order was
assigned at that one-time package initialization, which could differ between separate process
launches).

**Reproducer (before the fix):**

```sh
for i in $(seq 1 20); do ego test tests/packages/tokenizer.ego; done
## Intermittently fails a `string(reflect.Type(...)) == "[]struct{kind string, spelling string}"`
## assertion with fields in the opposite order, on some fraction of runs.
```

**Fix:**  
Both `NewStructFromMap` and `NewStructOfTypeFromMap`'s `t == nil` path now collect the map's keys
into a slice, sort it with `sort.Strings`, and call `DefineField` in that sorted order — so
`fieldOrder` (and everything that displays by it) is always alphabetical and deterministic,
regardless of Go's map iteration randomization, unless a caller explicitly overrides it afterward
with `.SetFieldOrder()`.

Regression tests `TestNewStructFromMap_FieldOrderIsAlphabetical` and
`TestNewStructOfTypeFromMap_FieldOrderIsAlphabetical` (`internal/language/data/structs_test.go`)
build a struct from a ten-key (and five-key) map across 20 iterations each, asserting the
resulting `fieldOrder` is alphabetical every time — chosen to make a false pass by coincidental
random ordering vanishingly unlikely. Confirmed the fix resolves the original flaky reproducer:
20/20 clean `ego test tests/packages/tokenizer.ego` runs after the fix, versus 3/3 failures when
temporarily reverted.
