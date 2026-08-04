# BUG-92 — An `any`-typed value forwarded into a second `any` parameter loses its type-assertable identity

**Severity:** MEDIUM

**Discovered by:** manual testing while verifying the type-assertion fallback path added for
PERFORMANCE.md Finding 17 (`docs/internals/GLOBALS.md`); unrelated to that work itself —
reproduces identically with no recursion, no deep call stacks, and no profiling/caching involved
at all. Confirmed present on `master` before that work began (byte-for-byte identical repro
output), so this is a pre-existing bug, not a regression from Finding 17's fix.

**Status: FIXED**

**Description:**  
A value stored in an `any`-typed variable or parameter, when passed *as an argument* into another
function's own `any`-typed parameter, silently loses the ability to be asserted back to its
original concrete type. A direct call with the original concrete value works correctly; only a
value that has already passed through one `any` parameter and is then forwarded into a second one
is affected:

```ego
type marker struct {
    v int
}

func inner(x any) bool {
    _, ok := x.(marker)
    return ok
}

func outer(x any) bool {
    return inner(x)   // x here is already an any-typed parameter value
}

m := marker{v: 1}
fmt.Println(inner(m))   // true  -- correct
fmt.Println(outer(m))   // false -- WRONG, should also be true
```

No recursion is required to reproduce it — a single extra level of `any`-to-`any` parameter
forwarding is sufficient. The bug also reproduces identically across many levels of recursion (a
function repeatedly calling itself with its own `any` parameter passed straight through): the very
first call succeeds, and every call after that fails, once the value has round-tripped through one
`any` parameter binding.

**Reproducer:**

```ego
type marker struct {
    v int
}

func recur(x any, depth int) bool {
    _, ok := x.(marker)
    if !ok || depth >= 5 {
        return ok
    }
    return recur(x, depth+1)
}

m := marker{v: 1}
fmt.Println(recur(m, 0))
```

**Actual output (before fix):** `false` — but `fmt.Println` on `depth`/`ok` at each level (see
investigation below) shows the assertion succeeds at `depth 0` and fails at every depth after
that, confirming the value degrades the moment it passes through a *second* `any` parameter
binding, not on the very first one.

**Expected output:** `true`.

**Investigation:**  
Isolated to a single extra level of indirection (no recursion needed at all — see the `outer`/
`inner` reproducer above), which immediately narrowed the search to argument/parameter binding
for `any`-typed parameters specifically.

`data.Interface` (`internal/language/data/interfaces.go`) is Ego's wrapper for a value declared
with the `interface{}`/`any` type — unlike plain Go, Ego always carries the concrete type
alongside the value (`Interface{Value: v, BaseType: TypeOf(v)}`) so the runtime never needs Go
reflection to answer "what type is this?" `data.Wrap(value)` constructs one of these; `data.UnWrap`
reverses it, returning the original value and its recorded type — but only strips **one** layer:

```go
func UnWrap(value any) (any, *Type) {
    if v, ok := value.(Interface); ok {
        ...
        return v.Value, v.BaseType
    }
    return value, nil
}
```

The bug: `relaxedConformanceCheck` (`internal/language/bytecode/types.go`), which runs whenever a
value is bound to a declared `any`/`interface{}` parameter under the default (non-strict) type
checking mode, unconditionally called `data.Wrap(v)` for an interface-kind target type — with no
check for whether `v` was **already** a `data.Interface`:

```go
if xf.Kind() == data.InterfaceType.Kind() {
    v = data.Wrap(v)
}
```

When `outer(x any)` calls `inner(x)`, `x` inside `outer` is already `Interface{Value: marker{...},
BaseType: markerType}` (from `outer`'s own parameter binding). Binding that same value to
`inner`'s `x any` parameter runs this check again, producing
`Interface{Value: Interface{Value: marker{...}, BaseType: markerType}, BaseType: <type of the
outer Interface struct itself>}` — a double-wrapped value. `unwrapByteCode`'s type-assertion
handler (`types.go`) only unwraps once:

```go
if _, ok := value.(data.Interface); ok {
    value, t = data.UnWrap(value)
}
```

so `value` after that single unwrap is still the **inner** `Interface`, not the `marker` struct.
`data.TypeOf(value)` then reports the type of that leftover wrapper object, which never matches
the asserted target type, and the assertion always reports `ok = false` — regardless of how many
further `any`-to-`any` forwards happen after the first one, since every level past the first
re-wraps whatever double (or triple, etc.) wrapper it was handed.

A second, independent call site has the identical latent bug: `internal/builtins/cast.go`'s
`interface{}(x)` cast function also calls `data.Wrap(source)` unconditionally when the target type
is an interface kind, with the same missing already-wrapped check. Confirmed reproducible the same
way (`y := interface{}(x)` inside a function receiving an `any` parameter, then asserting against
`y`).

**Root cause:** `data.Wrap` was not idempotent — it always constructed a fresh `Interface`
regardless of whether its argument was already one, while `data.UnWrap` only ever strips exactly
one layer. Any code path that could hand an already-wrapped value back into `Wrap` (parameter
binding being the common one, since Ego re-validates a value against its declared parameter type
on every single call) would silently nest wrappers that never got fully undone.

**Fix:** made `data.Wrap` idempotent — if the value passed in is already an `Interface`, return it
unchanged instead of wrapping it a second time:

```go
func Wrap(value any) any {
    if already, ok := value.(Interface); ok {
        return already
    }

    result := Interface{Value: value}
    ...
}
```

This is a single, minimal, centralized fix rather than patching each call site individually —
both `relaxedConformanceCheck` and `cast.go`'s `interface{}()` cast function call `data.Wrap`
directly and are fixed by the same change, and any future call site that wraps a value into `any`
is protected by construction rather than needing to remember this check itself.

**Files modified:**

- `internal/language/data/interfaces.go` — `Wrap` now checks for an already-wrapped `Interface`
  and returns it unchanged.

**Tests added:**

- `internal/language/data/interfaces_test.go` (new file) — `TestWrap_Idempotent` (the core BUG-92
  regression: `Wrap(Wrap(42))` must equal `Wrap(42)`, not nest), `TestWrap_NilValue` (idempotency
  holds for the nil/empty-interface case too), `TestUnWrap_RoundTrip` (existing single-wrap
  behavior unchanged).
- `tests/types/type_assertions.ego` — two new `@test` cases: forwarding an `any` parameter one
  level into a second `any` parameter (`outer`/`inner`), and the same value surviving several
  levels of recursive `any`-to-`any` forwarding.

**Verification:** `go build ./...`, `go vet ./...` clean. `go test ./...`, including
`go test -race ./internal/language/data/... ./internal/language/bytecode/... ./internal/builtins/...`,
clean. The full `ego test tests/` suite (1,708 cases, up from 1,706) passes, including both new
regression cases, in strict, relaxed, and dynamic type-checking modes, and the `interface{}(x)`
cast-function path.
