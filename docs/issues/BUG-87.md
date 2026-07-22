# BUG-87 — Optimizer's constant-push/CreateAndStore collapse can fold a stack-frame marker into a variable's value

**Severity:** HIGH

**Description:**  
Found investigating BUG-86 above: with the optimizer forced on, `tests/errors/optional.ego`
failed with `unknown identifier: $2` — a compiler-generated temporary name used by the
`?expr : fallback` optional operator (`Compiler.optional`, `expr_atom.go`), which emits:

```text
Push Marker<try>
<bytecode for expr>
CreateAndStore <generatedName>
DropToMarker Marker<try>
Load <generatedName>
SymbolDelete <generatedName>
```

When a second `?expr : fallback` appears in the same scope and its `expr` is itself
constant-foldable (e.g. `?(100/5) : 99`), the "Constant division fold" rule first
collapses `Push ^100; Push ^5; Div` into a single `Push ^20`, leaving
`Push Marker<try>; Push ^20; CreateAndStore "$2"; DropToMarker Marker<try>`. The
"Collapse constant Push and CreateAndStore" rule then correctly fires once on
`(Push ^20, CreateAndStore "$2")`, producing `CreateAndStore ["$2", 20]`. The scanner
backs up to let the newly emitted instructions participate in further rounds (by
design, so cascading opportunities aren't missed) — and lands back on
`Push Marker<try>`. Because that rule's pattern used bare, unconstrained
`placeholder{}` operands on both sides (`{Push <value>}, {CreateAndStore <name>}`),
it matched *again*, this time treating `Marker<try>` as "the value" and the
already-folded `["$2", 20]` pair as "the name" — corrupting the CreateAndStore's
operand into a broken three-part shape and, critically, deleting the `Push Marker<try>`
instruction outright, which the later `DropToMarker Marker<try>` still expected to
find on the runtime stack. The net effect: `$2` was never actually stored under that
name, so the later `Load "$2"` failed with "unknown identifier."

This is a general hole in the peephole engine, not specific to the optional operator:
any rule matching a bare `Push <placeholder>` immediately before some opcode that is
*also* the collapsed target of another rule (as `CreateAndStore` is, via both this
rule and the separate "Create and store" `SymbolCreate+Store` rule) is at risk of
re-firing on its own output and absorbing a `StackMarker` frame sentinel as if it were
ordinary data.

**Fix:**  
Added a new `ExcludeStackMarker` flag to the optimizer's `placeholder` type
(`internal/language/bytecode/optimizer.go`), checked alongside the existing
`MustBeString` flag during pattern matching: when set, a match is rejected if the real
operand is a `StackMarker`. Applied it to "Collapse constant Push and
CreateAndStore"'s `value` placeholder. Also added `MustBeString: true` to the same
rule's `name` placeholder (the `CreateAndStore` side) — mirroring the guard the
sibling "Constant storeAlways" rule already had for the identical reason — so a
`CreateAndStore` whose operand is already a `[name, value]` pair from a prior fold
(not a bare string) can never be matched a second time. Either guard alone would have
prevented this specific failure; both were added since each independently closes off
a different half of the same mechanism.

Regression coverage:
`internal/language/bytecode/optimizer_test.go`'s
`Test_Optimize_CollapsePushAndCreateAndStore_DoesNotEatStackMarker` builds the exact
`Push Marker<try>; Push 100; Push 5; Div; CreateAndStore "$1"; DropToMarker Marker<try>`
sequence directly and asserts the marker survives and the CreateAndStore operand stays
a clean two-element pair — confirmed failing (marker instruction deleted, corrupted
three-part operand) with both guards reverted.
`internal/language/compiler/for_optimizer_regression_test.go`'s
`TestOptionalOperator_SurvivesOptimizer` exercises the same bug end-to-end through the
compiler with `ego.compiler.optimize = 2`.

