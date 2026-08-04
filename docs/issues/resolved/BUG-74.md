# BUG-74 — A malformed function return type is silently accepted with no error

**Severity:** LOW

**Description:**  
Found while verifying the BUG-72 fix. Every other type-spec context (`var` declarations,
function parameters, `make()`, struct fields, type-assertion targets) now correctly rejects
`chan string` with a clear error. A function's *return* type does not — the malformed type is
silently discarded and the function compiles as if no return type had been declared at all,
with the tokens that would have described the (invalid) return type simply abandoned.

**Reproducer:**

```go
func makeChan() chan string {
    return make(chan, 1)
}

func main() {
    _ = makeChan
}
```

**Actual output:**

```text
(compiles and runs with no error)
```

**Expected output:**

A clear compile error — `channels do not have an element type; use "chan" alone, not "chan T"`
— matching every other context.

**Notes:**  
Initial investigation (during BUG-72) suspected `ParseFunctionDeclaration`'s return-type loop
(`internal/language/compiler/function.go`), which calls `c.parseType("", false)` for each return
type and, on any error, simply does `break` — discarding the error entirely. That loop turned out
to be only half the story; see Root cause below.

**Root cause:**  
Two separate code paths parse a function's return type(s), and both had the same underlying
defect, for the same reason:

- `compileReturnTypes` — used for a *real* function definition's own return types.
- `ParseFunctionDeclaration`'s return-type loop — used for a function *type spec* with no body
  (a `var` declaration, struct/interface field, or type assertion target), and, confusingly, the
  actual reproducer above (`func makeChan() chan string { ... }`) goes through this second path
  too, since `ParseFunctionDeclaration` performs a real named function's parameter-list parse a
  second time via a discarded, position-rewound speculative pre-parse.

Both paths use the same heuristic to decide whether a return item is a bare type or a *named*
return value (`func f() (result int)` — Ego, like Go, supports this): "is the next token an
identifier, and is the one after that also identifier-like?" If so, the first token is consumed
as the return value's *name* and the second is parsed as its *type*.

`chan` satisfies `IsIdentifier()` — it has done so since BUG-72 correctly classified it as
`TypeTokenClass`, and `Token.IsIdentifier()` deliberately treats any type keyword as identifier-
like too, precisely so it can be used as an ordinary variable name (see BUG-75). So `chan string`
in return-type position was never actually reaching the "parse a type, possibly get
`ErrChannelElementType`" code at all: the heuristic consumed `chan` as if it were the *name* of a
named return value, and then successfully parsed `string` alone as its type — no
malformed-channel-type parse was ever attempted, hence no error, and the function silently
compiled with (from the compiler's point of view) one named return value called `chan`, of type
`string`.

This is a different resolution than the one Ego already uses for *function parameters* in the
identical shape — `func f(chan string)` is deliberately read as "a parameter named `chan`, of
type `string`" (see the `channel_type.ego` test documenting that as intentional, tested BUG-72
behavior, since BUG-70's list-level unnamed/named disambiguation resolves it before any
type-vs-name ambiguity for a single identifier is even considered). Return types have no
equivalent list-level pre-scan, so nothing here forced this same resolution — and more to the
point, this bug's own stated expected behavior is an error "matching every other type-spec
context," not a silently-accepted named return. So the fix deliberately diverges from the
parameter precedent: `chan` is excluded from ever being read as a return-value name candidate, in
both `compileReturnTypes` and `ParseFunctionDeclaration`'s loop, since it has no legal
"`chan` immediately followed by another type-looking token" form other than the malformed `chan
T` mistake BUG-72 taught every other context to reject.

**Resolution:**  
In both `compileReturnTypes` and `ParseFunctionDeclaration`'s return-type loop
(`internal/language/compiler/function.go`), the "is this the start of a named return value"
identifier check now explicitly excludes `tokenizer.ChanToken`, so `chan` always falls straight
through to the ordinary type parse (`typeDeclaration()`/`parseType()`) instead of ever being
consumed as a candidate name. That type parse already produces `ErrChannelElementType` for `chan
T`, exactly as it does in every other context — the remaining piece was making sure that error
actually reaches the caller instead of being discarded:

- `compileReturnTypes` previously wrapped *any* `typeDeclaration()` error in a generic
  `ErrInvalidReturnTypeList`, discarding the original. It now checks for
  `errors.Equals(err, errors.ErrChannelElementType)` first and propagates that specific error
  unchanged, falling back to the generic wrapping only for other kinds of parse failures.
- `ParseFunctionDeclaration`'s loop previously treated *any* `parseType` error as "there simply
  wasn't a return type at all" and `break`-discarded it — correct for the common case (most
  `parseType` failures there really do just mean "no return type follows"), but wrong for `chan
  T` specifically, which is never ambiguous with "no return type." The loop now checks for
  `ErrChannelElementType` and returns it immediately, before falling through to the `break`
  used for every other failure.

Both fixes were verified not to disturb the legitimate named-return-value feature, in both its
bare (`func f() result int`) and parenthesized (`func f() (result int)`) forms, for any type
keyword other than `chan` — and bare `chan` (no element type) as a return type continues to work
exactly as before.

**Regression tests:** 6 new `@test` blocks added to `tests/types/channel_type.ego` (alongside the
existing BUG-72 tests it already covered channels with): `chan T` rejection as a bare function
return type, in a parenthesized return list, and as a function type spec's return type — plus
three explicit non-regression checks confirming bare `chan` return types, bare named returns, and
parenthesized named returns for other types all continue to work unchanged. Verified against `go
build ./...`, `go vet ./...`, `go test ./...`, and `ego test tests/` under `--types dynamic`,
`--types strict`, and `--types relaxed` (1389 `@test` blocks, up from 1383, with no regressions).

