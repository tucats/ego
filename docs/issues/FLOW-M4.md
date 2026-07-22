# FLOW-M4 — `defer namedFunc(arg)` evaluates arguments lazily, not eagerly

**Status:** Fixed.

**Affected files:**

- `compiler/defer.go` — defer statement compilation
- `bytecode/defer.go` (or equivalent) — argument capture at defer registration time

**Description:**  
In Go, the arguments to any defErred function call — named function or closure —
are evaluated immediately when the `defer` statement executes:

```go
x := "first"
defer fmt.Println(x)   // captures "first" NOW
x = "second"
// Output: "first"
```

In Ego, the closure-with-immediate-invocation form `defer func(p T){...}(arg)`
correctly captures `arg` at registration time. However, `defer namedFunc(arg)` does
**not** capture `arg` eagerly; the argument is re-read from the symbol table when
the defErred function actually runs:

```go
x := "first"
defer setLog(x)   // Ego: x is read lazily when defer runs
x = "second"
setLog is called with "second", not "first"
```

This silent behavioral difference can produce hard-to-diagnose bugs when the
variable changes between the `defer` statement and the function's return.

**Workaround (no longer required, kept for historical reference):**  
Use the closure form with an explicit argument to get Go-compatible eager capture:

```go
x := "first"
defer func(v string) { setLog(v) }(x)   // captures "first" eagerly
x = "second"
// setLog receives "first"
```

**Test file:** `tests/flow/defer_lifo.ego` — tests `"flow: closure arg captured at
defer time (eager)"` and `"flow: named function defer captures arg eagerly"` document
both forms now behaving identically, plus additional coverage described in the
Resolution section below.

**Recommendation (superseded by Resolution below):**  
When compiling `defer namedFunc(arg)`, evaluate and snapshot all argument expressions
at the point of the `defer` statement and store them in a local temporary, exactly
as is done for the closure-invocation form. This would make both forms consistent
with Go and with each other.

**Resolution:**  
Fixed exactly along the lines of the recommendation above, entirely in
`internal/language/compiler/defer.go`. No changes were needed in the runtime
(`internal/language/bytecode/defer.go`, `context.go`, or the `deferStatement`
struct) — that plumbing already correctly stores and replays a list of
pre-computed argument values; it was only ever fed those values *lazily* for
the named-function form.

**How `defer namedFunc(arg)` was compiled before the fix:** `compileDefer()`
rewrites any deferred call that isn't already a `func(){...}()` literal into
one, so `defer namedFunc(arg)` was rewritten to `defer func(){ namedFunc(arg) }()`
*before* being compiled. Because the closure's body is compiled once but only
*runs* later (when the deferred call actually executes), the token `arg`
inside that body was compiled as an ordinary variable reference — a `Load`
instruction — that reads whatever `arg` holds from the live, captured symbol
table at the time the closure finally runs. That is what made the argument
"lazy": the expression that computes its value was physically inside the
deferred closure, not in the compiler's normal (immediately-executing)
bytecode stream.

**The fix:** a new function, `hoistDeferCallArguments()`, runs *before* the
existing `func(){...}()` rewrite. For a call like `namedFunc(arg1, arg2)`, it:

1. Locates the call's argument list in the token stream (reusing the same
   "skip the identifier/dot chain" logic `findDeferCallEnd()` already used,
   factored out into a small sibling helper, `findDeferCallArgsStart()`).
2. Compiles each argument expression, left to right, using the compiler's
   normal expression grammar (`c.conditional()` — the same entry point an
   ordinary function call's argument list uses). Because this happens in the
   compiler's current (non-deferred) bytecode stream, each argument's value is
   computed right now, at the `defer` statement's point in the program — this
   is the crux of the fix.
3. Immediately stores each computed value into its own compiler-generated
   temporary variable (`data.GenerateName()` + a `StoreAlways` instruction —
   the same "materialize this expression under a private name" idiom already
   used by `compileAddressOf`/`compilePointerDereference` for `&expr`/`*expr`).
4. Rewrites the token stream so the call now reads `namedFunc($1, $2)` instead
   of `namedFunc(arg1, arg2)`, using the tokenizer's existing `Delete`/`Insert`
   primitives (the same primitives the surrounding `func(){...}()` rewrite
   already relied on).

The existing `func(){...}()` wrap then proceeds completely unchanged, now
wrapping `namedFunc($1, $2)`. When the deferred closure eventually runs, `$1`
and `$2` are simple variable *reads* of values that were already computed and
frozen — there is nothing left to re-evaluate, so the bug cannot recur.

Two details worth calling out:

- **Why this doesn't defer the call itself late enough to matter:** the
  callee (and, for a method call, its receiver) are still resolved only when
  the closure finally runs — exactly as before. Only the *arguments* moved
  earlier. This preserves the reason the closure-wrap exists in the first
  place (so a deferred method call's receiver setup is itself deferred; see
  `DEFER-1`/`DEFER-2`).
- **`$`-prefixed temporary names need no special scoping:** `DefineSymbol`/
  `ReferenceSymbol` (`compiler/symbols.go`) already special-case any name
  starting with `$` and never flag it as unused or undeclared, so the nested
  closure body's reference to `$1` resolves through the same "read a captured
  outer-scope variable at run time" mechanism any ordinary closure already
  uses — no new symbol-table plumbing was required.
- **Variadic spread (`defer f(s...)`) is handled correctly too:** the slice
  value itself is hoisted and frozen (matching Go, where a slice is a
  reference-type value — reassigning the variable afterward to point at a
  *different* slice does not affect the deferred call, but mutating the
  *same* underlying array would still be visible, exactly as in Go).

**Tests:**

- Go unit tests in `internal/language/compiler/defer_test.go`: new table
  cases for one argument, multiple arguments, a variadic argument, and a
  dotted method call, plus a case confirming a malformed argument expression
  is still correctly reported as a compile error. A dedicated
  `TestCompiler_compileDefer_ArgumentsHoistedEagerly` inspects the actual
  emitted bytecode shape (`DeferStart` → `Push` the argument value → `StoreAlways`
  the temp → `Push` the closure → `Defer`) and confirms the closure body loads
  the temp variable by name rather than containing any trace of the original
  literal argument value.
- Ego-language tests in `tests/flow/defer_lifo.ego`: the pre-existing test
  that had documented the buggy lazy behavior as expected
  (`"flow: named function defer evaluates arg lazily"`) was rewritten to
  assert the correct eager behavior and renamed
  `"flow: named function defer captures arg eagerly"`. New tests were added
  for: a named-function defer inside a loop capturing each iteration's value
  without needing a manual closure workaround; multiple arguments; an
  arithmetic expression argument; a value-receiver method call; and a
  variadic spread argument.

