# BUG-18 — `type()` documented but actual builtin is `typeof()`

**Severity:** LOW

**Description:**  
LANGUAGE.md (Functions section for `func` statement) states: `"If the function
body needs to know the actual type of the value passed, the type() function would
be used."` The actual builtin is named `typeof()`. Calling `type(v)` fails with
`"in type, invalid value: <v>"`.

**Test:**

```go
import "fmt"
func main() {
    n := 42
    t1 := type(n)     // Error: in type, invalid value: 42
    t2 := typeof(n)   // OK: returns "int"
    fmt.Println(t1, t2)
}
```

**Notes:**  
This is a documentation bug, not an implementation bug. `typeof()` works correctly.
The LANGUAGE.md should be updated to use `typeof()`.

**Resolution:**
Documentation updated to use correct function name `typeof()`.

