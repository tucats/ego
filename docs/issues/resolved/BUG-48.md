# BUG-48 — `fmt.Printf`/`Sprintf` do not collapse `%%` when the call has no substitution arguments

**Severity:** MEDIUM  **Status:** Fixed

**Description:**  
`fmt.Printf`/`fmt.Sprintf` with a format string containing `%%` but no substitution
arguments print the literal, unprocessed `%%` instead of collapsing it to a single `%`.
The same call with at least one substitution argument correctly collapses `%%`.

**Reproducer:**

```go
func main() {
    fmt.Printf("Progress: 50%%\n")
}
```

**Actual output (before fix):**

```text
Progress: 50%%
```

**Expected output:**

```text
Progress: 50%
```

**Fix:**  
Root cause was in `internal/runtime/fmt/print.go`, `stringPrintFormat`: when
`args.Len() == 1` (i.e. the format string only, no substitution values), the function
returned `fmtString` verbatim without ever calling Go's `fmt.Sprintf` or running the `%%`
preprocessing loop. The `args.Len() == 1` short-circuit was removed so the format string
always flows through to the final `fmt.Sprintf(fmtString, args.Elements()[1:]...)` call,
which correctly collapses `%%` (and applies any other formatting) even when there are no
substitution arguments — matching the behavior of `strings.Format`
(`internal/runtime/strings/conversion.go`), which never had this short-circuit and was
already correct.

Go-level regression test added in `internal/runtime/fmt/print_test.go`
(`Test_stringPrintFormat/%%_collapses_with_format_string_only,_no_values`), and an
Ego-level test in `tests/io/printf.ego` (`io: Sprintf collapses %% with no substitution
arguments (BUG-48)`).

