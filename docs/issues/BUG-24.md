# BUG-24 — Multi-target assignment lists reject indexed/member lvalues (`m[k], arr[i] = ...`)

**Severity:** MEDIUM

**Description:**  
A comma-separated list of assignment targets only works correctly when every target
is a simple, unqualified variable name (`a, b, c = ...`). As soon as any target in the
list is an indexed or member expression (`m["k"]`, `arr[0]`, `s.field`), the assignment
fails at runtime with `invalid or unsupported data type for this operation`. This is
pre-existing behavior in `assignmentTargetList` (`internal/language/compiler/lvalue.go`)
found while adding Go-style parallel assignment support (`a, b, c = 10, 20, 30`); it
reproduces even on the multi-return-call form that has been supported for a long time,
so it is not a regression from that new feature.

**Reproducer:**

```go
func pair() (int, int) {
    return 5, 6
}

func main() {
    m := map[string]int{}
    arr := []int{0, 0}

    m["k"], arr[0] = pair()
    fmt.Println(m["k"], arr[0])
}
```

**Actual output:**

```text
Error: at main(line 8), invalid or unsupported data type for this operation
```

**Expected output:**

```text
5 6
```

**Notes:**  
Single-target indexed/member assignment (`m["k"] = 5`) works fine on its own; the bug
is specific to mixing indexed/member targets into a *multi-target* list. The same
failure occurs with a literal expression list (`m["k"], arr[0] = 5, 6`), with the
two-value form of a multi-return call, and regardless of whether `:=` or `=` is used.
A workaround is to assign through a temporary simple variable and then copy it into the
indexed/member target on its own line.

**Root cause — three compounding bugs, all in the multi-target lvalue machinery:**

1. **`assignmentTargetList`** (`internal/language/compiler/lvalue.go`) unconditionally
   emitted a `SymbolOptCreate` instruction for *every* target in the list, compound or
   not, immediately before calling `patchStore`. `patchStore` decides whether to convert
   a compound lvalue's trailing `LoadIndex` into `StoreIndex` by checking whether the
   *last instruction emitted so far* is exactly that `LoadIndex` — the unconditional
   `SymbolOptCreate` landing in between made that check fail every time, so `patchStore`
   silently fell back to an ordinary `Store` on the base variable's own name (`m`)
   instead of storing into the map/array/struct element at all. This is what produced
   the reported `invalid or unsupported data type for this operation` error — the store
   was writing an interface value where the expression evaluator (running the *next*
   statement) expected something else.

2. Fixing (1) alone was not sufficient: `storeIndexByteCode`
   (`internal/language/bytecode/structs.go`) pushes its container back onto the stack
   after a successful write — `storeInMap`, `storeInArray`, and the `*data.Struct`/
   `*any`-wrapped-struct branches all do this unconditionally. For a *single*-target
   assignment that leftover value is harmless: the "let" stack marker pushed before the
   lvalue code runs, and its `DropToMarker` at the very end, cleans up any stray items in
   one shot regardless of how many accumulated. But within a multi-target *list*, the
   next target's real value needs to be sitting exactly where that leftover container
   ended up — so the container was silently consumed as the *next* target's "value"
   instead of the real one. For the reproducer above, this meant `arr[0]` ended up
   holding the whole map `m`, not `pair()`'s second return value.

3. Fixing (1) and (2) together correctly handles a list where compound targets are
   *mixed* with simple ones, but exposed a third, independent bug for a list made up of
   compound targets *only*: `ByteCode.StoreCount()` (`internal/language/bytecode/
   bytecode.go`) — used by `compileAssignment`'s `multipleTargets` detection,
   `compileUnwrap`'s two-value-assertion detection, and
   `compileParallelAssignment`'s RHS-count validation — only counted `Store`/
   `CreateAndStore` instructions, never `StoreIndex`. Once (1) started correctly
   emitting `StoreIndex` for compound targets, a list with zero simple names in it (e.g.
   `m["p"], m2["q"] = 100, 200`) undercounted as having **zero** targets, which broke
   all three of those checks — most visibly as `assignment mismatch: 0 variables but 2
   values` for the literal-RHS-list form.

**Resolution:**  
All three are fixed:

1. `assignmentTargetList`'s loop now only emits `SymbolOptCreate` (and the accompanying
   `ReferenceOrDefineSymbol` call) when the target turned out to be a genuinely simple
   name — detected via the same `needLoad` flag the loop already used to decide whether
   the `.field`/`[index]` suffix loop ran at all. A compound target's base variable must
   already exist (you cannot introduce `m` via `m["k"] := 5`) and was already `Load`'ed
   and `ReferenceSymbol`'d inside that suffix loop, so nothing further was needed for it
   there in the first place.
2. The same loop now emits an explicit `Drop 1` immediately after `patchStore` for any
   compound target, discarding the leftover container `StoreIndex` pushes back before
   moving on to the next target, so it can never be mistaken for that target's value.
3. `ByteCode.EmitAt`'s store-counting condition (`internal/language/bytecode/
   bytecode.go`) now also counts `StoreIndex`, alongside the pre-existing `Store`/
   `CreateAndStore`, so `StoreCount()` accurately reflects the number of lvalue targets
   regardless of whether they're simple, compound, or a mix of both.

**Regression tests:** 9 new `@test` blocks added to `tests/datamodel/parallel_assignment.ego`
(which already carried Go-style parallel assignment coverage, with this bug noted as a
known limitation at the top of the file — that note is now updated to reflect the fix):
a map+array target pair fed by a multi-return call, a literal (non-call) RHS list, the
`:=` form, struct field targets, a list mixing simple and compound targets, a list with
*only* compound targets (root cause 3's specific trigger), a nested (multi-level)
compound target, and two explicit regression checks confirming simple-target-only lists
and single-target compound assignments are unaffected. Verified against `go build
./...`, `go vet ./...`, `go test ./...`, and `ego test tests/` under `--types dynamic`,
`--types strict`, and `--types relaxed` (1422 `@test` blocks, up from 1413, with no
regressions).

