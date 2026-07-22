# BUG-17 — `var name = value` (type-inferred initializer) not supported

**Severity:** LOW  
**Status:** Fixed

**Description:**  
The originally reported symptom was that the `var (...)` group form failed
with `"invalid type specification"`. Further investigation showed the bug
report's title was not quite accurate: the Ego compiler already accepted
grouped `var` declarations that include an explicit type, for example:

```go
var (
    a int
    b string
)
```

The actual bug is narrower: **`var` initializers without an explicit type**
were rejected, whether written as a single declaration or inside a `var (...)`
group. That is,

```go
var pi = 3.14
```

failed with `"invalid type specification"`, while the fully-typed equivalent

```go
var pi float64 = 3.14
```

worked correctly. The compiler had no code path to infer a variable's type
from its initializer expression when no type token was present — it only knew
how to (a) use an explicit type token, or (b) treat the token after the name
as a user-defined type name. Since `=` is neither, case (b)'s error path fired.

**Reproducer:**

```go
import "fmt"

func main() {
    var (
        p = 3.14
        q = "pi"
    )
    fmt.Println(p, q)
}
```

**Actual output (before fix):**

```text
Error: at line 5:5, invalid type specification
```

**Expected (and now actual) output:**

```text
3.14 pi
```

**Fix:**  
`compileVar` in `internal/language/compiler/var.go` now distinguishes two
different reasons `parseTypeSpec()` can return an undefined type:

1. The name is immediately followed by `=` (no type token at all) — the type
   must be inferred from the initializer expression, exactly like the short
   variable declaration `pi := 3.14`. This case is now handled by a new
   function, `varInferredInitializer`, which compiles the right-hand-side
   expression with the ordinary expression compiler (no target type, no
   coercion) and stores the result — the variable's type becomes whatever the
   expression's runtime type turns out to be.
2. The name is followed by an identifier that isn't a recognized built-in
   type — this is presumed to be a user-defined type name and is still
   handled by the existing `varUserType`.

A second, related bug was found and fixed as part of this change:
`compileVar`'s per-declaration loop (used for the parenthesized `var (...)`
list form) was controlled by a second return value from
`collectVarListNames` that was supposed to mean "are there more declarations
in this list?" but actually just signaled "did name-collection stop early
because a name was immediately followed by `=` or `)`?" — a fact already
available for free from an existing check at the top of the loop. Those two
meanings coincide exactly in the newly-enabled "name immediately followed by
`=`" case, so a `var (...)` list containing more than one type-inferred
declaration would have silently stopped after the first one. The redundant
return value was removed from `collectVarListNames`, and list continuation is
now decided solely by the top-of-loop "have we reached the closing `)`"
check — which already existed and is correct for every declaration form.

**Known limitation (unchanged, not introduced by this fix):** `var a, b =
42` duplicates the single computed value across every declared name, rather
than implementing Go's true multi-value form (`var a, b = 1, 2`, one value
per name). This mirrors the pre-existing behavior of the typed path (`var a,
b int = 42` already worked the same way before this fix); real Go actually
rejects `var a, b int = 42` at compile time, so this is a long-standing Ego
extension/quirk, not a regression from this fix. `var a, b = 1, 2` (distinct
values) is not supported by either path and was out of scope for this fix.

**Files changed:**

- `internal/language/compiler/var.go` — added `varInferredInitializer`;
  restructured `compileVar`'s branch on `kind.IsUndefined()` to check for a
  following `=` before falling back to `varUserType`; removed the unused
  `bool` return from `collectVarListNames` and decoupled list-loop
  continuation from it
- `internal/language/compiler/var_inferred_test.go` — new Go unit tests
  covering single-name inferred declarations (scalar, expression, and typed
  composite-literal initializers), multi-declaration `var (...)` lists
  (two, three, and mixed typed/inferred/user-type entries), the multi-name
  shared-value case, and regression guards for unknown user types and
  malformed names
- `tests/compiler/var_inferred.ego` — new Ego-level tests covering the same
  cases using `@test`/`@assert`, plus `@compile block` regression guards for
  the two error paths that must still fail correctly

