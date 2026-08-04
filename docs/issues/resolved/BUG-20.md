# BUG-20 — `iota` not supported in `const` blocks

**Severity:** LOW

**Description:**  
Go's `iota` identifier provides automatically-incrementing constants in a `const`
block. Ego does not support `iota` — any reference to it in a `const` block
produces `"invalid constant expression file"`. This is not documented as an
unsupported feature.

**Reproducer:**

```go
const (
    Apple  = iota
    Banana
    Cherry
)
```

**Actual output:**

```text
Error: at line 2:9, invalid constant expression file
```

**Notes:**  
Since `iota` is not mentioned in LANGUAGE.md, it may be intentionally unsupported.
The error message is confusing and unhelpful — it should say something like
`"iota is not supported in Ego const blocks"`.

**Resolution:**  
`iota` is now supported inside `const` declarations, matching Go's behavior,
including the common idiom of omitting `= expr` on later entries in a block so
they repeat the nearest preceding entry's expression:

```go
const (
    Apple  = iota   // 0
    Banana          // 1 (repeats "= iota")
    Cherry          // 2 (repeats "= iota")
)
```

Two changes were needed, both in `internal/language/compiler/`:

1. **Recognizing `iota` as a value, not a symbol lookup** (`expr_atom.go`,
   `expressionAtom()`): `iota` is not a reserved word — there is no dedicated
   tokenizer type for it — so it is matched by its spelling, the same way the
   `true`/`false` boolean literals are recognized. When the compiler is inside
   a `const(...)` declaration, a bare `iota` reference is compiled as a literal
   integer push instead of falling through to the ordinary symbol-lookup path
   (which is what previously produced `ErrInvalidConstant`, since `iota` isn't
   itself a previously-declared constant).

2. **Tracking the current counter and supporting the "omit the expression"
   shorthand** (`constant.go`, `compileConst()`): a new `Compiler.iota` field
   holds the zero-based position of the ConstSpec currently being compiled,
   or `-1` when the compiler is not inside a `const` declaration at all (so an
   unrelated use of the name `iota` elsewhere in a program still resolves as a
   normal, possibly-undefined, identifier — matching Go's own
   `"undefined: iota"` behavior outside of `const`). `compileConst()` sets this
   to `0` at the start of a block, increments it after every spec (whether or
   not that spec had its own `= expr`), and restores the previous value on
   exit via a `defer`. `Compiler.Clone()` (used internally by `Expression()`)
   was updated to copy this field so the isolated expression-compiling clone
   still sees the active counter.

   To support omitting `= expr`, `compileConst()` now remembers the token
   positions spanning the most recent explicit right-hand side. When a later
   spec has no `=` of its own, those same tokens are re-parsed (via the
   tokenizer's existing `Mark()`/`Set()` bookmarking, the same mechanism
   already used elsewhere in the compiler for speculative parsing) against the
   *current* `iota` value, which reproduces Go's specified "textual
   substitution of the first preceding non-empty expression list" semantics
   without needing a general-purpose constant-folding evaluator. This only
   applies inside a parenthesized `const ( ... )` block, and only when a
   preceding spec actually had an expression to repeat — a single,
   non-parenthesized `const Name` with no `=`, or the very first spec in a
   block with no `=`, are still compile errors (`ErrMissingEqual`), exactly as
   before this change.

**Tests:**

- Go unit tests in `internal/language/compiler/constant_test.go`:
  `TestCompiler_compileConst` gained table cases for a bare `iota`, the
  repeat-the-previous-expression idiom, `iota` inside a larger expression
  (`1 << iota`), the standalone (non-block) form, and two guard cases
  confirming `ErrMissingEqual` is still raised when there's nothing to repeat.
  `TestCompiler_compileConst_IotaValues` inspects the emitted bytecode's
  `Push` operands directly to confirm `iota` actually takes the values `0, 1,
  2, ...` in order (not just that compilation succeeds).
  `TestCompiler_compileConst_IotaScopeRestored` confirms `Compiler.iota` is
  restored to its `-1` sentinel after a `const` block finishes compiling.
- Ego-language tests in `tests/types/iota.ego` cover the exact reproducer
  above, explicit `iota` on every spec, the classic bit-shift/blank-identifier
  idiom (`KB = 1 << (10 * iota)`), independent counters across separate
  `const` blocks, the standalone form, mixing `iota` with plain constants, and
  `iota` inside an arithmetic expression. Two `@compile block { ... }
  catch(e) { ... }` tests verify the error paths: referencing `iota` outside
  any `const` declaration reports `symbol.not.found` (matching Go's own
  "undefined: iota" behavior), and a `const` block whose first entry omits `=`
  still reports `equals` (`ErrMissingEqual`).

