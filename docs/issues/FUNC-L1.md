# FUNC-L1 — String multiplication is asymmetric

**Affected files:**

- `bytecode/mul.go` (or equivalent arithmetic opcode handler)

**Description:**  
Ego supported string repetition via the `*` operator when the string was the
**left** operand:

```go
"A" * 3   // → "AAA"   (string repetition, former behavior)
3 * "A"   // → ERROR   (numeric multiplication fails on string)
```

This was documented as intentional (similar to Python), but the asymmetry was
confusing and combined badly with FUNC-5: `double("5")` where `double` expects
`int` silently produced `"55"` (string repetition) rather than `10`.

Additionally, the function `double("5")` where `double` expects an `int`
produces `"55"` (string repetition) rather than `10` (arithmetic) in dynamic
mode — a consequence of FUNC-M3 combined with this behavior.

**Test file:** `tests/functions/arg_types.ego` — tests
`"functions: string times int is string repetition"` and
`"functions: int times string is an error"` document the current behavior.

**Recommendation:**  
The string-repetition shortcut was already listed as informational with no
required action. Given that FUNC-5 has been resolved (coercion at call
boundaries now produces correct results), the string-repetition shortcut became
unnecessary and its asymmetry was a source of confusion. Removing it simplifies
the operator model.

**Resolution (May 2026):**  
The string-repetition special case was removed from `bytecode/math.go`:

- The `multiplyByteCode` handler no longer checks for a `(string, numeric)` pair
  and no longer calls `strings.Repeat`. Both `"A" * 3` and `3 * "A"` now
  produce a runtime error (`"invalid or unsupported data type"`), consistent with
  Go's behavior. The `strings.Repeat` function remains available for the
  intentional use case.

The test `"functions: string times int is string repetition"` was removed from
`tests/functions/arg_types.ego`. The test `"functions: int times string is an
error"` was retained (its stale comment about the left-operand exception was
removed).

The unit test `"multiply strings"` was removed from `bytecode/math_test.go`.

