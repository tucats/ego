# BUG-64 — A pointer receiver's Ego pointer-type marker is stripped by `getThisByteCode`, breaking `--types strict` when the receiver is returned as its own type — **Resolved**

**Severity:** MEDIUM

**Description:**  
Found while writing an Ego regression test for PERFORMANCE.md Finding 8 (unrelated to that
change — confirmed present on the unmodified compiler by stashing all Finding 8 changes and
re-running the reproducer). A method with a pointer receiver (`func (b *Builder) …`) that
returns the receiver itself as the declared `*Builder` return type fails to compile/run under
`--types strict`, even though nothing about the method's logic is wrong. This breaks the
common Go "fluent builder" pattern (`b.add("a").add("b")`) under strict typing, though the
bug does not actually require chaining — a single call is enough to reproduce it, and simply
mutating the receiver without returning it works fine.

**Reproducer:**

```go
package main

type Builder struct {
    text string
}

func (b *Builder) add(s string) *Builder {
    return b
}

func main() {
    b := &Builder{text: ""}
    b.add("a")
}
```

**Actual output** (`ego run --types strict`):

```text
Error: at add(line 8), type mismatch: Builder struct{text string}, *Builder struct{text string}
Error: terminated with errors
```

**Expected output:**

```text
(no output; program completes successfully)
```

**Reproducer 2** (the "auto-address" form — calling a pointer-receiver method directly on a
plain, non-`&` struct value, which Go and Ego both implicitly treat as taking the value's
address):

```go
package main

import "fmt"

type Builder struct {
    text string
}

func (b *Builder) add(s string) *Builder {
    b.text = b.text + s
    return b
}

func main() {
    b := Builder{text: ""} // no "&" here
    b.add("y").add("z")
    fmt.Println(b.text)
}
```

**Actual output** (both `dynamic` and `--types strict` — this reproducer failed in *both*
modes, unlike Reproducer 1, because the receiver was never boxed as a pointer at all):

```text
Error: at add(line 9), type mismatch: Builder struct{text string}, *Builder struct{text string}
Error: terminated with errors
```

**Expected output:**

```text
yz
```

**Notes:**  
Reproducer 1 does **not** reproduce under the default `dynamic` type mode, only
`--types strict`. Also does **not** reproduce for an ordinary (non-receiver) `*Builder`-typed
parameter returned the same way (`func identity(b *Builder) *Builder { return b }` works fine
under strict mode) — the bug is specific to the *receiver* parameter of a pointer-receiver
method. Reproducer 2 (auto-address) fails in *both* type modes, because unlike Reproducer 1 it
never even started with a `*any`-wrapped receiver to preserve — see Resolution below for why.

Root cause: `internal/language/bytecode/this.go`, `getThisByteCode` (invoked by the `GetThis`
instruction that `internal/language/compiler/function.go`'s `generateFunctionBytecode` emits
at the top of every method body to bind the receiver parameter). When a method is called on a
pointer variable, the value pushed onto the "this" stack is Ego's own pointer wrapper (`*any`,
matching the `&Builder{}` value in the reproducer). `getThisByteCode` unconditionally
dereferences it:

```go
if ptr, ok := v.(*any); ok && ptr != nil {
    v = *ptr
}
```

The comment above this explains the *intent* correctly: value receivers need something they
can copy via `$new`, and pointer receivers need field writes to propagate through the
underlying Go `*data.Struct` pointer — both of which this deref achieves, and mutation through
the receiver (`b.text = b.text + s`, in the original chained-call reproducer this bug was
found from) works correctly either way. The problem is that the deref also **discards Ego's
own pointer-type marker**: after this runs, the value bound to the receiver name is a bare
`*data.Struct`, indistinguishable at the Ego type-system level from a plain (non-pointer)
struct value — `typeof(b)` inside the method body now reports `Builder`, not `*Builder`,
regardless of how the receiver was declared. In `dynamic` mode nothing checks this, so the
method works correctly end-to-end. In `strict` mode, `return b` runs the declared-return-type
coercion that `internal/language/compiler/return.go`'s `compileReturn` appends from
`c.coercions` (built by `compileReturnTypes` in `function.go` from the method's declared
`*Builder` return type) — that coercion compares the receiver's actual (now-unwrapped) runtime
type against the declared `*Builder` type and fails, because they no longer agree.

Reproducer 2 (auto-address) fails for a related but distinct reason. `SetThis`
(`internal/language/compiler/expr_reference.go`'s `compileDotReference`) just pushes whatever
value "b" currently evaluates to onto the receiver stack, unchanged — it does not know at
compile time whether "b"'s literal declared type is a pointer, since Ego method dispatch is
resolved dynamically (`Member`/`getMemberValue` in `internal/language/bytecode/member.go`,
looked up at runtime). `b := Builder{text: ""}` (no `&`) never goes through
`internal/language/compiler/expr_atom.go`'s `compileAddressOf` (which is what produces a
genuine `*any` — see `AddressOf` bytecode, `c.symbols.GetAddress`), so the receiver stack holds
a bare `*data.Struct` from the very start; `getThisByteCode`'s `*any` check simply never
matches, and the value is bound to the receiver name completely unchanged — never a pointer
type, regardless of `--types` mode.

