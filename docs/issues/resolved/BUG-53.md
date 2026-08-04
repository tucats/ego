# BUG-53 — `math.Primes` with a negative argument crashes instead of returning an empty result

**Severity:** MEDIUM  **Status:** Fixed

**Description:**  
`math.Primes(0)` gracefully returns an empty list, but `math.Primes` with a negative
argument crashes with a confusing internal error that points at an unrelated call site
(`make`) rather than at the actual `math.Primes` call or a validation message.

**Reproducer:**

```go
import "math"

func main() {
    fmt.Println(math.Primes(-5))
}
```

**Actual output:**

```text
Error: at make(line 97), invalid function argument: -4
```

**Expected output:**

An empty result (matching `math.Primes(0)`'s behavior) or a clear, argument-specific
validation error naming `Primes`, not `make`.

**Notes:**  
Root cause: `lib/packages/math/primes.ego:6` — `t := make([]bool, size+1)` performs no
validation that `size >= 0` before passing a negative length straight into `make()`.

**Fix:**  
`lib/packages/math/primes.ego` now validates its argument before doing any work:
`math.Primes` documents that it "accepts a positive integer value" (see
`lib/help_en.txt`), so any `size < 1` — this includes `0`, which previously returned a
silent empty result rather than erroring — now raises a catchable, argument-specific
error instead of falling through to `make()`'s confusing internal failure:

```go
import "errors"

func Primes(size int) []int {
    @extensions true

    if size < 1 {
        throw errors.New("func.arg").Context(size)
    }
    ...
```

`throw` is a language extension gated on `c.flags.extensionsEnabled` at compile time, so
`Primes` sets `@extensions true` for its own function body — this makes `throw` compile
and work correctly even when the caller's process has `ego.compiler.extensions` set to
`false` (verified with `ego --set ego.compiler.extensions=false run ...`). The thrown
error is caught the same way as any other runtime error: uncaught, it aborts the program
with `Error: at Primes(line N), invalid function argument: -5`; wrapped in `try/catch`,
`e.Is(errors.New("func.arg"))` matches. Regression tests added to
`tests/math/aggregate.ego`.

