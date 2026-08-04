# INDEX-1 — `GetTokenText` clamps only the end of the range, panicking on a reversed slice

**Affected function:** `(*Tokenizer).GetTokenText`
**File:** `language/tokenizer/line.go`
**Risk:** Medium — panics while building a compiler error message, replacing a
diagnostic with a crash
**Status: RESOLVED**

## INDEX-1: Description

`GetTokenText(start, end)` clamped `end` against the length of the token stream
but left `start` untouched:

```go
if start < 0 {
    start = 0
}

if end < 0 || end >= len(t.Tokens) {
    end = len(t.Tokens) - 1
}

for i, token := range t.Tokens[start : end+1] {
```

With 10 tokens, `GetTokenText(20, 30)` clamps `end` to 9 and then evaluates
`t.Tokens[20:10]` — a slice expression whose low bound exceeds its high bound,
which panics.

The callers are all error-reporting paths that quote the source fragment
containing a problem. The positions they pass are derived from a statement that
failed to parse, so they cannot be assumed to lie within the token stream — that
is precisely the situation in which this function is called.

## INDEX-1: Fix

Both ends are now clamped, and a range that describes no tokens after clamping
returns an empty string rather than forming an invalid slice. A nil-receiver
check was also added for consistency with `GetLine` in the same file.

```go
if start > end {
    return ""
}
```