**Resolution (July 2026):**  
Fixed in two parts.

1. **`getThisByteCode`** (`internal/language/bytecode/this.go`) now re-boxes the receiver into
   a fresh `*any` for a genuine pointer receiver, covering *both* reproducers with one change:
   after the existing auto-deref step (which still always runs, so field writes keep
   propagating exactly as before), `if !byValue { boxed := v; v = &boxed }` runs. For
   Reproducer 1, `v` at that point is the just-dereferenced `*data.Struct`, so this restores
   the pointer marker the deref removed. For Reproducer 2, `v` was never `*any` to begin with,
   so this boxes it for the first time — exactly matching Go's implicit "take the address of
   an addressable value" rule for calling a pointer-receiver method on a non-pointer variable.
   A value receiver (`byValue == true`) is left untouched either way, since the `$new()` copy
   `generateFunctionBytecode` emits immediately after `GetThis` needs a plain value, not a
   boxed pointer. `byValue` is now threaded through as part of `GetThis`'s operand (changed
   from a bare name to `[]any{name, byValue}` in `function.go`) so `getThisByteCode` can tell
   the two receiver kinds apart; a single bare-name operand is still accepted for backward
   compatibility (with `byValue` defaulting to `false`, i.e. the safer pointer-preserving
   behavior).

2. **`loadIndexByteCode`** (`internal/language/bytecode/structs.go`) gained a `*any` case,
   mirroring the one `storeIndexByteCode` already had. This was needed because a standalone
   `recv.field++`/`recv.field--` statement compiles a hand-rolled `Load`/`LoadIndex`/…/
   `StoreIndex` sequence (see the "Qualified lvalue" branch of `compileAssignment` in
   `internal/language/compiler/assignment.go`) instead of routing through the general `Member`
   opcode that ordinary `recv.field = recv.field + 1` assignments use. `storeIndexByteCode`
   already handled a boxed `*any` receiver correctly (dereferencing it before writing the
   field), but `loadIndexByteCode` — the read half of the same sequence, used to fetch the
   current value before applying `+1`/`-1` — never had the matching case, so it fell through to
   `ErrInvalidType` ("invalid or unsupported data type for this operation") for *any* boxed
   pointer, not just receivers. This was a **pre-existing bug independent of fix 1 above**: it
   already affected a standalone `p.field++` on an ordinary `*T` function parameter (confirmed
   by reproducing it against the unmodified compiler), and was simply never exercised by the
   receiver return-type bug until fix 1 started boxing receivers more consistently.

**Files modified:**

- `internal/language/bytecode/this.go` — `getThisByteCode` re-boxes for pointer receivers (see
  above).
- `internal/language/compiler/function.go` — `GetThis`'s operand now carries `byValue`
  alongside the receiver name.
- `internal/language/bytecode/structs.go` — `loadIndexByteCode` gained a `*any` case.
- `internal/language/bytecode/this_test.go` (new) — seven unit tests covering both receiver
  kinds (pointer/value) crossed with both call shapes (explicit pointer/auto-address), plus the
  backward-compatible bare-name operand and an empty-receiver-stack no-op.
- `internal/language/bytecode/structs_test.go` — two new `loadIndexByteCode` tests: reading a
  field through a boxed `*any`, and rejecting a boxed non-struct value.
- `tests/types/pointer_receiver_return.ego` (new) — ten `@test` blocks: single call, chained
  calls, discarded return value, mutation propagation, value-receiver-via-pointer still copies,
  the auto-address form, standalone `++`/`--` on a receiver's field (both call shapes), a
  standalone `--` on an array element reached through a receiver, and a standalone `++` on an
  ordinary (non-receiver) pointer parameter's field (the bonus `loadIndexByteCode` fix).
- `tests/flow/basic_block_shared_scope.ego` — the pointer-receiver test originally rewritten to
  avoid this bug (see below) was restored to its original chained-call form now that the bug is
  fixed.

`tests/flow/basic_block_shared_scope.ego`'s pointer-receiver test was initially rewritten to
avoid this bug (calling the receiver method for its mutation side effect only, without
returning/chaining the receiver) while it was still open, so PERFORMANCE.md Finding 8's test
suite did not depend on an unrelated bug being fixed. With BUG-64 now resolved, that test was
restored to its original chained-call form; `tests/types/pointer_receiver_return.ego` carries
the dedicated regression coverage for this bug specifically.

**Correctness verification:** `go test ./...`, `go test -race ./...`, and both
`ego test tests/` and `ego test --types strict tests/` (1184 `@test` blocks each) pass with no
regressions, including the pre-existing `TestBUG26ReceiverSemanticsUnaffected` Go test (which
briefly regressed during development — see below — before the `loadIndexByteCode` fix was
added).

**A regression found and fixed during development, not merely documented:** the first version
of the `getThisByteCode` fix (re-boxing only when a deref had just occurred) broke
`TestBUG26ReceiverSemanticsUnaffected`'s `c.N++`-based test, which is exactly what led to
discovering the `loadIndexByteCode` gap described above. Simplifying the fix to always box for
`!byValue` (instead of only after a successful deref) both resolved Reproducer 2 and made the
gap reliably reproducible enough to root-cause and fix directly, rather than requiring a second
narrower workaround.

