# BUG-33 — Struct field type declarations are never enforced, even in strict mode

**Severity:** HIGH

**Description:**  
A struct field declared with an explicit type (e.g. `Age int`) silently accepts a value of
a completely different, non-coercible type (e.g. a string) both at construction and on
later assignment — in dynamic mode *and* in `--types strict` mode. This is inconsistent
with typed arrays and maps, both of which do enforce element/value types.

**Reproducer:**

```go
type Employee struct {
    Name string
    Age  int
}

func main() {
    e := Employee{Name: "Robin", Age: 30}
    e.Age = "old"
    fmt.Println(typeof(e.Age), e)
}
```

**Actual output** (both `./ego run` and `./ego run --types strict`):

```text
string Employee{ Name: "Robin", Age: "old" }
```

**Expected output:**

In dynamic mode, at minimum a coercible value (e.g. `"42"`) should be converted to `int`,
matching how map values are type-checked/coerced on assignment. In strict mode, this should
be a compile- or run-time type error, matching the enforcement already applied to typed
arrays and maps.

**Notes:**  
Root cause: `internal/language/bytecode/structs.go` (`storeIndexByteCode`, case
`*data.Struct`) calls `a.Set(key, v)` with no type check at all — contrast with the sibling
`storeInArray` helper a few lines below it, which explicitly checks `c.typeStrictness`.
`data.Struct.Set` (`internal/language/data/structs.go:514`) does have a coercion block
gated on `s.typeDef.fields != nil`, but that field map does not appear to be populated for
structs built from named-type literals, so the coercion path never fires for this
construction pattern.

**Resolution (July 2026):**  
Two separate defects had to be fixed:

1. **`data.Struct.Set` looked at the wrong `*Type`.** `internal/language/data/structs.go`
   (`Struct.Set`, around line 545): for a struct created from an anonymous struct literal,
   `s.typeDef` *is* the struct-kind `*Type` and its `fields` map is populated directly. But
   for a struct created from a `type Employee struct {...}` declaration, `s.typeDef` is a
   `TypeKind` *wrapper* around the real struct type (see `data.TypeDefinition`), and that
   wrapper's own `fields` map is always `nil` — the field definitions live on the wrapped
   type, reachable via `s.typeDef.BaseType()`. The old code read `s.typeDef.fields`
   directly, so the lookup silently found nothing for every struct built from a named type
   (by far the more common case), and neither the strict-mode check nor the coercion
   attempt ever ran. The fix resolves through `s.typeDef.BaseType().fields` instead, so both
   shapes now find their declared field types. While in there, a second, subtler bug was
   fixed in the same block: on a failed `Coerce` (e.g. assigning `"old"` to an `int` field),
   the old code still fell through to `s.fields[name] = value`, zeroing the field even
   though an error was also returned. The fix returns immediately on a `Coerce` error, so a
   caller that recovers via `try/catch` finds the field unchanged rather than reset to the
   zero value of its declared type.

2. **Strict mode was never actually wired up.** `internal/language/bytecode/structs.go`
   (`storeIndexByteCode`, both the direct `*data.Struct` case and the `*any`-wrapped case)
   now calls a new helper, `checkStructFieldStrictType`, before writing the field. In strict
   mode (`--types strict` / `@type strict`) it looks up the field's declared type via
   `s.Type().BaseType().Field(key)` and rejects a type mismatch outright with
   `ErrInvalidType` — no coercion attempted, even for an otherwise-convertible value like the
   string `"42"` — matching the enforcement already applied to typed arrays and maps. In
   relaxed/dynamic mode the helper is a no-op; `data.Struct.Set` (fixed above) handles
   coercion itself. An earlier version of this fix recorded the strictness setting directly
   on the `*data.Struct` instance via the (until-then-unused) `SetStrictTypeChecks` method,
   but that flag is a private field of `data.Struct` and therefore part of what
   `reflect.DeepEqual` compares for struct equality (`data.Equals`) — two otherwise-identical
   structs could compare as unequal merely because one of them had a field assigned to it
   while running in strict mode. `checkStructFieldStrictType` reads `c.typeStrictness`
   directly instead, so no persistent state is added to the struct.

With both fixes, the original reproducer now behaves as documented: `e.Age = "old"` raises
a catchable runtime error (`invalid integer value: old`) in every mode, `e.Age = "42"`
converts to `int(42)` in dynamic/relaxed mode, and `e.Age = "42"` is rejected with
`ErrInvalidType` under `--types strict`. Verified with new Go tests in
`internal/language/data/structs_test.go` and `internal/language/bytecode/structs_test.go`,
new Ego tests in `tests/types/struct_ops.ego`, and a full run of `tools/gotests.sh` plus
`ego test` under all three typing modes (strict/relaxed/dynamic).

