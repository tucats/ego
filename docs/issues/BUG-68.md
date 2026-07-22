# BUG-68 — Precision-loss checking is inconsistent across assignment/expression/argument/return boundaries, and across integer widths — **Resolved**

**Severity:** LOW

**Description:**  
Found while documenting type-coercion rules in `docs/LANGUAGE.md` after the BUG-67 fix, which
made a constant literal adapt to a narrower numeric type in expressions, function arguments,
and return values under `--types strict` (matching Go's untyped-constant rule). Verifying that
fix surfaced two related, pre-existing inconsistencies in how "does this conversion lose
information" is checked, neither of which BUG-67 introduced:

1. **Boundary inconsistency.** Plain variable assignment (`internal/language/bytecode/context.go`'s
   `checkType`) always rejects a `strict`-mode constant conversion that would lose a fractional
   part, regardless of the `ego.runtime.precision.error` setting — this is deliberate, explicit,
   in-code-documented behavior. But the other three boundaries (expressions via
   `data.Normalize`, function arguments via `strictConformanceCheck`, and return values via
   `coerceByteCode`) never check for lost precision at all by default, and — see point 2 below
   — mostly don't check for it even when `ego.runtime.precision.error=true` is set. A developer
   relying on `strict` mode to catch a lossy narrowing conversion will find it caught at
   assignment but silently allowed everywhere else.
2. **Per-width inconsistency in the coercion helpers themselves**
   (`internal/language/data/coerce.go`). Of all the `coerceToIntN`/`coerceFloat64ToIntN`
   helpers, only the `int64` target (`coerceToInt64`'s `float64` case) checks whether a
   float value has a fractional part (`value != math.Trunc(value)`) before truncating, and only
   does so when `ego.runtime.precision.error=true`. Every other integer width — `int8`,
   `int16`, `int32`, `uint16`, `uint32`, `uint`, `uint64`, and plain `int` — only ever checks
   for *magnitude* overflow against that width's max value; none of them ever check for a lost
   fractional part, no matter how the setting is configured. This looks like an oversight where
   the fractional check was added for `int64` and never propagated to its sibling functions,
   rather than an intentional design choice.

**Reproducer (boundary inconsistency — assignment rejects, expression silently truncates):**

```go
func main() {
    var w int32 = 5
    w = 3.7 // assignment: always rejected in strict mode
    fmt.Println("after w = 3.7:", w)
}
```

Run with `ego --types strict run`. **Actual output:**

```text
Error: at main(line 4), conversion results in data loss: 3.7
Error: terminated with errors
```

Compare to the same kind of truncation via an expression, which is never rejected, in any
mode, with any `ego.runtime.precision.error` setting:

```go
func main() {
    var a int32 = 10
    c := a + 2.7 // expression: never rejected, in any mode
    fmt.Println("a + 2.7 =", c)
}
```

Run with `ego --types strict run`. **Actual output:**

```text
a + 2.7 = 12
```

**Expected output:** Either both reject the lossy conversion, or both accept it — whichever is
chosen, the four boundaries (assignment, expression, argument, return) should be consistent
with each other.

**Reproducer (per-width inconsistency — `int64` checks, `int32` does not):**

```go
_ = int64(3.9) // errors when ego.runtime.precision.error=true
r := int32(3.9) // never errors, at any setting
```

Run with `ego --set ego.runtime.precision.error=true test`. **Actual output:**

```text
int64(3.9) error: in int64, conversion results in data loss: 3.9
int64(3.9) caught: true
int32(3.9) caught: false result: 3
```

**Expected output:** If `ego.runtime.precision.error=true` is meant to catch fractional-part
loss on narrowing float-to-integer conversions, it should do so consistently for every integer
width, not just `int64`.

**Notes:**  
Root-caused and fixed; both parts turned out to be independently fixable without a large
redesign.

**Resolution (July 2026):**

Per explicit product decision, boundary consistency (point 1) was resolved by *tightening*
expressions, arguments, and return values to match assignment's existing behavior — not by
relaxing assignment — since real Go also statically rejects an untyped constant that can't be
represented exactly in the type it's adapting to (`x + 2.7` where `x` is `int32` is a compile
error in Go). This completes what BUG-67 started: a constant adapts to context, but only
losslessly.

- **New shared helper**: `internal/language/data/coerce.go` gained `CoerceLossless(value,
  model)`, extracting the round-trip-through-`float64` technique `checkType`
  (`internal/language/bytecode/context.go`) already used for assignment — coerce, then compare
  the coerced result and the original value as `float64`; a mismatch means information was lost.
  This works for any numeric kind pairing with no per-width special-casing, and is independent
  of `ego.runtime.precision.error` (that setting governs a separate, second mechanism — see
  below). `checkType` itself was refactored to call this shared helper instead of duplicating
  the logic inline.
- **Expressions**: `data.Normalize` gained a 5th parameter, `strict bool`. In the BUG-67
  constant-adaptation branch, it now calls `CoerceLossless` instead of `Coerce` when `strict` is
  true, so `anInt32Var + 2.7` is rejected in strict mode (previously silently produced
  `anInt32Var + 2`). All 14 existing call sites (from BUG-67: `math.go`'s arithmetic ops, the
  six comparison opcodes, `optimizer.go`'s constant folding, and `runtime/math/math.go`'s
  `math.Normalize()`) were updated to pass the caller's actual strict-mode state, or `false`
  where the leniency branch can never fire (comparisons always pass `false, false`/`v1Const,
  v2Const` such that `v1Const != v2Const` is never true; constant folding always passes
  `true, true` for the same reason) — a mechanical, behavior-preserving update for those sites.
- **Function arguments**: `strictConformanceCheck`'s (`internal/language/bytecode/types.go`)
  BUG-67 leniency branch now calls `data.CoerceLossless(v, data.InstanceOfType(t))` and returns
  its result directly, instead of falling through to the existing coercion switch with no
  precision check.
- **Return values**: `coerceByteCode`'s (`internal/language/bytecode/coerce.go`) strict-mode
  path now routes a numeric constant through `CoerceLossless` before falling through to the
  general coercion switch; a non-constant value still goes through `requireMatch` as before
  (unaffected — that path already required an exact kind match).

For the per-width inconsistency (point 2), the fractional-part check
(`value != math.Trunc(value)`, gated by `precisionError()`) already present in
`coerceToInt64`'s `float64` case was added to every sibling coercion path that lacked it:
`coerceToInt8`, `coerceToInt16`, `coerceToUInt16`, `coerceToInt`, `coerceFloat64ToInt32`,
`coerceFloat64ToUInt32`, `coerceFloat64ToUInt64`, and `coerceFloat64ToByte` (covering both their
`float32` and `float64` cases where both exist). `coerceToInt64` itself was left unchanged.
This mechanism is deliberately separate from `CoerceLossless` above: it governs
`ego.runtime.precision.error`'s effect on ordinary (non-constant, non-assignment) coercions and
explicit casts (`int32(x)`, `byte(x)`, etc.), which the setting now catches consistently across
every integer width instead of only `int64`.

Regression tests: `tests/types/bug68_precision_consistency.ego` — all three previously-lenient
boundaries with a lossy constant (now rejected), the same three with an exact-fitting constant
(still succeed), `relaxed`/`dynamic` mode with a lossy constant (still silently truncates, since
this fix is strict-mode only), all nine previously-inconsistent integer widths with
`ego.runtime.precision.error=true` (now all reject a fractional part, matching `int64`), and the
same nine widths with the setting at its default (still silently truncate, no regression for
the common case). The `relaxed`/`dynamic` and default-setting tests explicitly force
`ego.runtime.precision.error` via `profile.Set()` rather than relying on its ambient/persisted
value, since a developer's saved profile can have it set to `true` (as this repo's did during
development of this fix) — the tests must hold regardless of that ambient state.

Verified against `go test ./...`, `go test -race ./internal/language/... ./internal/runtime/math/...`,
and `ego test tests/` / `ego test --types strict tests/` (1299 `@test` blocks each) run three
ways — with the ambient (developer-machine) `ego.runtime.precision.error=true` setting, and
again with `--set ego.runtime.precision.error=false` forced explicitly — with no regressions in
any configuration. `tests/types/bug67_constant_narrowing.ego` was also re-verified in both modes
(its constants are all exact-fitting, so none should be newly rejected by this stricter check).

