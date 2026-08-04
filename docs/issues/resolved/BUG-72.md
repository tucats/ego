# BUG-72 — Channels do not support a Go-style element type (`chan string`) — **Resolved**

**Severity:** LOW

**Description:**  
Found incidentally while verifying the BUG-62 fix's documentation examples. Go channels are
always written with an element type — `chan string`, `chan int`, and so on. `docs/LANGUAGE.md`'s
own pre-existing channel examples used exactly this syntax (`c chan string`, `var xc chan
string`, `make(chan string, 10)`), but none of it actually works: Ego's `chan` type accepts no
element type anywhere it can be written — a `var` declaration, a function parameter, or
`make()`'s first argument — and each context fails with a different error.

**Reproducer:**

```go
func main() {
    var xc chan string
    _ = xc
}
```

**Actual output:**

```text
Error: at line 2:13, unexpected token: Type "string"
Error: terminated with errors
```

**Expected output:**

Either `chan string` should compile (matching Go), or — at minimum — the three different
contexts should fail the same, clearly-worded way rather than three different ones.

**Notes:**  
Three distinct failure shapes were observed, suggesting three separate parsing paths each lack
a case for a channel element type, similar in spirit to the `parseTypeSpec`/`parseType` split
found during BUG-70:

- `var xc chan string` → `unexpected token: Type "string"` (`internal/language/compiler/type.go`'s
  `parseTypeSpec`, used by `var` declarations)
- `func f(c chan string)` → `no such type` (the function-parameter type parser)
- `make(chan string, 10)` → not independently confirmed as a distinct error, but CLAUDE.md
  already documents this exact case as unsupported by design ("Channels use no type argument in
  `make`. Ego syntax is `make(chan, N)`, not `make(chan int, N)`. There is no per-channel
  element type in Ego channels.")

Plain, untyped `chan` works correctly everywhere (`var xc chan`, `func f(c chan)`,
`make(chan, N)`), and `docs/LANGUAGE.md`'s examples were corrected to use that form as part of
the BUG-62 fix. Whether Ego channels are intended to ever carry a static element type at all —
or whether "untyped, dynamically-typed messages" is the deliberate design (consistent with
`ego.compiler.types` defaulting to `dynamic`) — was not determined; if the latter, the right fix
may be documentation-only (updating the base-types table and `CLAUDE.md` to state plainly that
`chan` never takes an element type, and treating `chan string` as a permanent parse error by
design rather than a bug to fix in the compiler).

**Resolution (July 2026):**

The user confirmed the intended design: channels are deliberately untyped, and `chan T` should
be rejected everywhere with one clear, consistent error rather than three different confusing
ones. The investigation went one level deeper than the notes above anticipated and found the
true root cause was in the **tokenizer**, not the three separate parsers:

- **The real root cause** — `tokenizer.TypeTokens` (`internal/language/tokenizer/reserved.go`),
  the lexer's classification map for built-in type keywords, was simply missing `ChanToken`.
  Every sibling primitive (`int`, `string`, `map`, `struct`, ...) is registered there; `chan`
  was not. This meant the lexer produced a plain `IdentifierTokenClass` token for the text
  "chan" instead of the intended `TypeTokenClass` — so every `.Is(tokenizer.ChanToken)` check
  anywhere in the compiler silently failed (class mismatch), and "chan" only ever worked at all
  via the various parsers' fallback, spelling-based `TypeDeclarations` matching, which has no
  mechanism to look for a following element type. Fixed with a one-line addition:
  `ChanToken: true` in `TypeTokens`. This was verified safe with the full `go test ./...` and
  `ego test tests/` suites (no regressions) before being kept — the alternative, narrower fix
  (matching "chan" by spelling instead of by `.Is()` in just the two parsers below) was
  implemented first and worked, but the user asked for the proper root-cause fix once the
  tokenizer gap was identified, accepting the wider blast radius in exchange for `chan`
  genuinely behaving like every other primitive type keyword.
- **Explicit `chan` cases with a clear, shared error** — `internal/language/compiler/type.go`'s
  `parseTypeSpec` (used by `var` declarations) and `internal/language/compiler/typeCompiler.go`'s
  `parseType` (used by casts, assertions, struct/interface fields, function type specs — and,
  via `ParseFunctionDeclaration`, function parameters and return types) each gained a `chan`
  case that peeks at the following token: if it looks like the start of a type expression, a new
  error, `errors.ErrChannelElementType` ("channels do not have an element type; use `chan`
  alone, not `chan T`"), is raised immediately, added to `internal/errors/messages.go` and all
  three localization files.
- **Three separate "swallow the inner error and fall back to a different, wrong
  interpretation" call sites** had to be fixed so this new, clear error actually reaches the
  user instead of being silently discarded by a fallback path meant for genuinely ambiguous
  cases (a token that just isn't a type name at all, so retry it as a plain identifier
  reference): `expr_atom.go`'s "try type, else fall back to symbol lookup" atom parser (this is
  what made `make(chan string, 10)` fail with an unrelated "invalid list" instead — "chan" was
  silently re-parsed as a plain symbol reference, leaving "string" behind as an unconsumed,
  confusing leftover token), `expr_reference.go`'s `compileDotReference` (type-assertion
  targets), and `unwrap.go`'s `compileUnwrap` itself (the same swallow existed one level up,
  discarding the error `parseType` had already correctly raised). All three now check
  specifically for `errors.Equals(err, errors.ErrChannelElementType)` and propagate it
  immediately, rather than changing the general fallback behavior for every other kind of
  `parseType` failure.

**Verified working, consistently, in every context:** `var` declaration, function parameter,
`make()`, struct field, and type-assertion target all now raise the identical
`channel.element.type` error for `chan string`; bare `chan` continues to work unchanged in all
of the same contexts, plus as a pointer (`*chan`) and slice element (`[]chan`).

**Verified NOT regressed:** `chan` can still be used as an ordinary identifier — a variable
name or a struct field name — exactly like `int`, `string`, and every other primitive type
keyword already could; this was confirmed to be pre-existing, general Ego behavior (not
something newly granted to `chan`), and the full existing channel-operation test suite
(`tests/flow/channel_receive_atom.ego`, `tests/flow/two_value_receive.ego`,
`tests/flow/rangechannels.ego`, `tests/defer/channel.ego`) still passes unchanged.

**New issues found while verifying this fix, deliberately not fixed here** (confirmed present
on the pre-fix baseline too, so none of them are regressions from this change): a channel
stored in a struct field, array element, or map value does not work correctly for send/receive
([BUG-73](#BUG-73)); a malformed function return type is silently accepted with no error at all,
unlike every other type-spec context fixed above ([BUG-74](#BUG-74)); and a local
variable/parameter named after any primitive type keyword is shadowed by the type itself when
referenced in expression position, producing the wrong value ([BUG-75](#BUG-75)).

Regression tests: a new file, `tests/types/channel_type.ego`, with 16 `@test` blocks covering
`chan` as a type token, as an ordinary identifier (variable and struct-field name), bare `chan`
in every context (`var`, parameter, `make`, unnamed parameter, pointer, slice), `chan T`
rejection with the identical error code in every context, the disambiguation edge case where
`chan` immediately followed by a type-looking token is read as a named parameter rather than
rejected, and a regression check that BUG-62's channel-receive-as-expression-atom fix is
unaffected. Verified against `go test ./...` and `ego test tests/` / `ego test --types strict
tests/` / `ego test --types relaxed tests/` (1348 `@test` blocks, up from 1332) with no
regressions.

