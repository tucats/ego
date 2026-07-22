# BUG-75 — A local variable or parameter named after a primitive type keyword is shadowed by the type itself

**Severity:** LOW

**Description:**  
Found while writing regression tests for the BUG-72 fix. Every primitive type keyword (`int`,
`chan`, `string`, `bool`, ...) can be used as an ordinary identifier — declaring `chan := 5` or
`func f(chan int)` compiles without error, matching how `int := 5` already worked before BUG-72.
But *referencing* such a variable in an expression is unreliable: in some cases the expression
evaluator resolves the name to the type itself (pushing the `*data.Type` value) instead of the
local variable, producing the wrong value instead of a compile error.

**Reproducer:**

```go
import "fmt"

func main() {
    chan := 5
    fmt.Println(chan)     // prints "chan", not 5
    chan = chan + 1        // this line then fails outright
    fmt.Println(chan)
}
```

**Actual output:**

```text
chan
Error: at main(line 6), invalid value: 1
Error: terminated with errors
```

**Expected output:**

```text
5
6
```

**Notes:**  
Confirmed present for `int` too (a different symptom was observed — "declared but never used"
rather than the wrong printed value — depending on the exact surrounding statements), so this
is a general quirk affecting every primitive type keyword used as an identifier, not specific to
`chan`. The exact trigger condition is not fully understood: a bare `chan := 5; fmt.Println(chan)`
with no further use reports "declared but never used" (as if the reference genuinely failed to
resolve to the variable), while adding a later `chan = chan + 1` reassignment changes the
`fmt.Println(chan)` outcome to printing the type name instead — suggesting the ambiguity between
"is this identifier a type reference or a variable load" is resolved differently depending on
what else appears in the same scope, rather than consistently one way or the other. Root cause
likely lives in `internal/language/compiler/expr_atom.go`'s "try parsing this identifier as a
type reference before falling back to a symbol lookup" logic (the same code touched by the
BUG-72 fix's `ErrChannelElementType` propagation), which apparently does not always defer
correctly to an existing local symbol of the same name. Not investigated further, since it
affects every primitive type name equally and is unrelated to the tokenizer classification BUG-72
fixed.

**Root cause:**  
Confirmed exactly where the earlier investigation pointed: `expressionAtom` (`expr_atom.go`)
always attempted `c.parseType("", true)` on a bare identifier — trying to read it as a built-in
type reference — before ever checking whether that name had already been declared as a local
variable. When the parse succeeded (which it always does for a primitive type keyword, since
those are recognized unconditionally), the `*data.Type` value was pushed onto the stack instead
of the variable being loaded, with no regard for whether a `chan := 5`-style declaration had
shadowed it earlier in the same scope. Go's own scoping rule — a local declaration hides an
identically-named predeclared identifier for the rest of its scope — was never implemented at
all for this code path.

**Resolution:**  
Added `Compiler.isLocalSymbol(name string) bool` (`internal/language/compiler/symbols.go`), a
read-only scan of `c.scopes` (the same stack `DefineSymbol`/`ReferenceSymbol` already maintain
for "unused variable" tracking) that reports whether `name` has been declared anywhere still in
scope. `expressionAtom` now checks `t.IsType() && c.isLocalSymbol(text)` before attempting both
the type-cast parse (`T(value)`) and the bare-type-reference parse, and skips straight to the
ordinary symbol lookup when true — giving a shadowing variable priority over the built-in type,
matching Go.

The `t.IsType()` gate (true only for a token lexically classified as a built-in type keyword —
`int`, `chan`, `string`, etc.) is required, not just a nice-to-have: `typeEmitter`
(`typeCompiler.go`) *also* calls `DefineSymbol` on every `type X ...` declaration's own name,
purely for "unused type" tracking. Without the `t.IsType()` gate, `isLocalSymbol` would treat
that self-registration as if the type had been shadowed by a variable of its own name the moment
it was declared — breaking ordinary `Point{...}` struct literals and `Point(x)` conversions
immediately after `type Point struct { ... }`. This was caught during testing (`go test ./...`
failures in `TestBUG41MultilineLiteral`/`TestBUG26StructValueSemantics` from an earlier, ungated
version of this fix) and is why the check is keyed off the token's lexical class rather than a
plain name lookup against `c.scopes`.

