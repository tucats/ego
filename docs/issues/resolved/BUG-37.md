# BUG-37 — The single-argument (default newline delimiter) form of `strings.Split` is not implemented

**Severity:** HIGH

**Description:**  
`docs/LANGUAGE.md` documents a one-argument form of `strings.Split(text)` that splits on a
default delimiter of newline ("If the character is not present, then a newline is assumed
as the delimiter character"). Calling it with one argument fails with an argument-count
error; only the two-argument form works.

**Reproducer:**

```go
import "strings"

func main() {
    b := "line1\nline2\nline3"
    x := strings.Split(b)
    fmt.Println(x)
}
```

**Actual output:**

```text
Error: at main(line 4), incorrect function argument count: 1
```

**Expected output:**

```text
["line1", "line2", "line3"]
```

**Notes:**  
Root cause: `internal/runtime/strings/types.go:473-490` declares `Split` as `IsNative: true`
pointing directly at Go's `strings.Split`, which always requires exactly two parameters. No
`ArgCount` range and no Ego wrapper supplying a default `"\n"` separator was implemented, so
the documented single-argument form has no code path at all.

**Resolution:**
The `strings.Split()` function was implemented as a pure native passthrough function, so the
Ego extension of allowing a default separator if one is not given was not being honored.
Added a new shim function `split()` in the runtime strings package that handles the case of
the missing separator by providing a default if needed, and then calling through to the
native function. The Ego array is constructed from the returned native array and the
function returns.

Added new Ego unit tests to validate this Ego-specific behavior in the strings package.

1. `internal/runtime/strings/types.go` updated to specify the function as having 1 or 2
   arguments, and removing the native passthru designation. Added reference to runtime
   shim `split()` functino.

2. `internal/runtime/strings/split.go` create to contain the new `split()` runtime shim.

3. `tests/packages/strings.ego` added to contain tests for Ego-specific extensions to
   the normal Go strings functionality.

