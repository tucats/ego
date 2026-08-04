# BUG-62 — Channel receive (`<-ch`) is not supported as a general expression atom — **Resolved**

**Severity:** MEDIUM

**Description:**  
Found while writing Go unit tests for the BUG-30 fix. Go allows a channel receive
expression (`<-ch`) to appear anywhere a value is expected — as a function argument, as an
operand of an arithmetic or logical operator, and so on. In Ego, `<-ch` is only recognized
immediately after `:=` or `=` in an assignment statement (`v := <-ch` or `v = <-ch`); using
it anywhere else in an expression is a compile error.

**Reproducer:**

```go
func main() {
    ch := make(chan, 1)
    ch <- 5

    // As a function-call argument:
    fmt.Println(<-ch)
}
```

**Actual output:**

```text
Error: at line 6:17, unexpected token: Special "<-"
Error: terminated with errors
```

**Expected output:**

```text
5
```

**Notes:**  
The same error occurs for any non-assignment position, e.g. `total := 10 + <-ch` (as an
operand of `+`). The direct-assignment forms (`v := <-ch`, `v = <-ch`, and the two-value
`v, ok := <-ch`) all work correctly and are unaffected — this is specifically about `<-ch`
appearing anywhere else. Root cause: `tokenizer.ChannelReceiveToken` handling exists only in
`internal/language/compiler/assignment.go` and `internal/language/compiler/lvalue.go` (both
concerned with compiling the right-hand side of an assignment statement); the general
expression-atom parser used everywhere else (`internal/language/compiler/expr_atom.go`) has
no case for it at all, so the tokenizer's `<-` token simply falls through to "unexpected
token" wherever it is encountered outside those two call sites. The current workaround is to
always assign a channel receive to a temporary variable on its own line before using the
result: `v := <-ch; fmt.Println(v)`.

**Resolution (July 2026):**

Two changes were needed — the second was discovered while verifying the first, and the user
explicitly approved folding it into this fix rather than tracking it separately.

- **`<-ch` as a general expression atom** — `internal/language/compiler/expr_atom.go`'s
  `expressionAtom()` gained a case for `tokenizer.ChannelReceiveToken`, dispatching to a new
  `compileChannelReceive()`. It compiles the channel operand via `c.reference()` (not a full
  `c.Expression()`), so a suffix chain like `getChan()` or `chans[i]` still resolves, and so
  `<-` binds only as tightly as Go's own grammar requires — `10 + <-ch` compiles as
  `10 + (<-ch)`, not `10 + <-(ch)`. The existing `ReceiveChannel` opcode (used by the two-value
  comma-ok assignment form) pops the channel and pushes three items,
  `[StackMarker("receive"), ok, datum]`; since a plain expression atom only wants `datum`, the
  three items are collapsed to one with the same store-in-temp / `DropToMarker` / reload idiom
  the `?:` optional-catch operator (`optional()`, same file) already uses for equivalent stack
  cleanup — no new bytecode opcode was needed.
- **Leading `<-` on an assignment's right-hand side swallowed a trailing operator** — found
  while testing the fix above: `x := <-ch + 1` compiled (a regression-in-waiting, since before
  this fix it was a compile error) but evaluated as `<-(ch + 1)` instead of `(<-ch) + 1`,
  because a pre-existing special case in `assignment.go`/`lvalue.go` — needed only for the
  two-value form, `v, ok := <-ch`, whose right-hand side Go's own grammar requires to be bare
  `<-ch` with nothing else — unconditionally treated *any* leading `<-` on an assignment's
  right-hand side the same way, greedily parsing everything after it as one channel expression.
  Fixed by gating that special case on `c.flags.multipleTargets` (`assignment.go`): the
  single-value form no longer pre-consumes `<-` at all, letting the ordinary expression parser
  (and the new `expressionAtom` case above) handle it correctly wherever it appears. `lvalue.go`'s
  `assignmentTarget()` had a matching hack — peeking past the not-yet-consumed `:=`/`=` to detect
  a following `<-` and pre-baking a `StoreChan` instruction into the assignment's store code —
  which was removed outright; `StoreChan` is still emitted for the unrelated channel-*send*
  statement form (`ch <- value`, where `<-` itself plays the role of the assignment operator),
  which was unaffected by either change.

Regression tests: a new file, `tests/flow/channel_receive_atom.ego`, with 11 `@test` blocks
covering the original reproducer, a channel receive as an arithmetic operand (both a bare
operand and as the first token of an assignment's right-hand side, with and without a trailing
operator), multiple receives in one expression, an array-literal element, a user function-call
argument, a goroutine sender, `=` reassignment combined with a trailing operator, and explicit
regression guards for the channel-send statement and both pre-existing assignment forms
(single-value and comma-ok). Verified against `go test ./...` and `ego test tests/` /
`ego test --types strict tests/` / `ego test --types relaxed tests/` (1332 `@test` blocks, up
from 1321) with no regressions.

A separate, pre-existing bug found incidentally while double-checking this fix's documentation
examples is tracked as [BUG-72](#BUG-72): Ego channels do not actually support a Go-style
element type (`chan string`) anywhere, despite `docs/LANGUAGE.md`'s own examples using that
syntax before this fix.

