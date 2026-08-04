# BUG-76 — A struct field named the same as an unrelated type corrupts type registration

**Severity:** LOW

**Description:**  
Found while writing regression tests for the BUG-73 fix. A struct field whose name happens to
match the name of a completely unrelated `type` declared elsewhere in the same compilation
(a different `@test` block in the same file, in the reproducer below) fails to compile — even
though the two are otherwise unconnected: different type name, different fields, no embedding
or reference between them.

**Reproducer:**

```go
package main

type box struct {
    ch chan
}

func main() {
    b := box{ch: make(chan, 1)}
    _ = b

    type inner struct {
        ch chan
    }

    type outer struct {
        box inner // the field name "box" collides with the unrelated "type box" above
    }

    o := outer{box: inner{ch: make(chan, 1)}}
    _ = o
}
```

**Actual output:**

```text
INTERNAL: Duplicate field name ch
Error: invalid field name for type: box
Error: terminated with errors
```

**Expected output:**

No error — `outer`'s field named `box` has nothing to do with the unrelated top-level `type
box` and should not interact with it at all.

**Notes:**  
Confirmed present on the pre-BUG-73 baseline, so this is unrelated to that fix or to channels
specifically — a minimal reproducer without any channel involved at all (two plain `int`-typed
structs, same field/type name collision) produces a different but related error, `no such
type`, at the point the colliding field is used as a struct-literal key. Both symptoms point to
some shared namespace between *type names* and *struct field names* within a single
compilation unit — as if field name resolution falls back to (or is confused with) type name
resolution when the two strings coincide.

**Root cause:**  
`parseStructFieldTypes` (`internal/language/compiler/typeCompiler.go`) reads a struct field
entry's leading identifier and, if that identifier happens to match a name already in `c.types`
(the compiler's flat type-name registry), unconditionally assumed it was Go's anonymous
*embedded field* form — a field entry that is just a bare type name, with no separate field
name of its own (e.g. `type Outer struct { Inner }`). It never checked what actually followed
that identifier before committing to that assumption. So `box inner` — field named `box`, of
type `inner` — was misread as "embed the unrelated type `box`", and the `inner` token that
should have been consumed as `box`'s field type was left for the next loop iteration to
(mis)handle as if it were the start of an entirely different field. The real field the source
declared (`box` of type `inner`) was never added to the struct's type at all, so a struct
literal built from it later failed with "invalid field name for type: box" — the field the
literal referenced had simply never been registered.

**Resolution:**  
The fix resolves the ambiguity the same way a hand-written recursive-descent parser normally
would: by trying the more specific interpretation first and checking whether it actually works,
rather than guessing from a fixed-width token lookahead. When the leading identifier matches a
known type, `parseStructFieldTypes` now marks the tokenizer position (`c.t.Mark()`) and
speculatively calls `c.parseType("", false)` on whatever follows. Two outcomes:

- **The speculative parse succeeds** — what follows really is a valid type, so the identifier
  was an ordinary field *name* (`box inner`, `box`'s type being `inner`). The tokenizer position
  is rolled back (`c.t.Set(mark)`) and control falls through to the existing named-field path,
  which parses and consumes the name/type pair for real.
- **The speculative parse fails** — most commonly because what follows isn't a type at all, but
  the *next* field's own name (e.g. `Inner` embedded, then `other int` as an unrelated,
  independent field on the next line). The tokenizer position is still rolled back (the failed
  attempt must not leave partially-consumed tokens behind), and the identifier is treated as a
  solo embedded-type reference, exactly as before.

A one-token lookahead (checking only whether the *very next* token looks like the start of a
type) was tried first and rejected: it correctly fixed the reported collision case, but broke
the common, legitimate case of an embedded field immediately followed by another named field
(`Inner` then `other int`) — `other` is itself a plain identifier and satisfies "looks like a
type" just as readily as a real type name does, so a single-token lookahead cannot tell "this
identifier is my type" apart from "this identifier is the next field's name." Only actually
attempting the parse (and rolling back regardless of outcome, via the tokenizer's existing
`Mark`/`Set` "lexical recovery" mechanism used elsewhere in the compiler for the same kind of
ambiguity) disambiguates correctly in both directions.

**Known residual limitation:** a field name that collides with a known type *and* is written as
part of a comma-separated, shared-type name list (`box, other int`, intending two ordinary
fields named `box` and `other`, both typed `int`) is still misread as an embed of `box` followed
by a second, malformed embed attempt at `other`. This is not a new regression — the pre-fix code
had exactly the same failure mode for this input — and is narrow enough (it requires a field
name colliding with a type name *and* being written first in a multi-name field list) that it
was left as an accepted, documented gap rather than expanded scope; the speculative-type-parse
approach doesn't naturally extend to it because the ambiguity there hinges on what happens after
an entire comma-separated name list, not after a single identifier.

**Regression tests:** a new file, `tests/types/struct_field_type_collision.ego`, with 5 `@test`
blocks covering the reported collision (plain and channel-valued field types), a genuine solo
embedded field, an embedded field followed by a separate named field, and chained
comma-separated embedded fields. The nested-struct-field test in
`tests/flow/channel_compound_lvalue.ego` (originally written for BUG-73, before this fix
existed) was also simplified to use a field named `inner` — colliding with its own field's type
name — in place of the `content` workaround the collision previously required. Verified against
`go build ./...`, `go vet ./...`, `go test ./...`, and `ego test tests/` under `--types dynamic`,
`--types strict`, and `--types relaxed` (1366 `@test` blocks, up from 1361, with no regressions).

