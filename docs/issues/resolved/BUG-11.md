# BUG-11 — `fmt.Printf()` two-value return fails

**Severity:** MEDIUM

**Description:**  
LANGUAGE.md states that `fmt.Printf` returns `(length int, error)`. When callers
use the two-value assignment form `n, err := fmt.Printf(...)`, Ego reports
`"incorrect number of return values"`. The single-value form `n := fmt.Printf(...)`
works and returns the character count.

**Reproducer:**

```go
import "fmt"

func main() {
    n, err := fmt.Printf("hello %d\n", 42)
    fmt.Println("n:", n, "err:", err)
}
```

**Actual output:**

```text
hello 42
Error: at main(line 4), incorrect number of return values
```

**Expected output:**

```text
hello 42
n: 8 err: <nil>
```

**Notes:**  
`fmt.Sscanf` correctly supports the two-value return form. The bug is specific to
`fmt.Printf` and `fmt.Println` (which also returns `(int, error)` in Go but is not
documented to do so in Ego).

**Resolution:**

A number of the fmt functions were not observing the convention where a function that
can return an optional error value (like fmt.Printf()) must use a data.List as the
function argument, where the list contains the return values, and the return error is
also returned as the second parameter. When the caller function sees these, it tolerates
not having enough values on the stack for all destinations of the assignment. For
example:

```go
   fmt.Println("Hello")
   len = fmt.Println("Hello")        // result is 6
   len, err = fmt.Println("Hello")   // result is 6, <nil>
```

This fix addressed Print(), Println(), and Printf() which all had variations of
this issue.

