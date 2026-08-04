# BUG-89 — Named return of a pointer, slice, or function type fails to compile

**Severity:** MEDIUM

**Description:**  
A named return value whose type is a compound type introduced with `*`, `[`, or
`func` was rejected at compile time in both type modes:

```ego
func f() (r *int) { return nil }               // Error: invalid return type list
func f() (r []int) { r = []int{5}; return }    // Error: invalid return type list
func f() (g func(int) int) { ... }             // Error: invalid return type list
```

The named-return detector in `compileReturnTypes`
(`internal/language/compiler/function.go`) recognized the `name Type` shape only
when the token following the name was itself an identifier
(`c.t.Peek(2).IsIdentifier()`). `*`, `[`, and `func` are not identifiers, so a
named return of a pointer, slice, or function type was never detected as a name;
parsing then restarted on the name token, which is not a valid type, and the
generic "invalid return type list" error was raised. (Named returns of scalar,
map, `chan`, and user-defined types already worked because the token after the
name was an identifier in those cases.)

**Fix:**  
The detector now checks the second token with `isTypeStartToken` (which accepts
identifiers plus `*`, `[`, and `func`) rather than `IsIdentifier`. To keep an
unnamed `map[string]int` or bare `chan` return from being misread as a value
*named* `map`/`chan` — now that `[` is an accepted type-start token — the name
position explicitly excludes the `map` and `chan` keywords, which are always the
start of their own type spec, never a name. The name is still matched with
`IsIdentifier`, so a named return may continue to shadow a primitive type
keyword (`func f() (string string)`; see BUG-75), and the `chan` exclusion keeps
the malformed `chan T` form (BUG-74) flowing through to `parseType` for
diagnosis:

```go
if c.t.Peek(1).IsIdentifier() &&
    !c.t.Peek(1).Is(tokenizer.MapToken) &&
    !c.t.Peek(1).Is(tokenizer.ChanToken) &&
    isTypeStartToken(c.t.Peek(2)) {
    returnName = c.t.Next().Spelling()
}
```

Regression coverage: `tests/functions/return_types.ego` — the "returns: named
pointer, slice, and function returns" and "returns: named return may shadow a
type name" tests, plus the named forms in the pointer/slice/map/channel/function
tests, all under both `--types=strict` and `--types=dynamic`.

