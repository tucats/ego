# BUG-41 — Multi-line nested struct/map literals fail to parse with "invalid list"

**Severity:** HIGH

**Description:**  
A struct/map literal containing a nested struct/map literal, formatted across multiple
lines, fails to compile with `invalid list`. The identical literal written on a single line
parses correctly, as does one level of nesting when everything stays on one line. This
directly breaks `docs/LANGUAGE.md`'s own `@localization` example, which is formatted
multi-line with nested per-language maps and fails to even compile.

**Reproducer:**

```go
func main() {
    x := {
        "a": { "b": "c" }
    }
    fmt.Println(x)
}
```

**Actual output:**

```text
Error: at line 3:17, invalid list
Error: terminated with errors
```

**Expected output:**

```text
{ a: { b: "c" } }
```

**Second reproducer** (`docs/LANGUAGE.md`'s own `@localization` example, verbatim structure):

```go
@localization {
    "en": {
        "hello.msg": "hello, {{Name}}"
    }
}
```

**Actual output:** `Error: at line 3:14, invalid list` — the documentation's own canonical
example fails to compile. The single-line equivalent
`@localization { "en": { "hello.msg": "hello there" } }` compiles fine.

**Notes:**  
Root cause appears to be in `internal/language/compiler/expr_atom.go` (`parseStruct`) —
after parsing a nested struct value that ends in `}` followed by a newline, the loop's
"next token must be comma or terminator" check fails, suggesting the tokenizer's automatic
newline/statement-termination handling inserts something between the inner closing brace
and the outer comma/brace that the nested-struct-literal parser does not tolerate.

**Resolution (July 2026):**  
Confirmed root cause: `internal/language/tokenizer/line.go` (`splitLines`) inserts a
synthetic `;` at the end of any source line whose last token is one that would end a Go
statement — this includes `}` and `]`, matching Go's own automatic semicolon insertion
rules (a Go source line ending in `}` really does get a semicolon inserted, per the Go
spec). The tokenizer has no awareness that such a line might be inside a brace/bracket-
enclosed *literal* (struct, map, or array) rather than a statement block, so whenever an
element of such a literal — most commonly a nested literal value — ends a source line, a
stray `;` is left between that element and the `,` or closing bracket the enclosing
literal's parser expects next.

(For the record: real Go has the same synthetic-`;`-after-`}` behavior, but Go's
composite-literal grammar treats it as a hard error unless every line ends in an explicit
trailing comma — this is the well-known "missing ',' before newline in composite literal"
gotcha that `gofmt` works around by always inserting one. Ego intentionally chose *not* to
require the trailing comma, per `docs/LANGUAGE.md`'s own multi-line examples, so the fix
makes the parser tolerant instead of requiring source changes.)

This exact class of bug affects every brace/bracket-enclosed literal-list parser, not just
nested struct/map literals — untyped and typed array literals (`[]int{...}`), typed map
literals (`map[K]V{...}`), and both struct-literal forms (ordered and named-field) all
reproduce the same "invalid list" failure when a multi-line element ends in `}` or `]`
with no trailing comma. Two separate families of parser were affected:

- `internal/language/compiler/expr_atom.go` — `parseStruct` (untyped struct/map literals)
  and `compileArrayInitializer` (untyped/bracket array literals).
- `internal/language/compiler/initializer.go` — `parseArrayInitializer`,
  `parseMapInitializer`, `structInitializeByOrderedList`, `structInitializeByName`, and
  `compileEmbeddedInitializer` (all typed literals reached through `compileInitializer`,
  used when the type is already known — e.g. `Point{...}`, `map[string]int{...}`,
  `[]int{...}` with an explicit type prefix).

The fix adds `Compiler.skipSyntheticSemicolons()` in `expr_atom.go`:

```go
func (c *Compiler) skipSyntheticSemicolons() {
    for c.t.IsNext(tokenizer.SemicolonToken) {
    }
}
```

Every literal-list parsing loop in both files now calls this at both of its checkpoints —
immediately before testing whether the next token is the list's closing bracket, and again
immediately before testing whether the next token is the required `,` separator — so a
stray synthetic `;` at either position is silently consumed instead of producing
`invalid list`. This mirrors a pattern already used elsewhere in the compiler
(`internal/language/compiler/typeCompiler.go`'s `parseInterface`, which skips stray
semicolons between multi-line interface method declarations) and keeps the fix scoped to
list-parsing loops only — ordinary statement-block parsing is untouched, so this does not
change semicolon handling anywhere outside these literal parsers.

A deliberate regression guard was added alongside the fix: a genuinely malformed literal
missing its comma **on a single line** (so no synthetic `;` is ever inserted) must still be
rejected with `invalid list`, e.g. `[]int{1 2}` or `{ "a": 1 "b": 2 }`. The fix only
consumes `;` tokens, so this case is unaffected and continues to error correctly.

New tests:

- `internal/language/compiler/bug41_multiline_literal_test.go` —
  `TestBUG41MultilineLiteral` covers both reproducers above plus the typed-array,
  typed-map, ordered-field struct, named-field struct, and nested-struct-field cases, each
  with the closing element on its own line and no trailing comma; it also pins down that
  literals with an explicit trailing comma and single-line literals are unaffected.
  `TestBUG41GenuineMissingCommaStillErrors` is the negative regression guard described
  above.
- `tests/compiler/multiline_literals.ego` — the same set of cases as end-to-end Ego
  language tests, including the exact `docs/LANGUAGE.md` `@localization` shape (three
  levels of nesting, no trailing commas) and a `@compile`-block negative test confirming
  `[1 2]` (a genuinely malformed single-line array literal) still raises a `"list"` compile
  error.

