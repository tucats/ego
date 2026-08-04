# BUG-38 — The documented variadic multi-argument form of `strings.String` is not implemented

**Severity:** HIGH

**Description:**  
`docs/LANGUAGE.md` documents `strings.String(...)` as accepting multiple integer arguments
and combining each as a Unicode code point into the resulting string. Only the single-
argument form actually works; calling it with more than one argument fails with an
argument-count error.

**Reproducer:**

```go
import "strings"

func main() {
    a := strings.String(115, 101, 116, 115)
    fmt.Println(a)
}
```

**Actual output:**

```text
Error: incorrect function argument count: 4
```

**Expected output:**

```text
sets
```

**Notes:**  
Root cause: `internal/runtime/strings/types.go:491-503` declares `String` with exactly one
required parameter (`{Name: "any", Type: data.InterfaceType}`) and no `ArgCount` range or
variadic marker — the entire multi-argument use case documented for this function is
unimplemented. `strings.String(115)` (single argument) correctly returns `"s"`.

**Resolution:**
The function type declaration was not correctly set to cover variadic values. Additionally,
the return code should have been invalid argument type, not count.

1. `internal/runtime/strings/types.go` updated to specify that the `String()` function, an
   *Ego* extension, was a variadic function.

2. `internal/runtime/strings/conversion.go` updated `toString()` to return correct error
   when a non-integer or non-string value was found in the variadic list.

3. `tests/packapges/strings.ego` updated to add tests for the Ego-specific `Strings()`
   function, including verifying it works with different integer types, works with
   a mixture of integers and strings, and throws argument type error if other types
   found.

