# BUG-49 — `base64.Decode` is declared with only a single return value despite its documented `(string, error)` signature

**Severity:** MEDIUM  **Status:** Fixed

**Description:**  
`docs/LANGUAGE.md` documents `base64.Decode(data string) (string, error)`. In practice,
only the single-value form works; the two-value form fails with a return-count error.

**Reproducer:**

```go
import "base64"

func main() {
    s := base64.Encode("Hello, World!")
    d, err := base64.Decode(s)
    fmt.Println(d, err)
}
```

**Actual output (before fix):**

```text
Error: at main(line 4), incorrect number of return values
```

**Expected output:**

```text
Hello, World! <nil>
```

**Fix:**  
`internal/runtime/base64/types.go` declared `Returns: []*data.Type{data.StringType}` (a
single value) although the docs describe an `(any, error)` Go-idiom signature. Per the
`callRuntimeFunction` dispatch rules (see `CLAUDE.md`), a wrapper whose declaration has more
than one return value must return its result as a `data.List` — returning a bare
`(value, error)` Go tuple instead causes any non-nil error to become an uncatchable runtime
abort rather than a normal catchable return value, which was the underlying cause of the
"incorrect number of return values" failure and of the abort-on-invalid-input behavior noted
below.

Fixed by:

- Changing the declaration's `Returns` to `[]*data.Type{data.StringType, data.ErrorType}`.
- Changing `decode` (`internal/runtime/base64/encoding.go`) to return
  `data.NewList(string(b), nil), nil` on success and `data.NewList(nil, errors.New(err)), nil`
  on failure, matching the established pattern used by `strconv.Itor`/`strconv.Rtoi`
  (`internal/runtime/strconv/roman.go`).

`base64.Encode` was left unchanged — it has no error case and its documented signature is a
single string return.

Go-level tests updated/added in `internal/runtime/base64/encoding_test.go`, and Ego-level
tests added in `tests/base64/base64.ego` (previously no Ego-level coverage existed for this
package).