A second, related gap was fixed at the same time: once a built-in type name is shadowed, its
cast/conversion syntax should no longer apply either (`int := 5; int(3)` must not silently mean
"cast 3 to int" — Go treats it as attempting to call the variable `int`, which is an error). The
type-cast attempt at the top of `expressionAtom` got the same `t.IsType() &&
c.isLocalSymbol(text)` guard.

**New setting — `ego.compiler.type.shadowing`:**  
Alongside the bug fix, a companion teaching-oriented setting was added
(`internal/defs/config.go`; default `true`, matching Go's own behavior and Ego's historical
behavior; profile default wired up in `internal/runtime/profile/initialization.go`). When set to
`false`, declaring a variable, parameter, named return value, for-range variable, or catch
variable with the same spelling as a built-in type keyword is a compile-time error
(`errors.ErrTypeNameAsVariable`, "built-in type names cannot be used as variable names") instead
of the otherwise-legal shadowing this bug fix restores — useful in teaching contexts where `int
:= 5` is almost always a mistake, not a deliberate choice.

The setting is read once per compiler, in `New()` (`compiler.go`), and cached as
`c.flags.typeShadowing` — a plain field read, not a settings lookup, at every declaration site
(`checkTypeShadowing` in `symbols.go`). `Clone()` already copies the whole `flags` struct, so the
cached value naturally propagates to nested function compilers. The `@compile` directive gained a
matching `typeShadowing=true|false` flag (`directives.go`) that overrides the sub-compiler's
`c.flags.typeShadowing` directly, independent of both the parent compiler's flags and the global
profile setting — this is what lets a test exercise both settings values in one file without
ever touching the real profile.

`checkTypeShadowing` is called at every real variable-declaring site: `:=` short declarations and
plain assignment (`lvalue.go`, two call sites — one for the single-target lvalue path, one for
the multi-target list path), `var` declarations (`var.go`), function parameters and named return
values (`function.go`), for-range loop variables (`for.go`), `try`/`catch` and `@compile`'s own
`catch` variable (`try.go`, `directives.go`), `switch`'s semicolon-separated init variable
(`switch.go`), and `@capture`'s `:=` form (`capture.go`). It deliberately does *not* fire for a
`type X ...` declaration's own name (that's bookkeeping, not a variable) or for compiler-internal
registrations (package names, generated temporaries).

One implementation pitfall worth recording: the multi-target lvalue list parser
(`assignmentTargetList` in `lvalue.go`) is tried first by `assignmentTarget`, which silently
discards *any* error it returns and falls back to re-parsing from the original token position via
the single-target path — a pre-existing "is this a list, or not" disambiguation pattern that
predates this fix. The first version of the shadowing check here returned its error without
restoring the tokenizer position first (unlike every *other* early return in that loop), which
left the tokenizer mid-token by the time the silent fallback re-read from a stale position,
producing an unrelated "invalid symbol name: Special ':='" error instead of the real one. Fixed
by calling `c.t.Set(savedPosition)` before returning, matching the loop's existing convention.

**Regression tests:** a new file, `tests/types/type_shadowing.ego`, with 15 `@test` blocks
covering: shadowing via `:=` for both a channel-associated keyword (`chan`) and `int`
specifically; a function parameter, named return value, for-range variable, and `catch` variable
each shadowing a built-in type; a shadowed type's cast syntax correctly failing instead of
silently casting; ordinary (unshadowed) built-in type usage remaining unaffected; a user-defined
type's own name never being mistaken for shadowing; and six tests exercising the
`ego.compiler.type.shadowing` setting via `@compile`'s `typeShadowing=` flag — rejecting a
shadowing `:=`, `var`, and function parameter when `false`, confirming ordinary variable names
are unaffected when `false`, and confirming both an explicit `typeShadowing=true` and the flag's
absence (falling back to the ambient setting, `true` by default) allow shadowing. Verified
against `go build ./...`, `go vet ./...`, `go test ./...`, and `ego test tests/` under `--types
dynamic`, `--types strict`, and `--types relaxed` (1381 `@test` blocks, up from 1366, with no
regressions).

