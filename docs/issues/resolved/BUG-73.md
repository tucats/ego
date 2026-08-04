# BUG-73 — A channel stored in a struct field, array element, or map value does not work correctly for send/receive — **Resolved**

**Severity:** MEDIUM

**Description:**  
Found while verifying the BUG-72 fix. A channel stored directly in a local variable works
correctly for both send and receive, but the same channel accessed through a struct field
(`s.ch <- v` / `<-s.ch`), an array/slice element (`chans[0] <- v` / `<-chans[0]`), or a map
value fails. Confirmed present on the pre-BUG-72 baseline too, so this is unrelated to that
fix's tokenizer change.

**Reproducer:**

```go
import "fmt"

type S struct {
    ch chan
}

func main() {
    s := S{ch: make(chan, 1)}
    s.ch <- 5           // send succeeds
    fmt.Println(<-s.ch) // receive fails
}
```

**Actual output:**

```text
Error: at main(line 9), neither source or destination is a channel
Error: terminated with errors
```

**Expected output:**

```text
5
```

**Notes:**  
The error surfaces on the *receive* line (`fmt.Println(<-s.ch)`), but the true root cause is
the *send* the line before it: `s.ch <- 5` silently overwrites the field with the plain value
`5`, destroying the channel, so the receive that follows is reading a corrupted (non-channel)
field. Confirmed by isolating each half: `<-s.ch` alone, with the field populated by assigning
an already-sent-to channel from elsewhere (`s := S{ch: raw}` where `raw` was sent to directly),
receives correctly every time — receive through a compound lvalue was never actually broken.

**Resolution (July 2026):**

Two entirely independent root causes were found and fixed, covering all three compound-lvalue
kinds named in the title:

- **Struct field / array element send** — `patchStore`
  (`internal/language/compiler/lvalue.go`), which finalizes the bytecode for any assignment's
  left-hand side, always converted a compound lvalue's trailing `LoadIndex` instruction into an
  ordinary `StoreIndex`, regardless of whether the source used `<-` (a channel send) or `=` (a
  plain field/element write) — both forms compile to the identical instruction shape once a
  lvalue ends in `.field` or `[index]`, so `StoreIndex` (which knows nothing about channels)
  just overwrote whatever was there. This is distinct from the *simple*-lvalue case (`ch <- 5`,
  a bare variable name), which already worked via the existing `StoreChan` opcode — that opcode
  can inspect the popped value's Go type at runtime and fall back to an ordinary store, because
  it also has the destination *variable's name* to fall back to; a compound lvalue has no name,
  only a container and an index/key. Fixed by adding a new opcode, `StoreIndexChan`
  (`internal/language/bytecode/structs.go`), which `patchStore` now emits instead of
  `StoreIndex` specifically when `<-` was used on a compound lvalue. Since that only happens
  when the source unambiguously wrote a channel send, the new opcode needs no "maybe fall back
  to an ordinary write" logic: it reads back whatever is currently stored at that index/key,
  requires it to already be a `*data.Channel`, and sends to it, leaving the container itself
  unmodified.
- **Map value (a separate, deeper bug)** — a channel failed even on a *plain* assignment
  (`m["a"] = someChan`), before send/receive entered the picture at all, with
  `wrong map value type`. `data.TypeOf()` reported a live `*data.Channel` value's type as
  `PointerType(ChanType)` (kind `PointerKind`) instead of plain `ChanType` (kind `ChanKind`) —
  inconsistent with every sibling reference type (`*data.Map`, `*data.Struct`), both of which
  already report their own bare kind, not a synthetic "pointer to X". Since the compiler assigns
  a `chan`-typed variable plain `ChanType`, a genuine channel value could never structurally
  match its own declared type, and `data.Map.Set`'s value-type check (enforcing a typed map's
  declared element type, e.g. `map[string]chan`) rejected every channel outright. Fixed by
  correcting `data.TypeOf`/`data.KindOf`'s `*Channel` cases (`internal/language/data/types.go`)
  to match their siblings. Verified safe (no code anywhere depends on the old, inconsistent
  classification) via a dedicated investigation before making the change, given how widely
  `TypeOf`/`KindOf` are used throughout the type system.

Regression tests: a new file, `tests/flow/channel_compound_lvalue.ego`, with 13 `@test` blocks
covering struct field send/receive, array element send/receive, a goroutine sender writing to a
struct field, a pointer-receiver struct field, a nested struct field, multiple sequential
sends/receives, a map value (plain assignment, send/receive, and comma-ok lookup), and explicit
regression guards confirming ordinary (non-channel) struct/array/map/pointer-receiver stores are
completely unaffected. Verified against `go test ./...` and `ego test tests/` /
`ego test --types strict tests/` / `ego test --types relaxed tests/` (1361 `@test` blocks, up
from 1348) with no regressions.

A separate, pre-existing bug was found incidentally while writing these tests — a struct field
named the same as an unrelated type declared elsewhere in the file corrupts type registration —
and is tracked separately as [BUG-76](#BUG-76), not fixed here.

