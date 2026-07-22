# BUG-70 — A function type spec with unnamed parameters fails to compile everywhere — **Resolved**

**Severity:** MEDIUM

**Description:**  
Found while verifying the [BUG-60](#BUG-60) fix. Go allows a function type's parameter list to
give only types, with no parameter names (e.g. `func(int, int) int`) — this is the normal way
to write a function type as opposed to a function *definition*. Ego's parser does not support
this form anywhere a function type can appear (`var` declarations, struct/interface fields,
type assertions, etc.) — only the named form (`func(a, b int) int`) works. This is unrelated to
type assertions specifically; it is a general limitation of `ParseFunctionDeclaration`
(`internal/language/compiler/function.go`) → `collectParameterNames`, which always tries to
read one-or-more identifier *names* before a type, with no lookahead to recognize a bare type
token instead.

**Reproducer:**

```go
func main() {
    var f func(int, int) int
    f = func(a, b int) int { return a + b }
    fmt.Println(f(2, 3))
}
```

**Actual output:**

```text
Error: at line 2:7, invalid type specification
Error: terminated with errors
```

**Expected output:**

```text
5
```

**Notes:**  
`collectParameterNames` reads "int" (a valid identifier per `Token.IsIdentifier()`, since
built-in type names are lexed as `TypeTokenClass`) as if it were a *parameter name*, then
expects a type to follow; by the time it reaches the second "int" there is nothing left to
consume as a name once the closing `)` arrives, and `parseType` fails outright when positioned
on `)`. A fix needs `collectParameterNames` (or its caller) to look ahead and distinguish "this
identifier is followed by another identifier or `,` then a type" (named form) from "this
identifier IS itself a complete type, with no name" (unnamed form) — mirroring how Go's own
parser disambiguates the two forms. Not yet attempted, since it touches the same function-
declaration parser used for every named function definition in the language and needs its own
careful test pass; out of scope for the BUG-60 fix that surfaced it.

**Resolution (July 2026):**

Three separate bugs had to be fixed, not one — the second and third were only discovered while
verifying a fix for the first:

- **Unnamed parameter lists** — `internal/language/compiler/function.go` gained a read-only
  lookahead, `isUnnamedParameterList`, that scans the upcoming parameter list (tracking bracket/
  paren/brace nesting so a nested function type's own parameter list, e.g. `f func(int) int`,
  doesn't confuse it) looking for the one pattern that can only occur in the named form: a
  top-level identifier immediately followed by the start of a type (another identifier, `*`,
  `[`, or `func`), with no comma in between. If that pattern is never found before the list's
  closing `)`, the list is unnamed. `parseParameterDeclaration` dispatches to a new
  `parseUnnamedParameterList`, which parses a comma-separated list of bare types (via the same
  `parseType` every other type-spec context uses) and gives each parameter a blank name, in that
  case; the existing named-form loop is completely unchanged and untouched when the lookahead
  says "named" (zero regression risk for all already-working code).
- **`var` declarations never supported function types at all** — independent of naming. The
  reproducer above (`var f func(int, int) int`) still failed after the fix above, with the same
  "invalid type specification" error, because `compileVar` uses a separate, narrower type parser,
  `parseTypeSpec` (`internal/language/compiler/type.go`), which had no case for `func` at all —
  not even the ordinary named, zero-parameter form (`var f func() int`) worked. `parseType` (used
  by type casts and type assertions) already had a working `func` case; `parseTypeSpec` now
  delegates to the exact same `ParseFunctionDeclaration` call for its own `func` case.
- **Named function type specs falsely reported "declared but never used"** — exposed only once
  the two fixes above let a named function type (e.g. `var f func(a, b int) int`) actually reach
  parameter parsing in a *type-spec* context (no function body ever follows). Every named
  parameter list — regardless of where it was parsed from — unconditionally called
  `c.DefineSymbol(name)`, which is correct when a real function body follows (so a genuinely
  unused parameter is still flagged) but wrong for a bare type spec, since its names are pure
  documentation with no body to ever reference them. `parseParameterDeclaration` gained a
  `defineSymbols bool` parameter: `compileFunctionDefinition`'s own call (always a real function,
  named or literal, that requires a body) passes `true`, unchanged from before; every call
  reachable through `ParseFunctionDeclaration` (struct/interface fields, type casts, type
  assertions, and the new `parseTypeSpec` case above — none of which ever have a body) now passes
  `false`. A real function literal's own parameter parsing (`compileFunctionDefinition`, line 80)
  is a separate, direct call untouched by this change, so a genuinely unused parameter in an
  actual function body is still correctly flagged (verified by a dedicated regression test).

Regression tests: `tests/functions/parm_type_lists.ego` gained seven new `@test` blocks covering
an unnamed function type in a `var` declaration (both zero-parameter and multi-parameter), a
named function type in a `var` declaration (the "falsely unused" regression guard), an unnamed
variadic function type, an unnamed function type as a struct field, a type assertion to an
unnamed multi-parameter function type (tying this fix to [BUG-60](#BUG-60)), and two negative
guards confirming a real function body's genuinely unused parameter is still flagged and that
Go's disallowed mixed named/unnamed form is still rejected. Verified against `go test ./...` and
`ego test tests/` / `ego test --types strict tests/` / `ego test --types relaxed tests/` (1321
`@test` blocks, up from 1313) with no regressions.

