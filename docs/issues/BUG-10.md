# BUG-10 — `json.Unmarshal(b)` single-argument form rejected

**Severity:** MEDIUM

**Description:**  
LANGUAGE.md documents that `json.Unmarshal` can be called with just one argument
(the JSON byte array), in which case the decoded value is returned directly:
`r := json.Unmarshal(s)`. In practice this fails with
`"incorrect function argument count: 1"`.

**Reproducer:**

```go
import "fmt"
import "json"

func main() {
    b := json.Marshal({name: "Tom", age: 44})
    r := json.Unmarshal(b)     // single-arg form documented to return decoded value
    fmt.Println(r)
}
```

**Actual output:**

```text
Error: incorrect function argument count: 1
```

**Expected output:**

```text
{age: 44, name: "Tom"}
```

**Workaround:**  
Always use the two-argument form: `err := json.Unmarshal(b, &target)`.

**Resolution:**

The single-argument version was a legacy from a much older version of Ego
that didn't yet properly support pointers, so the Go-compliant version
was not possible. There's no reason to retain this legacy variable, so
the fix is to delete it from the documentation and require that the
`Unmarshal()` function be called properly.

