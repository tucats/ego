# BUG-50 — `strings.Substitution` leaks an internal error string instead of leaving unmatched markers unchanged

**Severity:** MEDIUM  **Status:** Fixed

**Description:**  
`docs/LANGUAGE.md` documents that "any marker whose key is not found in `values` is left
unchanged in the result." Instead, an unresolved `{{marker}}` is replaced with a raw
internal error string from a third-party dependency.

**Reproducer:**

```go
import "strings"

func main() {
    person := struct {
        First string
        Last  string
    }{First: "Tom", Last: "Smith"}
    msg := strings.Substitution("Hello, {{First}} {{Missing}}!", person)
    fmt.Println(msg)
}
```

**Actual output:**

```text
!jaxon.json.element.not.found: Missing!
```

**Expected output:**

```text
Hello, Tom {{Missing}}!
```

**Notes:**  
Reproduces identically with a `map[string]interface{}` value. Root cause:
`internal/runtime/strings/substrings.go:183` delegates directly to the external
`github.com/tucats/subs` package's `Substitution()`, whose "key not found" error
(`jaxon.json.element.not.found`) is substituted verbatim into the output text instead of
leaving the `{{marker}}` untouched.

**Fix:**  
Fixed upstream in `github.com/tucats/subs` (bumped to v1.0.0); `Substitution()` now leaves
an unmatched `{{marker}}` unchanged in the output instead of embedding its internal
"key not found" error text. `internal/runtime/strings/substrings.go`'s `substitution()` was
updated to match the new two-return-value signature (`text, err := subs.Substitution(...)`).
Verified both the struct and `map[string]interface{}` reproducers now produce
`Hello, Tom {{Missing}}!`.

