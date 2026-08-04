# BUILTIN-CAST-1 — `castToStringValue` only handled ASCII (single-byte) character literals

**Affected function:** `castToStringValue`
**File:** `builtins/cast.go`
**Risk:** Low
**Status: RESOLVED**

## Original CAST-1 behavior

The character-literal detection used `len(actual) == 3` and byte-indexed the
string (`actual[1]`).  A multi-byte Unicode character like `'é'` (2 UTF-8
bytes) produced `len == 4`, so the condition was never true and the cast failed.

## Fix for CAST-1

The byte-index check was replaced with a rune-based check:

```go
runes := []rune(actual)
if len(runes) == 3 && runes[0] == '\'' && runes[2] == '\'' {
    return int32(runes[1]), nil
}
```

`len(runes)` counts Unicode code points, so any single character enclosed in
single quotes (regardless of byte width) is handled correctly.

**Tests:** `Test_CastToStringValue_ASCIICharLiteral`,
`Test_CastToStringValue_MultibyteCharLiteral`
