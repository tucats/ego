# BUG-16 — `defer namedFunc(arg)` evaluates arguments lazily (cross-ref: FLOW-M4)

**Severity:** MEDIUM

**Description:**  
Already documented as `FLOW-M4` in `FUNCTIONAL_ISSUES.md`. Included here for
completeness. In Go, the arguments of a deferred function call are evaluated
immediately when the `defer` statement executes. In Ego, they are evaluated
lazily when the deferred function runs (at function return time), so the final
value of the variable is seen instead of the value at defer time.

**Reproducer:**

```go
import "fmt"

func main() {
    x := 1
    defer fmt.Println("defer x (should be 1):", x)
    x = 2
    defer fmt.Println("defer x (should be 2):", x)
    x = 3
    fmt.Println("main, x =", x)
}
```

**Actual output (LIFO, all see final x):**

```text
main, x = 3
defer x (should be 2): 3
defer x (should be 1): 3
```

**Expected output:**

```text
main, x = 3
defer x (should be 2): 2
defer x (should be 1): 1
```

**Workaround (no longer required, kept for historical reference):**  
Use a closure that receives the value as an argument:

```go
defer func(v int) { fmt.Println("defer x:", v) }(x)
```

**Resolution:**  
See `FLOW-M4` for the full write-up of the fix — the two identifiers track the
same underlying bug and were fixed together. Running the reproducer above now
correctly prints:

```text
main, x = 3
defer x (should be 2): 2
defer x (should be 1): 1
```

