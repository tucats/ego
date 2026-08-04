# BUG-36 — `strings.Left`/`Right`/`Substring` produce a blank error for documented edge-case arguments

**Severity:** HIGH

**Description:**  
`strings.Left`/`strings.Right`, when passed a `count` of `0` or less, produce a blank,
uninformative error and abort the program — instead of the empty string that
`docs/LANGUAGE.md` explicitly documents ("If the value of the count parameter is less than
1, an empty string is returned"). `strings.Substring` has the same underlying defect for a
negative start position.

**Reproducer:**

```go
import "strings"

func main() {
    x := strings.Left("Bob Smith", 0)
    fmt.Println(x)
}
```

**Actual output:**

```text
Error: 
Error: terminated with errors
```

**Expected output:**

```text
""
```

(`strings.Right("Bob Smith", 0)` fails identically.)

**Second reproducer:**

```go
import "strings"

func main() {
    x := strings.Substring("Abe Lincoln", -1, 4)
    fmt.Println(x)
}
```

**Actual output:** `Error:` (blank), program aborts.

**Notes:**  
Root cause in `internal/runtime/strings/substrings.go`, `leftSubstring`/`rightSubstring`:

```go
p, err := data.Int(args.Get(1))
if err != nil || p <= 0 {
    return "", errors.New(err).In("Left")   // err is nil here when p <= 0!
}
if p <= 0 {                                  // dead code — never reached
    return "", nil
}
```

When `p <= 0` but the argument parsed successfully, `err` is `nil`, so `errors.New(nil)` is
called; wrapped through `.In("Left")` this becomes a non-nil `error` interface around
effectively empty content, producing the blank `"Error: "` message and aborting instead of
returning the documented empty string (the immediately following `if p <= 0` line is dead
code that was clearly meant to handle this case). The same `errors.New(nil)` pattern
recurs in `substring()` (same file, ~line 21-24) for a negative start position.

**Resolution:***
The documented behavior is to silently return an empty string when ridiculous arguments are
given to the functions. So the functions in internal/runtime/substrings.go where each updated
to match the specification.

Additionally, the Go unit tests that validated this were corrected to not expect an error,
just an empty string in these cases. There are no Ego unit test cases for this scanario.

