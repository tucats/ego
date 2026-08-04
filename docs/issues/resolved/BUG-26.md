# BUG-26 — Struct assignment and pass-by-value alias instead of copying

**Severity:** CRITICAL  
**Status:** Fixed

**Description:**  
Assigning one struct variable to another (`p2 := p1`), or passing a struct to a function
parameter, does not copy the struct — both variables (or the parameter and the argument)
end up referring to the same underlying struct instance, so mutating one silently mutates
the other. This reproduces for `:=`, `=`, and function-parameter passing; for both named
and anonymous struct types; and in dynamic, strict, and relaxed type modes. This is the
same class of defect as the already-fixed BUG-23 (`var` zero-value structs sharing a single
compile-time instance), but is a much broader, still-open gap: BUG-23's fix does not cover
ordinary struct-to-struct assignment or function-argument binding.

**Reproducer:**

```go
type Point struct {
    X int
    Y int
}

func mutate(p Point) {
    p.X = 555
}

func main() {
    p1 := Point{X: 1, Y: 2}
    p2 := p1
    p2.X = 99
    fmt.Println("after p2.X=99: p1 =", p1, " p2 =", p2)

    mutate(p1)
    fmt.Println("after mutate(p1) [pass by value]: p1 =", p1)
}
```

**Actual output:**

```text
after p2.X=99: p1 = Point{ X: 99, Y: 2 }  p2 = Point{ X: 99, Y: 2 }
after mutate(p1) [pass by value]: p1 = Point{ X: 555, Y: 2 }
```

**Expected output** (verified against real Go):

```text
after p2.X=99: p1 = {1 2}  p2 = {99 2}
after mutate(p1) [pass by value]: p1 = {1 2}
```

**Notes:**  
`docs/LANGUAGE.md` line 495 states that a value argument "gets a copy made and that copy is
what is passed to the function," matching standard Go struct value semantics — this is
violated for both assignment and argument passing. Root cause: `internal/language/bytecode/store.go`
(`storeByteCode`) and `internal/language/bytecode/structs.go` (`storeIndexByteCode`) store
the `*data.Struct` pointer directly, with no call to `Struct.Copy()`
(`internal/language/data/structs.go:574`) — that method is only invoked by the explicit
`DeepCopy`/`copy()`-builtin paths, never by ordinary assignment or argument binding. Array
and map reference-alias semantics are correct and intentional (they match Go slice/map
behavior); this issue is specific to structs, which in Go are value types.

**Fix:**  
A new helper, `copyStructForValueSemantics(value any) any`, was added to
`internal/language/bytecode/store.go`. It returns `value` unchanged unless it is a
`*data.Struct`, in which case it returns an independent copy via a new
`copyStructRecursive()` helper (see below). It is now called at every point where a value is
bound to a new name or field:

1. `storeByteCode` (the `Store` opcode — plain `p2 = p1` assignment)
2. `createAndStoreByteCode` (the `CreateAndStore` opcode — `p2 := p1` short declaration)
3. `argByteCode` (the `Arg` opcode — binding a function argument to its parameter, i.e.
   `mutate(p1)`)
4. `storeIndexByteCode`'s two `*data.Struct` cases (the `StoreIndex` opcode — struct-field
   assignment, `outer.field = someStruct`, including the pointer-to-interface variant)

Method receiver binding (`getThisByteCode` in `this.go`) is a *different* opcode path and was
not touched — it already had correct, independent handling for both pointer and value
receivers (see FUNC-M2), so no change was needed there.

**Nested structs are also copied, not just the top level:**  
`Struct.Copy()` only duplicates one level: it allocates a new struct with a new fields map,
but any nested `*data.Struct` field value in that map is still the very same pointer as the
original. Without an extra step, `b2 := b1` for a struct `b1` containing a nested struct
field would stop `b1` and `b2` from sharing their top-level fields, yet a write to
`b2.Nested.X` would still be visible through `b1.Nested.X`, because both would still point at
one shared inner struct. `copyStructRecursive()` walks every field (via `FieldNames(true)`,
which includes private fields) and, for any field that is itself a `*data.Struct`, replaces
it with a recursive copy — exactly mirroring Go's own struct-copy semantics, where nested
struct fields are copied recursively but a slice/map/pointer/channel field only has its
lightweight header copied, leaving the underlying data shared. Verified against real Go with
a `Box{ Origin Point; Label string }` case, and separately verified that a struct field which
is itself a slice or map keeps sharing its backing storage after a struct-level copy (also
matching Go).

**Related fix discovered while testing — `Struct.Copy()` dropped `"__"`-prefixed fields:**  
`Struct.Copy()` built its result via `NewStructFromMap(s.fields)`. `NewStructFromMap()` is
designed to build a struct from a generic `map[string]any` that may have embedded
type-metadata keys mixed in with the real fields (as happens reconstructing a struct from
JSON-like data), so it deliberately skips any key starting with `MetadataPrefix` (`"__"`).
That filtering is wrong when the input is already a struct's own field map: `NativeFieldName`
(`"__native"`) — used by packages like `time`, `uuid`, and `sync` to store the wrapped
Go-native value (a `*uuid.UUID`, `*time.Time`, `*sync.Mutex`, ...) — shares that same prefix,
so it was silently dropped by every call to `Copy()`. This went unnoticed while `Copy()` had
only a couple of narrow callers (`$new()`, the unused-in-practice `DeepCopy` struct case), but
broke `tests/packages/uuid.ego` immediately once ordinary assignment started calling `Copy()`
for every struct. `Struct.Copy()` (`internal/language/data/structs.go`) no longer routes
through `NewStructFromMap`; it now copies `s.fields` key-by-key with no filtering, so every
field — `"__"`-prefixed or not — survives the copy.

