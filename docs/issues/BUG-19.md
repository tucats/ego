# BUG-19 — `for v := range string` yields single-char strings, not int32 runes

**Severity:** LOW  
**Status:** Fixed

**Description:**  
In Go, ranging over a string yields `(byte_index int, rune int32)` — the rune
value is an integer representing the Unicode code point. In Ego, the second
variable received a single-character `string`, not an `int32`. This behavioral
difference could cause porting issues for Go developers and silently produced
a different type than what Go documentation describes.

**Test:**

```go
import "fmt"
func main() {
    for i, c := range "ABC" {
        fmt.Printf("i=%d, c=%v, type=%T\n", i, c, c)
    }
}
```

**Previous (buggy) output:**

```text
i=0, c=A, type=string
i=1, c=B, type=string
i=2, c=C, type=string
```

**Expected (and now actual) Go-compatible output:**

```text
i=0, c=65, type=int32
i=1, c=66, type=int32
i=2, c=67, type=int32
```

**Fix:**  
`rangeNextString` in `internal/language/bytecode/range.go` pre-decodes a
string being ranged over into two parallel slices during `rangeInitByteCode`:
`keySet` (byte offsets) and `runes` (decoded `rune` values, i.e. `int32` code
points). On each step, `rangeNextString` previously wrapped the decoded rune in
`string(value)` before storing it into the loop's value variable, converting
it from an `int32` code point into a one-character string. That conversion
call was removed — the rune (already Go type `rune`, an alias for `int32`) is
now stored directly, matching Go's `for i, ch := range s` semantics where `ch`
has type `rune`.

No compiler changes were required: `compiler/for.go` and `rangeInitByteCode`
were already type-agnostic about the loop value; only the final assignment
step in `rangeNextString` needed correcting.

**Files changed:**

- `internal/language/bytecode/range.go` — `rangeNextString` now stores the
  decoded rune value directly instead of converting it to a string first
- `internal/language/bytecode/range_test.go` — updated
  `Test_rangeNextString_FullIteration` and
  `Test_rangeNextString_MultiByteUTF8` to assert `int32` rune values instead
  of single-character strings
- `tests/flow/forrange.ego` — updated the "range over a string" test case to
  compare against `int32` code points (`'f'`, `int32(0x2318)`, etc.) instead
  of single-character strings

**Notes:**  
This is an intentional behavior change to match Go semantics, not a
documentation-only clarification — any existing Ego program that relied on
`for _, ch := range someString` producing single-character strings must be
updated to treat `ch` as an integer code point (e.g., use
`string([]int32{ch})` or `fmt.Sprintf("%c", ch)` to get a one-character string
back, or compare `ch` against a rune literal like `'A'` directly).

