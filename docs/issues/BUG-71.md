# BUG-71 — Type assertion used inline in a larger expression leaves a stray boolean on the stack

**Severity:** MEDIUM

**Description:**  
Found while verifying the [BUG-60](#BUG-60) fix. `unwrapByteCode`
(`internal/language/bytecode/types.go`) always pushes both the asserted value and a success
boolean for `x.(T)` (`(value, bool)`, bool on top), so that the two-value comma-ok form
(`v, ok := x.(T)`) can pop the bool separately. For the single-value form (`v := x.(T)`), the
compiler is supposed to follow the assertion with cleanup that consumes the bool and turns a
`false` into a catchable runtime error — but that cleanup is wired up in exactly one place,
`assignment.go`'s handling of `c.flags.hasUnwrap` (see `internal/language/compiler/
assignment.go:356`), which only fires when the assertion is the direct right-hand side of a
`:=`/`=` assignment *statement*. Any other position — used as a sub-expression, immediately
called, or combined with an operator — leaves the stray bool sitting on top of the intended
value, corrupting whatever bytecode runs next. This is completely independent of BUG-60 (it
reproduces with any assertion target, not just function types) and was previously masked for
function-type targets simply because they could not compile at all before that fix.

**Reproducer:**

```go
func main() {
    var x any = 5
    fmt.Println(x.(int) + 1)
}
```

**Actual output:**

```text
Error: at main(line 3), invalid function invocation: 5
```

(The specific error text varies with what happens to consume the stray `true`/`false` next —
here `+ 1` after `x.(int)` actually leaves the *value* as the operand for `Add` and the bool
gets misinterpreted downstream; other expression shapes fail differently, e.g. calling the
result of a function-type assertion inline reports "invalid function invocation: true".)

**Expected output:**

```text
6
```

**Notes:**  
In real Go, the single-value form of a type assertion is an ordinary primary expression and is
valid anywhere a value is expected — `x.(int) + 1`, `f(x.(int))`, and `x.(func())()` all compile
and run in Go, panicking on a type mismatch exactly as the bare-statement form does. Ego's
version should behave the same way.

**Resolution:**  
The fix matches the approach sketched above: the single-value cleanup (an `IfError` right after
`UnWrap`, consuming the trailing success/fail boolean `unwrapByteCode` always pushes and turning
`false` into a catchable `ErrTypeMismatch`) is now emitted directly by `compileUnwrap`
(`internal/language/compiler/unwrap.go`) itself, for both assertion-target forms (a plain
identifier name, and a compound type spec such as `*Point` or `func() int`). It fires whenever
the two-value comma-ok form is not in play — i.e. whenever `c.flags.inAssignment &&
c.flags.multipleTargets` is false, which is true both for a bare `v := x.(T)` assignment
statement and for every other expression position (BUG-71's actual target).

One case needs an explicit exclusion beyond the comma-ok form: `x.(type)`, the discriminator used
by `switch v := x.(type) { ... }`. `unwrapByteCode` (`internal/language/bytecode/types.go`) special-
cases that exact spelling and pushes `(type, value)` instead of the ordinary `(value, bool)` — so
blindly emitting `IfError` there would pop the *value* and misinterpret it as the success
boolean, corrupting a working feature. `compileUnwrap` checks `typeName.Is(tokenizer.TypeToken)`
and skips the new cleanup in that one case, leaving `switch.go`'s existing, separate
post-processing (keyed off `c.flags.hasUnwrap`) completely untouched.

With the cleanup now happening at the assertion site itself, `assignment.go`'s old
position-specific pass (which detected `c.flags.hasUnwrap` after the fact and only ever fired for
the direct right-hand side of a bare assignment statement) became partially dead code for the
single-value case, and partially still necessary: the two-value form (`v, ok := x.(T)`) still
needs one `Swap` there to put the extracted value and the `ok` boolean into the order
`storeLValue`'s two `Store` instructions expect. That `Swap` was kept; the now-redundant (and, if
left in place, actively harmful — it would double-process a stack `compileUnwrap` had already
cleaned up) single-value branch was removed.

**A closely related design question, raised and resolved during this fix:** should a failed
single-value assertion instead be raised through Ego's `panic()`/`recover()` mechanism, to more
closely mirror Go's real runtime panic on a failed assertion? Investigated and decided against:
Ego already has an established, consistent convention that *implicit* Go-panic-shaped runtime
errors — index-out-of-range, a nil map write, and now type-assertion failure — are all ordinary
catchable-via-`try`/`catch` errors, not `panic()`-based, regardless of what Go itself would do at
runtime; only an explicit call to the `panic()` builtin uses the recoverable-unwind mechanism
(confirmed by testing both index-out-of-range and nil-map-write, which behave identically to the
type-assertion case: caught by `try`/`catch`, *not* caught by a deferred `recover()`). Changing
type assertions specifically to use `panic()` would have made them the sole exception to that
pattern, and achieving the "also still catchable by `try`/`catch`" half of the request on top of
that would have required a much larger, unrelated change to how `panic()`'s recoverable unwind
interacts with `try`/`catch` generally — out of scope for this fix.

**Regression tests:** 11 new `@test` blocks added to `tests/types/type_assertions.ego` (alongside
the existing type-assertion coverage it already had): single-value assertions used as an
arithmetic operand, a function-call argument, an immediately-called function-type result, both
operands of an expression, a compound (pointer) target with a following member access, and a
nested assertion — plus three failure-case tests confirming each of those inline positions still
raises a catchable `ErrTypeMismatch` on a type mismatch, and two explicit regression checks
confirming the bare-assignment and comma-ok forms are unaffected. Verified against `go build
./...`, `go vet ./...`, `go test ./...`, and `ego test tests/` under `--types dynamic`, `--types
strict`, and `--types relaxed` (1400 `@test` blocks, up from 1389, with no regressions).

