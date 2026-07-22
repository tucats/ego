# BUG-52 — `fmt.Sscanf` silently returns a `nil` error on literal-text mismatch or insufficient input

**Severity:** MEDIUM  **Status:** Fixed

**Description:**  
Real Go's `fmt.Sscanf` returns a non-nil error (e.g. `io.EOF`, or a "does not match format"
error) when fewer values are scanned than the format string requests, or when literal text
in the format string does not match the input. Ego's `fmt.Sscanf` silently returns `nil`
for `err` in both cases, even though the scanned-value count (`n`) correctly reflects the
shortfall — callers relying on `if err != nil` to detect a failed/partial scan will
silently miss it.

**Reproducer:**

```go
func main() {
    var c, d int
    n, err := fmt.Sscanf("5", "%d %d", &c, &d)
    fmt.Println(n, err, c, d)
}
```

**Actual output:**

```text
1 <nil> 5 0
```

**Expected output** (verified against real Go, `go run`):

```text
1 EOF 5 0
```

**Notes:**  
A second case, `fmt.Sscanf("wrong 35", "age %d", &a)`, returns `0 input does not match format 0`
in real Go vs. Ego's `0 <nil> 0`. Root cause in `internal/runtime/fmt/scan.go`'s
`scanner()`: the literal-text-mismatch branch (`count == 0`, ~line 363) does a bare `break`
out of the loop without ever setting the shared `err` variable declared earlier in the
function (~line 145). Additionally, the integer-parsing case (`'d'`, `'x'`, `'b'`, `'o'`)
shadows the outer `err` via `n, err := nativeFormat.Sscanf(...)` (`:=` at ~line 234), so even
a genuine parse error in that branch never propagates to the function's return value either.

**Fix:**  
`internal/runtime/fmt/scan.go`'s `scanner()` now sets and returns a non-nil error at every
point where it stops scanning early:

- Running out of data before a verb can be satisfied (either at a `%verb` boundary or
  mid-way through matching literal text) now returns the new `errors.ErrScanEOF` ("EOF"),
  matching real Go's `io.EOF`.
- A literal-text mismatch (data present but does not match the format string) now returns
  the new `errors.ErrScanMismatch` ("input does not match format").
- An empty/invalid digit string for the `%d`/`%x`/`%b`/`%o` verbs, and a genuine parse
  failure from the previously-shadowed `nativeFormat.Sscanf(...)` call (fixed to assign
  through the outer `err` with `=` instead of shadowing it with `:=`), now return the new
  `errors.ErrScanExpectedInteger` ("expected integer").

Every error branch in `scanner()` was also changed from a bare `break` (which only exited
the enclosing `switch`, letting the outer format-string loop silently continue) to an
immediate `return result, err`, so a failure stops the scan rather than masking the error
on a later, unrelated success.

`stringScan` (`fmt.Scan`) and `stringScanFormat` (`fmt.Sscanf`) no longer discard partial
results when `scanner()` returns an error: values successfully parsed before the failure
are still assigned to their pointer arguments, and the returned count reflects how many
values were actually scanned — matching Go's partial-scan behavior (e.g. the reproducer
above now returns `1 EOF 5 0`, not `0 EOF 5 0`).

New i18n keys `scan.eof`, `scan.expected.integer`, and `scan.mismatch` were added to
`internal/errors/messages.go` and all three language files. Verified against both
documented reproducers (byte-for-byte match with real Go's error text) plus additional
regression cases in `internal/runtime/fmt/fmt_test.go` and `tests/io/sscanf.ego`.