**Files changed:**

- `internal/language/bytecode/store.go` — added `copyStructForValueSemantics()` and
  `copyStructRecursive()`; called from `storeByteCode`
- `internal/language/bytecode/symbols.go` — called from `createAndStoreByteCode`
- `internal/language/bytecode/arg.go` — called from `argByteCode`
- `internal/language/bytecode/structs.go` — called from both `*data.Struct` cases in
  `storeIndexByteCode`
- `internal/language/data/structs.go` — rewrote `Struct.Copy()` to preserve every field,
  including `"__"`-prefixed ones like `NativeFieldName`
- `internal/language/compiler/struct_value_semantics_test.go` — new Go unit tests: the
  BUG-26 reproducer for `:=`, `=`, and function-argument passing; anonymous struct
  assignment; struct-field assignment; nested struct-in-struct copying; regression guards
  confirming arrays/maps still alias (both at the top level and as struct fields); and a
  guard confirming pointer/value receiver semantics are unaffected
- `internal/language/data/struct_copy_test.go` — new Go unit tests for `Struct.Copy()`:
  preserves the native field, preserves ordinary fields, and produces an independent copy
- `tests/types/struct_value_semantics.ego` — new Ego-level regression tests covering the
  same scenarios end-to-end

**Second follow-up regression — REST service handlers lost `ResponseWriter` mutations:**  
After this fix shipped, a live REST server trace (`GET /services/factor/bob`, which should
400 on a non-numeric `value`) showed the client receiving a `200` status with the correct
error body text instead of `400`. Root cause: the `@handler` compiler directive
(`internal/language/compiler/directives.go:handlerDirective`) invokes a service's `handler`
function by loading the REST server's live `ResponseWriter` object directly out of a reserved
symbol (`_response_writer`) and passing it as a call argument — the value that arrives is a
raw `*data.Struct`, never wrapped in the `*any` indirection that Ego's own `&x` operator
produces, even when the handler parameter is declared with a pointer type such as
`w *http.ResponseWriter`. `argByteCode`'s blanket "an argument that is a `*data.Struct` gets
copied" rule (the BUG-26 fix itself) copied this value like any other struct argument, so the
handler's local `w` became an independent copy: `w.WriteHeader(400)` and
`w.Header().Add(...)` inside the handler mutated only the copy, never reaching the real
`ResponseWriter` the REST server reads back from after the handler returns. (`w.Write(...)`
still appeared to work because the response body lives in a `*data.Array` field, which
`copyStructRecursive` intentionally leaves shared — only the struct-typed `_status` and
`_headers` state was lost.)

Fix: `argByteCode` (`internal/language/bytecode/arg.go`) now checks the argument's *declared*
parameter type (`argType.IsPointer()`, available whenever the parameter has any type
annotation) before copying. When the declared type is a pointer, the argument is bound
exactly as received, regardless of its actual runtime Go type — matching real Go, where a
`*T` parameter is never implicitly copied, and covering this native-injection case where
"pointer" exists only in Ego's type system, not as an actual extra layer of Go indirection
around the value.

While tracking this down, the same class of mis-declaration was found in six other bundled
service handlers under `lib/services/`, all written with the plain (non-pointer)
`w http.ResponseWriter` form instead of `w *http.ResponseWriter`: `sample.ego`,
`count.ego`, `up.ego`, `hello.ego`, `bogus-compile.ego`, `bogus-runtime.ego`, and
`lib/services/admin/{apple,debug,memory}.ego`. Every one of these was silently losing
`WriteHeader`/`Header().Add()` calls the same way (confirmed for `sample.ego` via
`tools/apitest`: `services-24-sample-404.json` expected a `404` for an unknown user and
got `200` until the parameter was corrected to `w *http.ResponseWriter`). All were corrected
to the pointer form.

**Additional files changed:**

- `internal/language/bytecode/arg.go` — `argByteCode` skips the struct copy when
  `argType.IsPointer()` is true
- `internal/language/compiler/struct_value_semantics_test.go` — added
  `TestBUG26PointerTypedParameterNotCopied`, which injects a raw `*data.Struct` into the
  symbol table (mirroring `_response_writer`) and passes it to a pointer-typed parameter,
  confirming the callee's field write is visible through the original afterward
- `lib/services/sample.ego`, `count.ego`, `up.ego`, `hello.ego`, `bogus-compile.ego`,
  `bogus-runtime.ego`, `admin/apple.ego`, `admin/debug.ego`, `admin/memory.ego` — corrected
  the `handler` function's `ResponseWriter` parameter to the pointer form
- Verified live: rebuilt the server, exercised `GET /services/factor/{bob,12,""}` with
  `curl` (correct `400`/`200`/`400` results and headers both before finding, and after
  fixing, this regression), and ran `tools/apitest` against `tests/1-logon` and
  `tests/3-services` (all pass, including `services-24-sample-404.json`, which required an
  `ego flush cache` after editing `sample.ego` since the REST server's `ServiceCache` holds
  compiled service bytecode indefinitely with no source-file mtime check)

