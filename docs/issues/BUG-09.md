# BUG-09 — Import alias (`import alias "pkg"`) not recognized at use site

**Severity:** MEDIUM

**Description:**  
LANGUAGE.md documents an import alias syntax: `import str "strings"`, after which
the package should be accessible as `str.ToUpper(...)`. In practice, using the
alias name produces `"unknown symbol: str"` — the alias is ignored and the
package is not accessible under that name or its original name.

**Reproducer:**

```go
import str "strings"
import "fmt"

func main() {
    result := str.ToUpper("hello")   // unknown symbol: str
    fmt.Println(result)
}
```

**Actual output:**

```text
Error: at line 5:8, unknown symbol: str
```

**Expected output:**

```text
HELLO
```

**Workaround:**  
Import the package under its canonical name and use it without aliasing.

